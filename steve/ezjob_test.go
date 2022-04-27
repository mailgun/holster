package steve_test

import (
	"context"
	"io"
	"sync"
	"testing"

	"github.com/mailgun/holster/v4/steve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestEZJob(t *testing.T) {
	const grpcPort = 0
	ctx := context.Background()

	t.Run("Happy path", func(t *testing.T) {
		const jobId = "Foobar job 1"

		var wg sync.WaitGroup
		wg.Add(1)

		mockAction := &MockAction{}
		mockAction.On("Run", mock.Anything, mock.Anything).Once().
			Run(func(args mock.Arguments) {
				writer := args.Get(1).(io.Writer)
				writer.Write([]byte("Foobar\n"))
				wg.Done()
			}).
			Return(nil)
		mockAction.On("Status", mock.Anything).Return()

		job := steve.NewEZJob(mockAction)

		// Start gRPC server.
		jobMap := map[steve.JobId]steve.Job{
			jobId: job,
		}
		testSrv, err := NewTestServer(grpcPort, jobMap)
		require.NoError(t, err)
		client := testSrv.jobClt
		defer func() {
			err := testSrv.Close()
			require.NoError(t, err)
		}()

		// Start task.
		resp, err := client.StartTask(ctx, &steve.StartTaskReq{
			JobId: jobId,
		})
		require.NoError(t, err)
		taskId := steve.TaskId(resp.TaskId)

		// Get task output.
		wg.Wait()
		outputResp, err := client.GetTaskOutput(ctx, &steve.GetTaskOutputReq{
			TaskId: string(taskId),
		})
		require.NoError(t, err)
		require.Len(t, outputResp.Output, 1)
		assert.Equal(t, "Foobar", outputResp.Output[0])
	})
}
