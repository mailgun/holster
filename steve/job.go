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
	"context"
	"io"
	"time"
)

type Status struct {
	ID      ID        `json:"id"`
	Running bool      `json:"running"`
	Started time.Time `json:"created"`
	Stopped time.Time `json:"stopped"`
	Error   error     `json:"error"`
}

type Job interface {
	// Start the job in background and return immediately.
	// Returns an error if the job failed to start or context was canceled.
	Start(context.Context, io.Writer, *JobCloser) error

	// Stop the job and wait for stop.
	// Returns an error if the context was cancelled before job was stopped.
	Stop(context.Context) error

	// TODO: Add `Status()` method.
	// Returns running flag, finish time, success flag, error message.
}

type ID string

// Runner provides a job running service which runs a single job. The job is provided a writer which
// is buffered and stored for live monitoring or later retrieval. A client interested in a job may
// request a reader, then close it, then request a new reader and resume monitoring the output
// of the job. In this way long running jobs can be monitored for output, disconnect and resume
// monitoring later.
type Runner interface {
	// Run the provided job, returning a ID which can be used to track the status of a job.
	// Returns an error if the job failed to start of context was cancelled.
	Run(context.Context, Job) (ID, error)

	// NewStreamingReader returns an io.Reader which can be read to get the
	// most current output from a running job.  Job runner supports multiple
	// readers for the same job. In this way multiple remote clients may
	// monitor the output of the job simultaneously.  Reader will return io.EOF
	// when all output has been read and the job is stopped.
	// Be sure to close the reader when done reading.
	NewStreamingReader(id ID, offset int) (io.ReadCloser, error)

	// NewReader returns an io.Reader which reads the current job output.
	// Reader will return io.EOF when it reaches the end of the output buffer.
	// Multiple readers from NewReader and StreamReader can coexist to read
	// from the same job output.
	// Be sure to close the reader when done reading.
	NewReader(id ID, offset int) (io.ReadCloser, error)

	// OutputLen returns length of the job's output buffer.
	OutputLen(id ID) (n int, exists bool)

	// Stop a currently running job, returns an error if the context was cancelled before the job stopped.
	Stop(context.Context, ID) error

	// Done returns a channel that closes when the job stops.
	Done(ID) (done <-chan struct{}, exists bool)

	// Close all currently running jobs.
	Close(context.Context) error

	// Status returns the status of the job, returns false if the job doesn't exist.
	Status(ID) (status Status, exists bool)

	// List all jobs.
	List() []Status
}
