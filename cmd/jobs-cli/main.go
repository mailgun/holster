package main

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/mailgun/holster/v4/errors"
	"github.com/mailgun/holster/v4/steve"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Options struct {
	Endpoint string        `short:"e" long:"endpoint" description:"gRPC endpoint host:port" value-name:"x" default:"127.0.0.1:8081"`
	Timeout  time.Duration `short:"t" long:"timeout" description:"gRPC request timeout (not job timeout)" value-name:"duration" default:"30s"`
	Quiet    bool          `short:"q" description:"Quiet logging"`
	Run      RunOptions    `command:"run" description:"Run jobs"`
	Ls       LsOptions     `command:"ls" description:"List available jobs"`
	Output   OutputOptions `command:"logs" description:"Get task output logs"`
}

type RunOptions struct {
	ContinueOnError bool `long:"continue" description:"Continue to next job on error"`
	Detach          bool `short:"d" description:"Detach task, don't follow output"`
	Args            struct {
		JobIds []string `positional-arg-name:"job-id" description:"Job ids to run" required:"1"`
	} `positional-args:"yes" required:"yes"`
}

type LsOptions struct{}

type OutputOptions struct {
	Follow bool `short:"f" description:"Follow task output"`
	Args struct {
		TaskId string `position-arg-name:"task-id" description:"Task id" required:"yes"`
	} `positional-args:"yes" required:"yes"`
}

var (
	log        = logrus.WithField("category", "jobs-cli")
	options    Options
	mainCtx    context.Context
	mainCancel context.CancelFunc
	mainWg     sync.WaitGroup
)

func main() {
	mainCtx, mainCancel = context.WithCancel(context.Background())
	defer func() {
		// Handle panic and gracefully teardown.
		if err := recover(); err != nil {
			log.Error(err)
		}
		mainCancel()
		mainWg.Wait()
	}()

	// Parse CLI arguments.
	// `Options` implements handlers for CLI commands.
	parser := flags.NewParser(&options, flags.Default)
	_, err := parser.Parse()
	if err != nil {
		panic(fmt.Sprintf("Error parsing command line: %s", err.Error()))
	}

	return
}

func healthCheck(ctx context.Context, client steve.JobsV1Client) {
	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	_, err := client.HealthCheck(ctx, new(emptypb.Empty))

	if err != nil {
		panic(fmt.Sprintf("Error checking health of endpoint.  Does it support jobs API?: %s", err.Error()))
	}
}

func startTask(ctx context.Context, client steve.JobsV1Client, jobId steve.JobId) (steve.TaskId, error) {
	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	resp, err := client.StartTask(ctx, &steve.StartTaskReq{
		JobId: string(jobId),
	})
	if err != nil {
		return steve.TaskId(""), errors.Wrap(err, "error in client.StartTask")
	}

	return steve.TaskId(resp.TaskId), nil
}

func followTaskOutput(ctx context.Context, client steve.JobsV1Client, taskId steve.TaskId) error {
	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	outputClt, err := client.GetStreamingTaskOutput(ctx, &steve.GetTaskOutputReq{
		TaskId: string(taskId),
	})
	if err != nil {
		return errors.Wrap(err, "error in client.GetStreamingTaskOutput")
	}

	for {
		outputResp, err := outputClt.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "error in outputClt.Recv")
		}

		for _, line := range outputResp.Output {
			fmt.Println(line)
		}
	}

	return nil
}

func getTaskOutput(ctx context.Context, client steve.JobsV1Client, taskId steve.TaskId) error {
	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	pagination := &steve.PaginationArgs{Limit: 1000}

	for {
		resp, err := client.GetTaskOutput(ctx, &steve.GetTaskOutputReq{
			Pagination: pagination,
			TaskId: string(taskId),
		})
		if err != nil {
			return errors.Wrap(err, "error in client.GetTaskOutput")
		}

		for _, line := range resp.Output {
			fmt.Println(line)
		}

		if int64(len(resp.Output)) < pagination.Limit {
			break
		}

		pagination.Offset += pagination.Limit
	}

	return nil
}

func runTasks(ctx context.Context, client steve.JobsV1Client, runOptions *RunOptions) bool {
	success := true

	for _, jobId := range runOptions.getJobIds() {
		startTime := time.Now()

		// Run task.
		taskId, err := startTask(ctx, client, jobId)
		if err != nil {
			if runOptions.ContinueOnError {
				log.WithFields(logrus.Fields{
					"jobId":   jobId,
					"elapsed": time.Now().Sub(startTime),
				}).WithError(err).Error("Error starting task")
				continue
			}
			panic(errors.Wrap(err, "error starting task"))
		}
		log.WithFields(logrus.Fields{
			"jobId": jobId,
			"taskId": taskId,
		}).Info("Starting task...")

		if runOptions.Detach {
			continue
		}

		// Watch for output.
		err = followTaskOutput(ctx, client, taskId)
		if err != nil {
			if runOptions.ContinueOnError {
				log.WithFields(logrus.Fields{
					"jobId":   jobId,
					"elapsed": time.Now().Sub(startTime),
				}).Error(err)
				continue
			}
			panic(err)
		}

		// Get status.
		taskSuccess, err := getTaskSuccess(ctx, client, taskId)
		if err != nil {
			if runOptions.ContinueOnError {
				log.WithFields(logrus.Fields{
					"jobId":   jobId,
					"elapsed": time.Now().Sub(startTime),
				}).Error(err)
				continue
			}
			panic(err)
		}

		if taskSuccess {
			log.WithFields(logrus.Fields{
				"jobId":   jobId,
				"elapsed": time.Now().Sub(startTime),
			}).Info("Task finished")
		} else {
			log.WithFields(logrus.Fields{
				"jobId":   jobId,
				"elapsed": time.Now().Sub(startTime),
			}).Warn("Task finished unsuccessfully")
		}

		success = success && taskSuccess
	}

	return success
}

func getTaskSuccess(ctx context.Context, client steve.JobsV1Client, taskId steve.TaskId) (bool, error) {
	req := &steve.GetStoppedTasksReq{
		Pagination: &steve.PaginationArgs{Limit: 1},
		Filter: &steve.TaskFilter{
			TaskIds: []string{string(taskId)},
		},
	}
	resp, err := client.GetStoppedTasks(ctx, req)
	if err != nil {
		return false, errors.Wrap(err, "error in client.GetStoppedTasks")
	}
	if len(resp.Tasks) == 0 {
		return false, fmt.Errorf("error getting task status")
	}

	return resp.Tasks[0].Pass, nil
}

func lsJobs(ctx context.Context, client steve.JobsV1Client) {
	pagination := &steve.PaginationArgs{
		Limit: 1000,
	}

	for {
		getResp, err := client.GetJobs(ctx, &steve.GetJobsReq{Pagination: pagination})
		if err != nil {
			panic(errors.Wrap(err, "error in client.GetJobs"))
		}

		for _, item := range getResp.Jobs {
			fmt.Println(item.JobId)
		}

		if int64(len(getResp.Jobs)) < pagination.Limit {
			break
		}

		pagination.Offset += pagination.Limit
	}
}

func newClient(ctx context.Context, endpoint string) steve.JobsV1Client {
	// Connect to server.
	log.Infof("Connecting to %s...", endpoint)
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
	}

	mainWg.Add(1)
	conn, err := grpc.Dial(endpoint, opts...)
	go func() {
		<-ctx.Done()
		conn.Close()
		mainWg.Done()
	}()
	if err != nil {
		panic(fmt.Sprintf("Error connecting: %s", err.Error()))
	}
	client := steve.NewJobsV1Client(conn)
	healthCheck(ctx, client)

	return client
}

func processOptions() {
	if options.Quiet {
		logrus.SetLevel(logrus.ErrorLevel)
	}
}

func (o *RunOptions) Execute(_ []string) error {
	processOptions()
	client := newClient(mainCtx, options.Endpoint)
	runTasks(mainCtx, client, o)
	return nil
}

func (o *RunOptions) getJobIds() []steve.JobId {
	jobIds := make([]steve.JobId, len(o.Args.JobIds))
	for idx, jobId := range o.Args.JobIds {
		jobIds[idx] = steve.JobId(jobId)
	}
	return jobIds
}

func (o *LsOptions) Execute(_ []string) error {
	processOptions()

	ctx, cancel := context.WithTimeout(mainCtx, options.Timeout)
	defer cancel()

	client := newClient(ctx, options.Endpoint)
	lsJobs(mainCtx, client)
	return nil
}

func (o *OutputOptions) Execute(_ []string) error {
	processOptions()

	taskId := steve.TaskId(o.Args.TaskId)
	client := newClient(mainCtx, options.Endpoint)
	if o.Follow {
		err := followTaskOutput(mainCtx, client, taskId)
		if err != nil {
			return errors.Wrap(err, "error in followTaskOutput")
		}
	} else {
		err := getTaskOutput(mainCtx, client, taskId)
		if err != nil {
			return errors.Wrap(err, "error in getTaskOutput")
		}
	}
	return nil
}
