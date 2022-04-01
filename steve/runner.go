package steve

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mailgun/holster/v4/collections"
	"github.com/mailgun/holster/v4/syncutil"
	"github.com/pkg/errors"
)

var (
	ErrJobNotFound   = errors.New("no such job found")
	ErrJobNotRunning = errors.New("job not running")
	ErrTooManyJobs   = errors.New("too many jobs")
)

type jobIO struct {
	sync.RWMutex
	br       syncutil.Broadcaster
	writer   io.WriteCloser
	buffer   bytes.Buffer
	id       ID
	job      Job
	status   Status
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

	// Spawn a go routine to monitor job output, storing the output into the j.buffer
	r.wg.Go(func() {
		ch := make(chan []byte)
		j.Lock()
		j.status.Running = true
		j.status.Started = time.Now()
		j.Unlock()
		close(startChan)

		// Pipe data from reader to channel.
		go func() {
			buf := make([]byte, 2024)
			for {
				n, err := reader.Read(buf)
				if err != nil {
					close(ch)
					return
				}
				out := make([]byte, n)
				copy(out, buf[:n])
				ch <- out
			}
		}()

		// Read job output from channel, broadcast to readers.
	readLoop:
		for {
			select {
			case line, ok := <-ch:
				if !ok {
					break readLoop
				}

				j.Lock()
				j.buffer.Write(line)
				j.Unlock()
				j.br.Broadcast()

			case <-ctx.Done():
				break readLoop
			}
		}

		// Job stopped
		j.Lock()
		j.status.Running = false
		j.status.Stopped = time.Now()
		j.Unlock()
		close(j.stopChan)
		j.br.Broadcast()
		return
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

var readerCounter int64

func (r *runner) NewReader(id ID) (io.ReadCloser, error) {
	r.Lock()
	defer r.Unlock()

	obj, ok := r.jobs.Get(id)
	if !ok {
		return nil, ErrJobNotFound
	}
	j := obj.(*jobIO)

	// If the job isn't running, then copy the current buffer
	// into a read closer and return that to the caller.
	j.RLock()
	running := j.status.Running
	j.RUnlock()
	if !running {
		j.Lock()
		defer j.Unlock()
		buf := bytes.Buffer{}
		buf.Write(j.buffer.Bytes())
		return ioutil.NopCloser(&buf), nil
	}

	// Create a go routine that sends all unread bytes to the reader then
	// waits for new bytes to be written to the j.buffer via the broadcaster.
	reader, writer := io.Pipe()
	r.wg.Go(func() {
		var idx = 0
		for {
			// Grab any bytes from the buffer we haven't sent to our reader
			j.RLock()
			running := j.status.Running
			src := j.buffer.Bytes()
			dst := make([]byte, j.buffer.Len()-idx)
			copy(dst, src[idx:j.buffer.Len()])
			j.RUnlock()

			// Preform the write outside the mutex as it could block and we don't
			// want to hold on to the mutex for long
			n, err := writer.Write(dst)
			if err != nil {
				// If the reader called Close() on the pipe
				return
			}
			idx += n

			// The job routine will broadcast when it stops the job and no
			// more bytes are available to read.
			if !running {
				writer.Close()
				return
			}

			// Wait for broadcaster to tell us there are new bytes to read.
			j.br.Wait(string(j.id))
		}
	})

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
