package steve_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mailgun/holster/v4/errors"
	"github.com/mailgun/holster/v4/steve"
	"github.com/mailgun/holster/v4/syncutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type TestServer struct {
	wg      syncutil.WaitGroup
	grpcSrv *grpc.Server
	jobSrv  *steve.LocalJobsV1Server
	conn    *grpc.ClientConn
	jobClt  steve.JobsV1Client
}

const numFakeJobs = 50

var fakeJobs []steve.Job
var fakeJobIds []steve.JobId
var fakeJobMap map[steve.JobId]steve.Job
var testServerPort = uint32(3000)

func createFakeJobs() {
	// Build test data.
	fakeJobs = make([]steve.Job, numFakeJobs)
	fakeJobIds = make([]steve.JobId, numFakeJobs)
	fakeJobMap = map[steve.JobId]steve.Job{}

	for i := 0; i < numFakeJobs; i++ {
		jobId := steve.JobId(fmt.Sprintf("t%04x", i+1))
		fakeJob := &FakeJob{JobId: jobId}
		fakeJobIds[i] = jobId
		fakeJobs[i] = fakeJob
		fakeJobMap[jobId] = fakeJob
	}

	sort.SliceStable(fakeJobIds, func(i, j int) bool {
		return strings.Compare(string(fakeJobIds[i]), (string(fakeJobIds[j]))) < 0
	})
}

func TestMain(m *testing.M) {
	logrus.SetLevel(logrus.DebugLevel)
	createFakeJobs()

	// Do work.
	code := m.Run()

	// Clean up.
	os.Exit(code)
}

// Create gRPC server.
func NewTestServer(port uint32, jobMap map[steve.JobId]steve.Job) (*TestServer, error) {
	if port == 0 {
		port = atomic.AddUint32(&testServerPort, 1)
	}
	s := new(TestServer)
	s.grpcSrv = grpc.NewServer()
	s.jobSrv = steve.NewLocalJobsV1Server(jobMap)
	steve.RegisterJobsV1Server(s.grpcSrv, s.jobSrv)

	// Launch gRPC server.
	listener, err := net.Listen("tcp",
		fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return nil, errors.Wrap(err, "while listening")
	}

	errs := make(chan error, 1)

	// Start the gRPC server.
	s.wg.Go(func() {
		logrus.Infof("Job server listening on %s ...", listener.Addr().String())
		errs <- s.grpcSrv.Serve(listener)
	})

	// Ensure the server is running before we return
	go func() {
		opts := []grpc.DialOption{
			grpc.WithInsecure(),
		}

		errs <- retry(2, 500*time.Millisecond, func() error {
			s.conn, err = grpc.Dial(listener.Addr().String(), opts...)
			if err != nil {
				return err
			}
			s.jobClt = steve.NewJobsV1Client(s.conn)
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			_, err = s.jobClt.HealthCheck(ctx, &emptypb.Empty{})
			return err
		})
	}()

	// Wait until the server starts or errors.
	err = <-errs
	if err != nil {
		return nil, errors.Wrap(err, "while waiting for gRPC server readiness")
	}
	logrus.Info("Job server gRPC health check OK")

	return s, nil
}

func (s *TestServer) Close() error {
	err := s.conn.Close()
	if err != nil {
		logrus.WithError(err).Warn("Error in c.conn.Close")
	}

	s.grpcSrv.GracefulStop()

	s.wg.Wait()
	return nil
}

// CreateJobTasks creates a number of `MockJob`s and starts a task on each.
func (s *TestServer) CreateJobTasks(ctx context.Context, t *testing.T, numJobs int) []JobTaskItem {
	client := s.jobClt
	jobJobs := make([]JobTaskItem, numJobs)

	for i := 0; i < numJobs; i++ {
		jobId := steve.JobId("mockJob-" + randomId())

		mockJob := &MockJob{}
		mockJob.On("Start", mock.Anything, mock.Anything, mock.Anything).Once().Return(nil)
		mockJob.On("Stop", mock.Anything).Once().Return(nil)
		mockJob.On("Status", mock.Anything).Return()

		s.jobSrv.AddJob(jobId, mockJob)

		startResp, err := client.StartTask(ctx, &steve.StartTaskReq{
			JobId: string(jobId),
		})
		require.NoError(t, err)

		jobJobs[i] = JobTaskItem{
			JobId:  jobId,
			Job:    mockJob,
			TaskId: steve.TaskId(startResp.TaskId),
		}
	}

	return jobJobs
}

func retry(attempts int, d time.Duration, callback func() error) (err error) {
	for i := 0; i < attempts; i++ {
		err = callback()
		if err == nil {
			return nil
		}
		time.Sleep(d)
	}
	return err
}

func randomId() string {
	return uuid.NewString()
}
