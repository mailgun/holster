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
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/mailgun/holster/v4/collections"
	"github.com/mailgun/holster/v4/errors"
	"github.com/mailgun/holster/v4/syncutil"
)

var (
	ErrTaskNotFound   = errors.New("no such task found")
	ErrTaskNotRunning = errors.New("task not running")
	ErrTooManyTasks   = errors.New("too many tasks")

	readerCounter int64
)

// Runtime state for a running Job.
type jobIO struct {
	// Mutex used to synchronize status field.
	sync.RWMutex
	br     syncutil.Broadcaster
	writer io.WriteCloser
	buffer bytes.Buffer
	taskId TaskId
	job    Job
	status Status

	// Run() will close stopChan to signal a stop request has completed.
	stopChan chan struct{}
}

type runner struct {
	sync.Mutex
	capacity int
	tasks    *collections.LRUCache
	wg       syncutil.WaitGroup
}

func NewJobRunner(capacity int) Runner {
	return &runner{
		capacity: capacity,
		tasks:    collections.NewLRUCache(capacity),
	}
}

// Run a `Job`.
// Job concurrency limited to capacity set in `NewJobRunner`.  Additional calls
// to `Run` will result in the least used task to be rolled off the cache and
// no longer watched.  This may have indeterminate effects as the expired task
// will continue to run if it hasn't already stopped.
func (r *runner) Run(ctx context.Context, job Job) (TaskId, error) {
	reader, writer := io.Pipe()

	taskId := TaskId(uuid.New().String())
	j := &jobIO{
		taskId: taskId,
		br:     syncutil.NewBroadcaster(),
		writer: writer,
		job:    job,
		status: Status{
			TaskId: taskId,
		},
		stopChan: make(chan struct{}),
	}

	startChan := make(chan struct{})

	r.wg.Go(func() {
		ch := make(chan []byte)
		updateStatus(j, func(status *Status) {
			status.Running = true
			status.Started = time.Now()
		})
		close(startChan)

		defer func() {
			// Stop task on exit.
			updateStatus(j, func(status *Status) {
				status.Running = false
				status.Stopped = time.Now()
			})
			close(j.stopChan)
		}()

		// Spawn a separate goroutine as the read could block forever
		go func() {
			defer close(ch)
			buf := make([]byte, 2024)
			for {
				n, err := reader.Read(buf)
				if err != nil {
					return
				}
				out := make([]byte, n)
				copy(out, buf[:n])
				ch <- out
			}
		}()

		for {
			select {
			case line, ok := <-ch:
				if !ok {
					// Channel closed.
					j.br.Broadcast()
					return
				}

				j.Lock()
				j.buffer.Write(line)
				j.Unlock()
				j.br.Broadcast()
			}
		}
	})

	r.tasks.Add(j.taskId, j)

	closer := &TaskCloser{
		job:    j,
		writer: writer,
	}

	if err := job.Start(ctx, writer, closer); err != nil {
		return "", errors.Wrap(err, "error in job.Start")
	}

	// Send initial status once task has started.
	updateStatus(j, nil)

	// Wait for task to start.
	select {
	case <-startChan:
		return taskId, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (r *runner) NewReader(taskId TaskId, offset int) (io.ReadCloser, error) {
	if offset < 0 {
		return nil, errors.New("invalid offset")
	}

	r.Lock()
	defer r.Unlock()

	obj, ok := r.tasks.Get(taskId)
	if !ok {
		return nil, ErrTaskNotFound
	}

	j := obj.(*jobIO)
	j.RLock()
	defer j.RUnlock()

	if offset > j.buffer.Len() {
		return nil, fmt.Errorf("offset beyond upper bound %d", j.buffer.Len())
	}

	buf := bytes.Buffer{}
	output := j.buffer.Bytes()[offset:]
	buf.Write(output)
	return ioutil.NopCloser(&buf), nil
}

func (r *runner) NewStreamingReader(taskId TaskId, offset int) (io.ReadCloser, error) {
	if offset < 0 {
		return nil, errors.New("invalid offset")
	}

	r.Lock()
	defer r.Unlock()

	obj, ok := r.tasks.Get(taskId)
	if !ok {
		return nil, ErrTaskNotFound
	}

	j := obj.(*jobIO)
	readerId := atomic.AddInt64(&readerCounter, 1)
	broadcastId := fmt.Sprintf("%s-%d", string(j.taskId), readerId)
	broadcastChan := j.br.WaitChan(broadcastId)

	// Create a go routine that sends all unread bytes to the reader then
	// waits for new bytes to be written to the j.buffer via the broadcaster.
	reader, writer := io.Pipe()

	go func() {
		defer func() {
			j.br.Remove(broadcastId)
			writer.Close()
		}()

		var dst []byte

		for {
			// Grab any bytes from the buffer we haven't sent to our reader
			j.RLock()
			bufLen := j.buffer.Len()
			newBytesCount := bufLen - offset
			if newBytesCount > 0 {
				src := j.buffer.Bytes()
				dst = make([]byte, newBytesCount)
				copy(dst, src[offset:bufLen])
			}
			j.RUnlock()

			// Perform the write outside the mutex as it could block and we don't
			// want to hold on to the mutex for long
			if newBytesCount > 0 {
				n, err := writer.Write(dst)
				if err != nil {
					// If the reader called Close() on the pipe
					return
				}
				offset += n
				continue
			}

			// Wait for broadcaster to tell us there are new bytes to read.
			select {
			case <-broadcastChan:
				// Broadcast signals new data available.
			case <-j.stopChan:
				// Task stopped.
				return
			}
		}
	}()

	return reader, nil
}

func (r *runner) OutputLen(id TaskId) (n int, exists bool) {
	r.Lock()
	defer r.Unlock()

	obj, ok := r.tasks.Get(id)
	if !ok {
		return 0, false
	}

	j := obj.(*jobIO)
	j.RLock()
	defer j.RUnlock()

	return j.buffer.Len(), true
}

func (r *runner) Stop(ctx context.Context, taskId TaskId) error {
	r.Lock()
	defer r.Unlock()

	obj, ok := r.tasks.Get(taskId)
	if !ok {
		return ErrTaskNotFound
	}
	j := obj.(*jobIO)

	// Ignore if already stopped
	j.RLock()
	running := j.status.Running
	j.RUnlock()
	if !running {
		return ErrTaskNotRunning
	}

	return r.stop(ctx, j)
}

func (r *runner) stop(ctx context.Context, j *jobIO) error {
	// Stop the task
	if err := j.job.Stop(ctx); err != nil {
		return err
	}

	// Close the writer, this tells the reader goroutine in Run() to shutdown.
	j.writer.Close()

	// Wait for stop.
	select {
	case <-j.stopChan:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Done returns a channel that closes when the task stops.
func (r *runner) Done(taskId TaskId) (done <-chan struct{}, exists bool) {
	obj, ok := r.tasks.Get(taskId)
	if !ok {
		return nil, false
	}

	j := obj.(*jobIO)
	return j.stopChan, true
}

// Status returns the status of the task, returns false if the task doesn't
// exist.
// Status gets task status by id.
// Returns bool as exists flag.
func (r *runner) Status(taskId TaskId) (status Status, exists bool) {
	obj, ok := r.tasks.Get(taskId)
	if !ok {
		return Status{}, false
	}
	j := obj.(*jobIO)
	j.RLock()
	defer j.RUnlock()
	return j.status, true
}

func (r *runner) List() []Status {
	r.Lock()
	defer r.Unlock()

	return r.list()
}

func (r *runner) list() []Status {
	var result []Status
	r.tasks.Each(1, func(key interface{}, value interface{}) error {
		j := value.(*jobIO)
		j.RLock()
		defer j.RUnlock()

		result = append(result, j.status)
		return nil
	})

	return result
}

// Close all currently running jobs.
func (r *runner) Close(ctx context.Context) error {
	r.Lock()
	defer r.Unlock()

	for _, s := range r.list() {
		obj, ok := r.tasks.Get(s.TaskId)
		if !ok {
			continue
		}
		j := obj.(*jobIO)

		j.RLock()
		running := j.status.Running
		j.RUnlock()
		if running {
			// Stop running task.
			if err := r.stop(ctx, j); err != nil {
				return errors.Wrap(err, fmt.Sprintf("while stopping task id '%s'", j.taskId))
			}
		}
	}

	return nil
}

// Callback updates status, then signal a status change event.
func updateStatus(j *jobIO, fn func(status *Status)) {
	j.Lock()
	if fn != nil {
		fn(&j.status)
	}
	status := j.status
	j.Unlock()
	j.job.Status(status)
}
