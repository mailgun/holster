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

	"github.com/mailgun/holster/v4/errors"
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
		mockJob.On("Start", mock.Anything, mock.Anything, mock.Anything).Once().Return(nil)
		mockJob.On("Stop", mock.Anything).Once().Return(nil)
		mockJob.On("Status", mock.Anything).Return()

		id, err := runner.Run(ctx, mockJob)
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		s, ok := runner.Status(id)
		require.True(t, ok)
		assert.Equal(t, id, s.TaskId)
		assert.True(t, s.Running)
		assert.False(t, s.Started.IsZero())
		assert.True(t, s.Stopped.IsZero())

		err = runner.Stop(ctx, id)
		require.NoError(t, err)

		// List should show the job as not running
		l := runner.List()
		require.Len(t, l, 1)
		assert.Equal(t, id, l[0].TaskId)
		assert.False(t, l[0].Running)

		mockJob.AssertExpectations(t)
	})

	t.Run("Job returned error", func(t *testing.T) {
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
		mockJob.On("Start", mock.Anything, mock.Anything, mock.Anything).Once().
			Run(func(args mock.Arguments) {
				closer := args.Get(2).(*steve.TaskCloser)
				closer.Close(errors.New("Foobar error"))
			}).
			Return(nil)
		mockJob.On("Status", mock.Anything).Return()

		id, err := runner.Run(ctx, mockJob)
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		// Wait for job to stop.
		var status steve.Status
		for {
			var ok bool
			status, ok = runner.Status(id)
			require.True(t, ok)
			require.Equal(t, id, status.TaskId)

			if !status.Running {
				break
			}
			time.Sleep(1 * time.Millisecond)
		}

		// Verify status.
		assert.False(t, status.Running)
		assert.False(t, status.Started.IsZero())
		assert.False(t, status.Stopped.IsZero())
		assert.Contains(t, "Foobar error", status.Error.Error())

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
		mockJob.On("Start", mock.Anything, mock.Anything, mock.Anything).Once().Return(nil)
		mockJob.On("Stop", mock.Anything).Once().Return(nil)
		mockJob.On("Status", mock.Anything).Return()

		id, err := runner.Run(ctx, mockJob)
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		err = runner.Close(ctx)
		require.NoError(t, err)

		s, ok := runner.Status(id)
		require.True(t, ok)
		assert.Equal(t, id, s.TaskId)
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

		err := runner.Stop(ctx, steve.TaskId("bogus"))
		require.Equal(t, steve.ErrTaskNotFound, err)
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
		mockJob.On("Start", mock.Anything, mock.Anything, mock.Anything).Once().Return(nil)
		mockJob.On("Stop", mock.Anything).Once().Return(nil)
		mockJob.On("Status", mock.Anything).Return()

		id, err := runner.Run(ctx, mockJob)
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		err = runner.Stop(ctx, id)
		require.NoError(t, err)

		err = runner.Stop(ctx, id)
		require.Equal(t, steve.ErrTaskNotRunning, err)

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
		startReaderChan := make(chan struct{})
		message := []byte("Foobar\n")
		jobReadyWg.Add(1)
		mockJob.On("Start", mock.Anything, mock.Anything, mock.Anything).Once().
			Run(func(args mock.Arguments) {
				jobWriter = args.Get(1).(io.Writer)
				jobReadyWg.Done()
			}).
			Return(nil)
		mockJob.On("Stop", mock.Anything).Once().Return(nil)
		mockJob.On("Status", mock.Anything).Return()

		id, err := runner.Run(ctx, mockJob)
		require.NoError(t, err)

		// Create multiple readers for the same job.
		var wgPass sync.WaitGroup
		for i := 0; i < numReaders; i++ {
			wgPass.Add(1)

			// Launch reader in goroutine.
			go func(i int) {
				// Wait to start.
				<-startReaderChan

				r, err := runner.NewReader(id, 0)
				require.NoError(t, err)
				defer r.Close()

				buf := bufio.NewReader(r)
				pass := false

				for {
					// Wait for next line of text.
					line, err := buf.ReadBytes('\n')
					if err == io.EOF {
						assert.True(t, pass, fmt.Sprintf("[%d] Unexpected EOF", i))
						return
					}
					require.NoError(t, err)
					require.False(t, pass, fmt.Sprintf("[%d] Got extraneous data after passing condition", i))

					// Check if we got the expected value.
					require.Equal(t, message, line, fmt.Sprintf("[%d] Buffer mismatch", i))
					wgPass.Done()
					pass = true
				}
			}(i)
		}

		// Wait for job to be ready.
		jobReadyWg.Wait()
		// Send output.
		jobWriter.Write(message)
		// Start readers.
		close(startReaderChan)
		// Then wait for passing condition.
		wgPass.Wait()
	})

	t.Run("Readers created and closed sequentially", func(t *testing.T) {
		const numReaders = 100
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
		mockJob.On("Start", mock.Anything, mock.Anything, mock.Anything).Once().
			Run(func(args mock.Arguments) {
				jobWriter = args.Get(1).(io.Writer)
				jobStartWg.Done()
			}).
			Return(nil)
		mockJob.On("Stop", mock.Anything).Once().Return(nil)
		mockJob.On("Status", mock.Anything).Return()

		id, err := runner.Run(ctx, mockJob)
		require.NoError(t, err)

		// Write job output
		// Create a reader
		// Verify total job output
		// Close reader
		// Repeat.
		jobStartWg.Wait()
		message := []byte("Foobar\n")
		accumulator := bytes.NewBuffer(nil)
		readBuf := make([]byte, numReaders*len(message))
		var lastReadCount int

		for i := 0; i < numReaders; i++ {
			jobWriter.Write(message)
			accumulator.Write(message)
			var readCount int
			var err error
			var reader io.ReadCloser

			for {
				reader, err = runner.NewReader(id, 0)
				require.NoError(t, err)

				n, err := reader.Read(readBuf[readCount:])
				if err != nil && err != io.EOF {
					require.NoError(t, err, fmt.Sprintf("[%d] error in reader.Read", i))
				}

				if n > lastReadCount {
					// Got new data.
					readCount += n

					require.Equal(t, accumulator.Len(), readCount, fmt.Sprintf("[%d] Read buffer size mismatch", i))
					break
				}

				// No new data.
				// Sometimes the Write doesn't pipe to the Read fast
				// enough and Read() returns no new data.
				// Retry until success.
				reader.Close()
				time.Sleep(1 * time.Millisecond)
			}

			reader.Close()
			lastReadCount = readCount
			assert.Equal(t, accumulator.Bytes(), readBuf[:readCount], fmt.Sprintf("[%d] Read buffer mismatch", i))
		}

		// Clean up.
		err = runner.Stop(ctx, id)
		require.NoError(t, err)

		mockJob.AssertExpectations(t)
	})

	t.Run("Wait for job to finish", func(t *testing.T) {
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
		var writer io.Writer
		mockJob.On("Start", mock.Anything, mock.Anything, mock.Anything).Once().
			Run(func(args mock.Arguments) {
				writer = args.Get(1).(io.Writer)
				require.NotNil(t, writer)
			}).
			Return(nil)
		mockJob.On("Stop", mock.Anything).Once().Return(nil)
		mockJob.On("Status", mock.Anything).Return()

		// Start job.
		id, err := runner.Run(ctx, mockJob)
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		// Check that Done() doesn't close.
		doneChan, exists := runner.Done(id)
		require.True(t, exists)
		select {
		case <-doneChan:
			require.Fail(t, "unexpected job stop")
		case <-time.After(10 * time.Millisecond):
			// Expected outcome.
		}

		// Stop the job.
		err = runner.Stop(ctx, id)
		require.NoError(t, err)

		// Wait for done signal.
		<-doneChan

		mockJob.AssertExpectations(t)
	})
}
