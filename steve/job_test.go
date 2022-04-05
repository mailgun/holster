package steve_test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/mailgun/holster/v4/steve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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
		const numReaders = 1000
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

		// Create a mock job and capture the writer object.
		mockJob := &MockJob{}
		var jobWriter io.Writer
		var jobReadyWg sync.WaitGroup
		message := []byte("Foobar\n")
		jobReadyWg.Add(1)
		mockJob.On("Start", mock.Anything, mock.Anything).Once().
			Run(func(args mock.Arguments) {
				jobWriter = args.Get(1).(io.Writer)
				jobReadyWg.Done()
			}).
			Return(nil)
		mockJob.On("Stop", mock.Anything).Once().Return(nil)

		id, err := runner.Run(ctx, mockJob)
		require.NoError(t, err)

		// Create multiple readers for the same job.
		var wgPass sync.WaitGroup
		var wgReady sync.WaitGroup
		for i := 0; i < numReaders; i++ {
			wgPass.Add(1)
			wgReady.Add(1)

			// Launch reader in goroutine.
			go func(i int) {
				r, err := runner.NewReader(id)
				require.NoError(t, err)

				buf := bufio.NewReader(r)
				wgReady.Done()
				pass := false

				for {
					// Wait for next line of text.
					line, err := buf.ReadBytes('\n')
					if err == io.EOF {
						return
					}
					require.NoError(t, err)
					require.False(t, pass, "Got extraneous data after passing condition")

					// Check if we got the expected value.
					require.Zero(t, bytes.Compare(line, message), "Buffer mismatch")
					wgPass.Done()
					pass = true
				}
			}(i)
		}

		// Wait for readers to be ready.
		wgReady.Wait()
		// Wait for job to be ready.
		jobReadyWg.Wait()
		// Send output.
		jobWriter.Write(message)
		// Then wait for passing condition.
		wgPass.Wait()
	})

	t.Run("Readers created and closed sequentially", func(t *testing.T) {
		const numReaders = 1000
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

		// Create a mock job and capture the writer object.
		mockJob := &MockJob{}
		var jobWriter io.Writer
		var jobStartWg sync.WaitGroup
		jobStartWg.Add(1)
		mockJob.On("Start", mock.Anything, mock.Anything).Once().
			Run(func(args mock.Arguments) {
				jobWriter = args.Get(1).(io.Writer)
				jobStartWg.Done()
			}).
			Return(nil)
		mockJob.On("Stop", mock.Anything).Once().Return(nil)

		id, err := runner.Run(ctx, mockJob)
		require.NoError(t, err)

		// Simulate job output, create a reader, verify total job output, close
		// reader.
		jobStartWg.Wait()
		message := []byte("Foobar\n")
		accumulator := bytes.NewBuffer(nil)
		readBuf := make([]byte, numReaders * len(message))

		for i := 0; i < numReaders; i++ {
			reader, err := runner.NewReader(id)
			require.NoError(t, err)

			jobWriter.Write(message)
			accumulator.Write(message)
			var readCount int

			for {
				n, err := reader.Read(readBuf[readCount:])
				require.NoError(t, err)
				readCount += n

				// Sometimes the Write doesn't pipe to the Read fast
				// enough.  Retry until success.
				if readCount == accumulator.Len() {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}

			assert.Equal(t, accumulator.Bytes(), readBuf[:readCount], fmt.Sprintf("i=%d", i))

			reader.Close()
		}

		// Clean up.
		err = runner.Stop(ctx, id)
		require.NoError(t, err)

		mockJob.AssertExpectations(t)
	})
}
