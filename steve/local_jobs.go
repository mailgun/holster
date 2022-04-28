/*
Copyright 2022 Mailgun Technologies Inc

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package steve

import (
	"bufio"
	"context"
	"io"
	"time"

	"github.com/mailgun/holster/v4/errors"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// LocalJobsV1Server implements gRPC service JobsV1.
// Runs jobs locally in-process.
type LocalJobsV1Server struct {
	UnimplementedJobsV1Server
	runner       Runner
	jobRepo      *JobRepo
	runningTasks *TaskRepo
	stoppedTasks *TaskRepo
}

type QueryResult struct {
	Line string
	Err  error
}

var (
	defaultPagination = &PaginationArgs{
		Offset: 0,
		Limit:  20,
	}
	ErrJobNotFound = errors.New("no such job found")
)

func NewLocalJobsV1Server(jobMap map[JobId]Job) *LocalJobsV1Server {
	return &LocalJobsV1Server{
		// jobMap:     jobMap,
		runner:       NewJobRunner(1000),
		jobRepo:      NewJobRepo(jobMap),
		runningTasks: NewTaskRepo(),
		stoppedTasks: NewTaskRepo(),
	}
}

func (s *LocalJobsV1Server) HealthCheck(_ context.Context, _ *emptypb.Empty) (*HealthCheckResp, error) {
	return &HealthCheckResp{
		Message: "OK",
	}, nil
}

func (s *LocalJobsV1Server) GetJobs(ctx context.Context, req *GetJobsReq) (*GetJobsResp, error) {
	resp := &GetJobsResp{
		Jobs: []*JobHeader{},
	}

	iterCtx, iterCancel := context.WithCancel(ctx)
	defer iterCancel()
	matches := s.jobRepo.Query(iterCtx, req.Pagination)

	for match := range matches {
		resp.Jobs = append(resp.Jobs, NewJobHeader(match.JobId))
	}

	return resp, nil
}

func (s *LocalJobsV1Server) StartTask(ctx context.Context, req *StartTaskReq) (*StartTaskResp, error) {
	jobId := JobId(req.JobId)
	job, ok := s.jobRepo.Get(jobId)
	if !ok {
		return nil, ErrJobNotFound
	}

	taskId, err := s.runner.Run(ctx, job)
	if err != nil {
		return nil, errors.Wrap(err, "error starting task")
	}

	s.addTaskId(jobId, taskId)

	resp := &StartTaskResp{
		TaskId: string(taskId),
	}
	return resp, nil
}

func (s *LocalJobsV1Server) StopTask(ctx context.Context, req *StopTaskReq) (*emptypb.Empty, error) {
	// Verify ownership of task id.
	taskId := TaskId(req.TaskId)
	if !s.runningTasks.Has(taskId) {
		return nil, ErrTaskNotFound
	}

	err := s.runner.Stop(ctx, taskId)
	if err != nil {
		return nil, errors.Wrap(err, "error stopping task")
	}

	// Rely on job done handler in addTaskId() to mark the task as stopped.

	return &emptypb.Empty{}, nil
}

func (s *LocalJobsV1Server) GetRunningTasks(ctx context.Context, req *GetRunningTasksReq) (*GetRunningTasksResp, error) {
	resp := &GetRunningTasksResp{
		Tasks: make([]*RunningTaskHeader, 0),
	}

	matches := s.runningTasks.Query(ctx, req.Pagination, req.Filter)

	for taskId := range matches {
		jobId, ok := s.runningTasks.GetJobId(taskId)
		if !ok {
			log.WithField("taskId", taskId).Error("consistency error, expected s.runningTasks to contain taskId")
			continue
		}

		status, exists := s.runner.Status(taskId)
		if !exists {
			log.WithField("taskId", taskId).Error("consistency error, s.runner.Status returned task not found")
			continue
		}

		taskHeader := &RunningTaskHeader{
			Job:     NewJobHeader(jobId),
			TaskId:  string(taskId),
			Started: timestamppb.New(status.Started),
		}
		resp.Tasks = append(resp.Tasks, taskHeader)
	}

	return resp, nil
}

func (s *LocalJobsV1Server) GetStoppedTasks(ctx context.Context, req *GetStoppedTasksReq) (*GetStoppedTasksResp, error) {
	resp := &GetStoppedTasksResp{
		Tasks: make([]*StoppedTaskHeader, 0),
	}

	matches := s.stoppedTasks.Query(ctx, req.Pagination, req.Filter)

	for taskId := range matches {
		jobId, ok := s.stoppedTasks.GetJobId(taskId)
		if !ok {
			log.WithField("taskId", taskId).Error("consistency error, expected s.stoppedTasks to contain taskId")
			continue
		}

		status, exists := s.runner.Status(taskId)
		if !exists {
			log.WithField("taskId", taskId).Error("consistency error, s.runner.Status returned task not found")
			continue
		}

		taskHeader := &StoppedTaskHeader{
			Job:     NewJobHeader(jobId),
			TaskId:  string(taskId),
			Started: timestamppb.New(status.Started),
		}
		resp.Tasks = append(resp.Tasks, taskHeader)
	}

	return resp, nil
}

func (s *LocalJobsV1Server) GetTaskOutput(ctx context.Context, req *GetTaskOutputReq) (*GetTaskOutputResp, error) {
	taskId := TaskId(req.TaskId)
	reader, err := s.runner.NewReader(taskId, 0)
	if err != nil {
		return nil, errors.New("error in s.runner.NewReader")
	}
	defer reader.Close()

	resp := &GetTaskOutputResp{
		Output: make([]string, 0),
	}
	pagination := req.Pagination
	if pagination == nil {
		pagination = defaultPagination
	} else if pagination.Limit == 0 {
		pagination.Limit = defaultPagination.Limit
	}
	items := queryReader(ctx, reader, pagination)

	for item := range items {
		if item.Err != nil {
			return nil, errors.Wrap(err, "error reading task output buffer")
		}

		resp.Output = append(resp.Output, item.Line)
	}

	return resp, nil
}

func (s *LocalJobsV1Server) GetStreamingTaskOutput(req *GetTaskOutputReq, server JobsV1_GetStreamingTaskOutputServer) error {
	const flushLineThreshold = 1000
	flushTimeout := 100 * time.Millisecond
	ctx := server.Context()
	taskId := TaskId(req.TaskId)
	reader, err := s.runner.NewStreamingReader(taskId, 0)
	if err != nil {
		return errors.New("error in s.runner.NewReader")
	}
	defer reader.Close()

	resp := new(GetTaskOutputResp)
	pagination := req.Pagination
	if pagination == nil {
		pagination = defaultPagination
		pagination.Limit = 0
	}
	items := queryReader(ctx, reader, pagination)

	flush := func() error {
		if len(resp.Output) > 0 {
			err = server.Send(resp)
			if err != nil {
				return errors.Wrap(err, "error in server.Send")
			}

			resp = new(GetTaskOutputResp)
		}

		return nil
	}

lineLoop:
	for {
		select {
		case item, ok := <-items:
			if !ok {
				// EOF.
				break lineLoop
			}
			if item.Err != nil {
				// Error from queryReader.
				return errors.Wrap(item.Err, "error reading task output buffer")
			}

			resp.Output = append(resp.Output, item.Line)

			if len(resp.Output) >= flushLineThreshold {
				err := flush()
				if err != nil {
					return errors.Wrap(err, "error in flush")
				}
			}

		case <-time.After(flushTimeout):
			err := flush()
			if err != nil {
				return errors.Wrap(err, "error in flush")
			}
		}
	}

	err = flush()
	if err != nil {
		return errors.Wrap(err, "error in flush")
	}

	return nil
}

// Add a job object to the collection.  Used by unit tests to inject mock
// jobs.
func (s *LocalJobsV1Server) AddJob(jobId JobId, job Job) {
	s.jobRepo.Add(jobId, job)
}

// addTaskId to internal data structures.
func (s *LocalJobsV1Server) addTaskId(jobId JobId, taskId TaskId) {
	s.runningTasks.Set(jobId, taskId)

	// Watch for task done.
	go func() {
		done, exists := s.runner.Done(taskId)
		if !exists {
			log.WithField("taskId", taskId).Error("consistency error, task id not found in runner")
			return
		}

		<-done
		s.jobDone(jobId, taskId)
	}()
}

// Handle job done event.
func (s *LocalJobsV1Server) jobDone(jobId JobId, taskId TaskId) {
	s.runningTasks.Clear(taskId)
	s.stoppedTasks.Set(jobId, taskId)
}

func min(a, b int64) int64 {
	if b < a {
		return b
	}
	return a
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// Query reader by lines.
// pagination.Limit = 0 means no limit.
func queryReader(ctx context.Context, reader io.Reader, pagination *PaginationArgs) <-chan QueryResult {
	out := make(chan QueryResult)

	go func() {
		defer close(out)
		scanner := bufio.NewScanner(reader)
		skip := pagination.Offset
		remaining := pagination.Limit
		hasLimit := remaining > 0

		for scanner.Scan() {
			if skip > 0 {
				skip--
				continue
			}

			line := scanner.Text()

			select {
			case out <- QueryResult{Line: line}:
			case <-ctx.Done():
				out <- QueryResult{Err: ctx.Err()}
				return
			}

			if hasLimit {
				remaining--
				if remaining == 0 {
					break
				}
			}
		}

		if scanner.Err() != nil {
			err := errors.Wrap(scanner.Err(), "error reading task output buffer")
			select {
			case out <- QueryResult{Err: err}:
			case <-ctx.Done():
			}
		}
	}()

	return out
}
