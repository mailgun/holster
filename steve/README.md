# Steve Jobs
A generic tmux/screen like batch job running system

The idea here is that this library would be used in a service to facilitate starting
a local job remotely via HTTP or Websockets and allow the client to disconnect then
reconnect later and see the previous output and stream any further output. (like
tmux or screen sessions in ssh)

One intended use case is to expose these jobs via service endpoints over HTTP,
gRPC, etc. to invoke batch jobs, such as health checks or benchmarks.

## Usage
Implement a `steve.Job` interface, which provides start/stop methods.

Then, launch the job with `Runner.Run()`, which returns a job id.

The runner captures and buffers job output, which can be read using
the `io.Reader` returned from `Runner.NewReader()`.

Clients should read the `io.Reader` until `err == io.EOF`, indicating the job
is complete and no more output can be read.  Reads from this reader will block
until new data in the buffer is available to read or EOF.

The reader is designed to allow multiple clients to read from the same buffer
simultaneously.  In this way, many clients can monitor the progress of a job in
real time.
