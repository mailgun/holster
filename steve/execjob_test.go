package steve_test

import (
	"context"
	"testing"

	"github.com/mailgun/holster/v4/steve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type JobHandler1 struct {
	LastExitCode int
}

func (h *JobHandler1) Done(exitCode int) {
	h.LastExitCode = exitCode
}

func TestExecAction(t *testing.T) {
	const grpcPort = 0
	ctx := context.Background()

	jobId1 := steve.JobId("ExecAction1")
	handler1 := new(JobHandler1)
	job1 := steve.NewExecJob(handler1, "/bin/sh", "-c", "echo 'Foobar'")

	jobId2 := steve.JobId("ExecAction2")
	handler2 := new(JobHandler1)
	job2 := steve.NewExecJob(handler2, "/bin/sh", "-c", "echo 'Foobar'; false")

	// Start gRPC server.
	jobMap := map[steve.JobId]steve.Job{
		jobId1: job1,
		jobId2: job2,
	}
	testSrv, err := NewTestServer(grpcPort, jobMap)
	require.NoError(t, err)
	client := testSrv.jobClt
	defer func() {
		err = testSrv.Close()
		require.NoError(t, err)
	}()

	t.Run("Happy path", func(t *testing.T) {
		// Start job.
		startResp, err := client.StartTask(ctx, &steve.StartTaskReq{
			JobId: string(jobId1),
		})
		require.NoError(t, err)
		taskId := steve.TaskId(startResp.TaskId)
		assert.NotEmpty(t, taskId)

		// Wait for task to stop.
		waitForStop(ctx, t, client, taskId)

		// View task output.
		outputResp, err := client.GetTaskOutput(ctx, &steve.GetTaskOutputReq{
			TaskId:     string(taskId),
			Pagination: &steve.PaginationArgs{Limit: 100},
		})
		require.NoError(t, err)
		require.Len(t, outputResp.Output, 1)
		assert.Equal(t, "Foobar", outputResp.Output[0])
		assert.Equal(t, 0, handler1.LastExitCode)
	})

	t.Run("Return error", func(t *testing.T) {
		// Start job.
		startResp, err := client.StartTask(ctx, &steve.StartTaskReq{
			JobId: string(jobId2),
		})
		require.NoError(t, err)
		taskId := steve.TaskId(startResp.TaskId)
		assert.NotEmpty(t, taskId)

		// Wait for task to stop.
		waitForStop(ctx, t, client, taskId)

		// View task output.
		outputResp, err := client.GetTaskOutput(ctx, &steve.GetTaskOutputReq{
			TaskId:     string(taskId),
			Pagination: &steve.PaginationArgs{Limit: 100},
		})
		require.NoError(t, err)
		assert.Equal(t, "Foobar", outputResp.Output[0])
		assert.Equal(t, 1, handler2.LastExitCode)
	})
}
