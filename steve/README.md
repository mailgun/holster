# Steve Jobs
A generic batch job running system inspired by tmux.

The idea here is that this library would be used in a service to facilitate starting
a local job remotely via HTTP or gRPC, such health checks or benchmarks.  And
then allow the client to disconnect then reconnect later and see the previous
output and stream any further output (like tmux or screen sessions).

## Usage
### Implement a Job
Implement a `steve.Job` interface, which provides start/stop methods.

The start method should create a task to run the job in a goroutine or similar
and return quickly.

Job logic can send output to the provided writer.  Be sure to call the closer
before exiting to indicate the task is done.

The stop method signals the task to close and exit.

### Run the Job
Launch the job with `Runner.Run()`, which returns a task id.

The runner captures task output, which can be read using the `io.Reader`
returned from `Runner.NewReader()`.

Or use `Runner.NewStreamingReader()` to block, continuously reading until the
task is finished.  Clients should read the `io.Reader` until `err == io.EOF`,
indicating the job is done and no more output can be read.  Reads from this
reader will block until new data is available or EOF.

Multiple readers may be created against a running task simultaneously.  In this
way, many clients can monitor the progress of a job in real time.
