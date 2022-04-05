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
	j := jobIO{
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
					j.Lock()
					j.status.Running = false
					j.Unlock()
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

	r.jobs.Add(j.id, &j)

	if err := job.Start(ctx, writer); err != nil {
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

// NewReader creates a reader that starts at the beginning of the job's output
// log.
func (r *runner) NewReader(id ID) (io.ReadCloser, error) {
	r.Lock()
	defer r.Unlock()

	readerId := atomic.AddInt64(&readerCounter, 1)

	obj, ok := r.jobs.Get(id)
	if !ok {
		return nil, ErrJobNotFound
	}
	j := obj.(*jobIO)
	broadcastId := fmt.Sprintf("%s-%d", string(j.id), readerId)

	// If the job isn't running, then copy the current buffer
	// into a read closer and return that to the caller.
	j.Lock()
	running := j.status.Running
	if !running {
		j.Unlock()
		buf := bytes.Buffer{}
		buf.Write(j.buffer.Bytes())
		return ioutil.NopCloser(&buf), nil
	}
	j.br.WaitChan(broadcastId)
	j.Unlock()

	// Create a go routine that sends all unread bytes to the reader then
	// waits for new bytes to be written to the j.buffer via the broadcaster.
	reader, writer := io.Pipe()

	go func() {
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

// Status gets job status by id.
// Returns bool as ok flag.
func (r *runner) Status(id ID) (Status, bool) {
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
