package steve_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mailgun/holster/v4/steve"
	"github.com/mailgun/holster/v4/syncutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type TestJob struct {
	wg        syncutil.WaitGroup
	startChan chan struct{}
	t         *testing.T
}

func NewTestJob(t *testing.T) *TestJob {
	return &TestJob{
		startChan: make(chan struct{}),
		t:         t,
	}
}

func (j *TestJob) Start(ctx context.Context, writer io.Writer) error {
	go func() {
		// Wait for signal to start writing.
		<-j.startChan
		_, err := fmt.Fprintln(writer, "Job start")
		require.NoError(j.t, err)
	}()
	return nil
}

func (j *TestJob) Stop(ctx context.Context) error {
	j.wg.Stop()
	return nil
}

func TestSteve(t *testing.T) {
	t.Run("Happy path", func(t *testing.T) {
		deadline, ok := t.Deadline()
		require.True(t, ok)
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()

		runner := steve.NewJobRunner(20)
		require.NotNil(t, runner)
		defer func() {
			err := runner.Close(ctx)
			require.NoError(t, err)
		}()

		mockJob := &MockJob{}
		mockJob.On("Start", mock.Anything, mock.Anything).Once().Return(nil)
		mockJob.On("Stop", mock.Anything).Once().Return(nil)

		id, err := runner.Run(ctx, mockJob)
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		s, ok := runner.Status(id)
		require.True(t, ok)
		assert.Equal(t, id, s.ID)
		assert.True(t, s.Running)
		assert.False(t, s.Started.IsZero())
		assert.True(t, s.Stopped.IsZero())

		err = runner.Stop(ctx, id)
		require.NoError(t, err)

		// List should show the job as not running
		l := runner.List()
		require.Len(t, l, 1)
		assert.Equal(t, id, l[0].ID)
		assert.False(t, l[0].Running)

		mockJob.AssertExpectations(t)
	})

	t.Run("Stop jobs on close", func(t *testing.T) {
		deadline, ok := t.Deadline()
		require.True(t, ok)
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()

		runner := steve.NewJobRunner(20)
		require.NotNil(t, runner)

		mockJob := &MockJob{}
		mockJob.On("Start", mock.Anything, mock.Anything).Once().Return(nil)
		mockJob.On("Stop", mock.Anything).Once().Return(nil)

		id, err := runner.Run(ctx, mockJob)
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		err = runner.Close(ctx)
		require.NoError(t, err)

		s, ok := runner.Status(id)
		require.True(t, ok)
		assert.Equal(t, id, s.ID)
		assert.False(t, s.Running)
		assert.False(t, s.Started.IsZero())
		assert.False(t, s.Stopped.IsZero())

		mockJob.AssertExpectations(t)
	})

	t.Run("Error stopping job with invalid id", func(t *testing.T) {
		deadline, ok := t.Deadline()
		require.True(t, ok)
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()

		runner := steve.NewJobRunner(20)
		require.NotNil(t, runner)
		defer func() {
			err := runner.Close(ctx)
			require.NoError(t, err)
		}()

		err := runner.Stop(ctx, steve.ID("bogus"))
		require.Equal(t, steve.ErrJobNotFound, err)
	})

	t.Run("Error stopping an already stopped job", func(t *testing.T) {
		deadline, ok := t.Deadline()
		require.True(t, ok)
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()

		runner := steve.NewJobRunner(20)
		require.NotNil(t, runner)
		defer func() {
			err := runner.Close(ctx)
			require.NoError(t, err)
		}()

		mockJob := &MockJob{}
		mockJob.On("Start", mock.Anything, mock.Anything).Once().Return(nil)
		mockJob.On("Stop", mock.Anything).Once().Return(nil)

		id, err := runner.Run(ctx, mockJob)
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		err = runner.Stop(ctx, id)
		require.NoError(t, err)

		err = runner.Stop(ctx, id)
		require.Equal(t, steve.ErrJobNotRunning, err)

		mockJob.AssertExpectations(t)
	})

	t.Run("Multiple readers", func(t *testing.T) {
		// FIXME: Bug prevents this test from passing consistently with >1 readers.
		const numReaders = 2
		deadline, ok := t.Deadline()
		require.True(t, ok)
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()

		t.Log("Launch job...")
		runner := steve.NewJobRunner(20)
		require.NotNil(t, runner)
		defer func() {
			err := runner.Close(ctx)
			require.NoError(t, err)
		}()

		testJob := NewTestJob(t)
		id, err := runner.Run(ctx, testJob)
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		// Create multiple readers for the same job.
		var wgPass sync.WaitGroup
		var wgReady sync.WaitGroup
		var passCount int64
		t.Logf("Spawning %d readers...", numReaders)
		for i := 0; i < numReaders; i++ {
			wgPass.Add(1)
			wgReady.Add(1)
			go func(i int) {
				r, err := runner.NewReader(id)
				require.NoError(t, err)

				buf := bufio.NewReader(r)
				wgReady.Done()
				var readBuf []byte
				for {
					t.Logf("Reader %d/%d ReadBytes()", i+1, numReaders)
					line, err := buf.ReadBytes('\n')
					if err == io.EOF {
						break
					}
					require.NoError(t, err)

					readBuf = append(readBuf, line...)
					t.Logf("Reader %d/%d read %d bytes", i+1, numReaders, len(readBuf))
					if string(readBuf) == "Job start\n" {
						wgPass.Done()
						t.Logf("Reader %d/%d passed", atomic.AddInt64(&passCount, 1), numReaders)
						return
					}
				}

			}(i)
		}

		// Signal start then wait for passing condition.
		wgReady.Wait()
		t.Log("Start job...")
		close(testJob.startChan)
		wgPass.Wait()
	})
}
