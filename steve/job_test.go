package steve_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/mailgun/holster/v4/steve"
	"github.com/mailgun/holster/v4/syncutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testJob struct {
	wg syncutil.WaitGroup
}

func (t *testJob) Start(ctx context.Context, writer io.Writer) error {
	fmt.Fprintf(writer, "Job Start\n")
	var count int

	t.wg.Until(func(done chan struct{}) bool {
		fmt.Fprintf(writer, "line: %d\n", count)
		count++
		select {
		case <-done:
			fmt.Fprintf(writer, "Job Stop\n")
			return false
		case <-time.After(time.Millisecond * 300):
			return true
		}
	})
	return nil
}

func (t *testJob) Stop(ctx context.Context) error {
	t.wg.Stop()
	return nil
}

func TestRunner(t *testing.T) {
	runner := steve.NewJobRunner(20)
	require.NotNil(t, runner)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	id, err := runner.Run(ctx, &testJob{})
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// Supports Multiple Readers for the same job
	go func() {
		r, err := runner.NewReader(id)
		require.NoError(t, err)

		buf := bufio.NewReader(r)
		for {
			line, err := buf.ReadBytes('\n')
			if err != nil {
				return
			}
			fmt.Printf("+ GOT: %s", string(line))
		}
	}()

	go func() {
		r, err := runner.NewReader(id)
		require.NoError(t, err)

		buf := bufio.NewReader(r)
		for {
			line, err := buf.ReadBytes('\n')
			if err != nil {
				return
			}
			fmt.Printf("- GOT: %s", string(line))
		}
	}()

	time.Sleep(time.Second)

	s, ok := runner.Status(id)
	require.True(t, ok)
	assert.Equal(t, id, s.ID)
	assert.Equal(t, true, s.Running)
	assert.False(t, s.Started.IsZero())
	assert.True(t, s.Stopped.IsZero())

	err = runner.Stop(ctx, id)
	require.NoError(t, err)

	// List should show the job as not running
	l := runner.List()
	assert.Equal(t, id, l[0].ID)
	assert.Equal(t, false, l[0].Running)

}
