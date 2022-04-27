package steve_test

import (
	"context"
	"testing"
	"time"

	"github.com/mailgun/holster/v4/steve"
	"github.com/stretchr/testify/require"
)

// Wait for job to stop.
func waitForStop(ctx context.Context, t *testing.T, client steve.JobsV1Client, taskId steve.TaskId) {
	for {
		resp, err := client.GetStoppedTasks(ctx, &steve.GetStoppedTasksReq{
			Filter: &steve.TaskFilter{
				TaskIds: []string{string(taskId)},
			},
		})
		require.NoError(t, err)

		if len(resp.Tasks) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}
