## Steve Jobs
A tmux/screen like job running system

The idea here is that this library would be used in a service to facilitate starting
a local job remotely via HTTP or Websockets and allow the client to disconnect then
reconnect later and see the previous output and stream any further output. (like
tmux or screen sessions in ssh)

Users create a job with a Start and Stop method
```go
type Job interface {
    // Start the job, returns an error if the job failed to start or context was cancelled
    Start(context.Context, io.Writer) error

    // Stop the job, returns an error if the context was cancelled before job was stopped
    Stop(context.Context) error
}
```

The job uses the provided `io.Writer` for any output which will be saved into the job
buffer which can be broadcast to any clients who are connected via `io.ReadClosers` 
to the buffer.

Once the job is started remote clients via HTTP or Websockets can read the job buffer
by calling.
```go
reader, err := jobRunner.NewReader("job-id")
```
Clients should read from this returned `reader` until `io.EOF`, indicating the job
is complete and no more output can be read. Reads to this reader will block until new
data in the buffer is available to read. This will make it simple to stream data back
to clients via what ever transport the implementor has choosen, GRPC, HTTP, or Websockets.

The library is designed to allow multiple clients to read from the same buffer
simultaneously, in this way many clients can monitor the progress of a job in real time.







