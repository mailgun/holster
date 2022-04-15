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
	ErrJobNotFound   = errors.New("no such job found")
	ErrJobNotRunning = errors.New("job not running")
	ErrTooManyJobs   = errors.New("too many jobs")

	readerCounter int64
)

// Runtime state for a running Job.
type jobIO struct {
	// Mutex used to synchronize status field.
	sync.RWMutex
	br     syncutil.Broadcaster
	writer io.WriteCloser
	buffer bytes.Buffer
	id     ID
	job    Job
	status Status

	// Run() will close stopChan to signal a stop request has completed.
	stopChan chan struct{}
}

type runner struct {
	sync.Mutex
	capacity int
	jobs     *collections.LRUCache
	wg       syncutil.WaitGroup
}

func NewJobRunner(capacity int) Runner {
	return &runner{
		capacity: capacity,
		jobs:     collections.NewLRUCache(capacity),
	}
}

func (r *runner) Run(ctx context.Context, job Job) (ID, error) {
	// FIXME: How to clean up jobs cache once at capacity?
	if r.jobs.Size() >= r.capacity {
		return "", ErrTooManyJobs
	}

	reader, writer := io.Pipe()

	id := ID(uuid.New().String())
	j := &jobIO{
		id:     id,
		br:     syncutil.NewBroadcaster(),
		writer: writer,
		job:    job,
		status: Status{
			ID: id,
		},
		stopChan: make(chan struct{}),
	}

	startChan := make(chan struct{})

	r.wg.Go(func() {
		ch := make(chan []byte)
		j.Lock()
		j.status.Running = true
		j.status.Started = time.Now()
		j.Unlock()
		close(startChan)

		defer func() {
			// Stop job on exit.
			j.Lock()
			j.status.Running = false
			j.status.Stopped = time.Now()
			j.Unlock()
			close(j.stopChan)
		}()

		// Spawn a separate go routine as the read could block forever
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

	r.jobs.Add(j.id, j)

	closer := &JobCloser{
		job:    j,
		writer: writer,
	}

	if err := job.Start(ctx, writer, closer); err != nil {
		return "", errors.Wrap(err, "error in job.Start")
	}

	// Wait for job to start.
	select {
	case <-startChan:
		return id, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (r *runner) NewReader(id ID, offset int) (io.ReadCloser, error) {
	if offset < 0 {
		return nil, errors.New("invalid offset")
	}

	r.Lock()
	defer r.Unlock()

	obj, ok := r.jobs.Get(id)
	if !ok {
		return nil, ErrJobNotFound
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

func (r *runner) NewStreamingReader(id ID, offset int) (io.ReadCloser, error) {
	if offset < 0 {
		return nil, errors.New("invalid offset")
	}

	r.Lock()
	defer r.Unlock()

	obj, ok := r.jobs.Get(id)
	if !ok {
		return nil, ErrJobNotFound
	}

	j := obj.(*jobIO)
	readerId := atomic.AddInt64(&readerCounter, 1)
	broadcastId := fmt.Sprintf("%s-%d", string(j.id), readerId)

	// If the job isn't running, fall back to non-streaming reader.
	j.RLock()
	if offset > j.buffer.Len() {
		return nil, fmt.Errorf("offset beyond upper bound %d", j.buffer.Len())
	}
	running := j.status.Running
	if !running {
		j.RUnlock()
		return r.NewReader(id, offset)
	}
	j.br.WaitChan(broadcastId)
	j.RUnlock()

	// Create a go routine that sends all unread bytes to the reader then
	// waits for new bytes to be written to the j.buffer via the broadcaster.
	reader, writer := io.Pipe()

	go func() {
		defer writer.Close()
		defer func() {
			j.br.Remove(broadcastId)
		}()

		var idx = 0
		var dst []byte

		for {
			// Grab any bytes from the buffer we haven't sent to our reader
			j.RLock()
			bufLen := j.buffer.Len()
			newBytesCount := bufLen - idx
			if newBytesCount > 0 {
				src := j.buffer.Bytes()
				dst = make([]byte, newBytesCount)
				copy(dst, src[idx:bufLen])
			}
			j.RUnlock()

			// Preform the write outside the mutex as it could block and we don't
			// want to hold on to the mutex for long
			if newBytesCount > 0 {
				n, err := writer.Write(dst)
				if err != nil {
					// If the reader called Close() on the pipe
					return
				}
				idx += n
			}

			// Check if job was stopped.
			select {
			case <-j.stopChan:
				writer.Close()
				return
			default:
			}

			// Wait for broadcaster to tell us there are new bytes to read.
			j.br.Wait(broadcastId)
		}
	}()

	return reader, nil
}

func (r *runner) OutputLen(id ID) (n int, exists bool) {
	r.Lock()
	defer r.Unlock()

	obj, ok := r.jobs.Get(id)
	if !ok {
		return 0, false
	}

	j := obj.(*jobIO)
	j.RLock()
	defer j.RUnlock()

	return j.buffer.Len(), true
}

func (r *runner) Stop(ctx context.Context, id ID) error {
	r.Lock()
	defer r.Unlock()

	obj, ok := r.jobs.Get(id)
	if !ok {
		return ErrJobNotFound
	}
	j := obj.(*jobIO)

	// Ignore if already stopped
	j.RLock()
	running := j.status.Running
	j.RUnlock()
	if !running {
		return ErrJobNotRunning
	}

	return r.stop(ctx, j)
}

func (r *runner) stop(ctx context.Context, j *jobIO) error {
	// Stop the job
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

// Done returns a channel that closes when the job stops.
func (r *runner) Done(id ID) (done <-chan struct{}, exists bool) {
	obj, ok := r.jobs.Get(id)
	if !ok {
		return nil, false
	}

	j := obj.(*jobIO)
	return j.stopChan, true
}

// Status returns the status of the job, returns false if the job doesn't exist.
// Status gets job status by id.
// Returns bool as ok flag.
func (r *runner) Status(id ID) (status Status, exists bool) {
	obj, ok := r.jobs.Get(id)
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
	r.jobs.Each(1, func(key interface{}, value interface{}) error {
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
		obj, ok := r.jobs.Get(s.ID)
		if !ok {
			continue
		}
		j := obj.(*jobIO)

		j.RLock()
		running := j.status.Running
		j.RUnlock()
		if running {
			// Stop running job.
			if err := r.stop(ctx, j); err != nil {
				return errors.Wrap(err, fmt.Sprintf("while stopping job id '%s'", j.id))
			}
		}
	}

	return nil
}
