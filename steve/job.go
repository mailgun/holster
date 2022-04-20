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
	TaskId  TaskId    `json:"task_id"`
	Running bool      `json:"running"`
	Started time.Time `json:"created"`
	Stopped time.Time `json:"stopped"`
	Error   error     `json:"error"`
}

type Job interface {
	// Start the task in background and return immediately.
	// Returns an error if the task failed to start or context was canceled.
	Start(ctx context.Context, writer io.Writer, closer *TaskCloser) error

	// Stop the task and wait for stop.
	// Returns an error if the context was canceled before task was stopped.
	Stop(ctx context.Context) error

	// Status change notification.
	// Provides running flag, start/stop times, and error message.
	Status(status Status)
}

// TaskId of a running `Job`.
type TaskId string

// Runner provides a task running service which runs a job to create a task.
// The task is provided a writer which is buffered and stored for live
// monitoring or later retrieval. A client interested in a task may request a
// reader, then close it, then request a new reader and resume monitoring the
// output of the task. In this way long running jobs can be monitored for
// output, disconnect and resume monitoring later.
type Runner interface {
	// Run the provided job, returning a TaskId which can be used to track the
	// status of its task.
	// Returns an error if the task failed to start of context was canceled.
	Run(ctx context.Context, job Job) (TaskId, error)

	// NewStreamingReader returns an io.Reader which can be read to get the
	// most current output from a running task.  Job runner supports multiple
	// readers for the same task. In this way multiple remote clients may
	// monitor the output of the task simultaneously.  Reader will return io.EOF
	// when all output has been read and the taks is stopped.
	// Be sure to close the reader when done reading.
	NewStreamingReader(taskId TaskId, offset int) (io.ReadCloser, error)

	// NewReader returns an io.Reader which reads the current task output.
	// Reader will return io.EOF when it reaches the end of the output buffer.
	// Multiple readers from NewReader and StreamReader can coexist to read
	// from the same task output.
	// Be sure to close the reader when done reading.
	NewReader(taskId TaskId, offset int) (io.ReadCloser, error)

	// OutputLen returns length of the task's output buffer.
	OutputLen(taskId TaskId) (n int, exists bool)

	// Stop a currently running task, returns an error if the context was
	// canceled before the task stopped.
	Stop(ctx context.Context, taskId TaskId) error

	// Done returns a channel that closes when the task stops.
	Done(taskId TaskId) (done <-chan struct{}, exists bool)

	// Close all currently running tasks.
	Close(ctx context.Context) error

	// Status returns the status of the task, returns false if the task doesn't
	// exist.
	Status(taskId TaskId) (status Status, exists bool)

	// List all tasks.
	List() []Status
}
