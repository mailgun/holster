# Steve Jobs
A generic batch job running system inspired by tmux.

The idea here is that this library would be used in a service to facilitate starting
a local job remotely via gRPC, such as health checks or benchmarks.  And then
allow the client to connect later and see the output and stream any further
output (like tmux or screen sessions).

## Basic Usage
### Implement a Job
Implement a `steve.Job` interface, which provides start/stop methods.

The start method should create a task to run the job in a goroutine or similar
and return quickly.

Job logic can send output to the provided writer.  Be sure to call the closer
before exiting to indicate the task is done.

The stop method signals the task to close and exit.

### Implement Using EZJob
Or, use `steve.NewEZJob`.  Just implement the job logic as interface
`steve.Action`.  `NewEZJob` handles goroutine management, catching panics, and
status changes.

```go
import (
	"ctx"
	"io"

	"github.com/mailgun/holster/v4/steve"
)

myAction := new(MyAction)
myJob := steve.NewEZJob(myAction)

// ...

type MyAction struct{}

func (a *MyAction) Run(ctx context.Context, writer io.Writer) error {
	io.WriteString(writer, "Hello World.\n")
	return nil
}

func (a *MyAction) Status(status steve.Status) {
}
```

### Run the Job
Launch the job with `Runner.Run()`, which returns a task id.

```go
import (
	"github.com/mailgun/holster/v4/steve"
)

runner := steve.NewRunner(1000)
taskId, err := runner.Run(ctx, myJob)
```

The runner captures task output, which can be read using the `io.Reader`
returned from `Runner.NewStreamingReader()`.

The reader will read all output then block, continuously reading until the
task is finished.  Clients should read the `io.Reader` until `err == io.EOF`,
indicating the task is done and no more output can be read.

Multiple readers may be created against a running task simultaneously.  In this
way, many clients can monitor the progress of a job in real time.

## gRPC Service
Jobs can be managed remotely via gRPC endpoints.

### Methods
See `steve.JobV1Client` interface.

#### `HealthCheck`
Returns successful response if server is running.

#### `GetJobs`
Paginate jobs available in the server.

#### `StartTask`
Start a task from job id.

#### `StopTask`
Stop a task by task id.

#### `GetRunningTasks`
Paginate and filter running tasks.

#### `GetStoppedTasks`
Paginate and filter stopped tasks.

#### `GetTaskOutput`
Paginate task output line by line.

#### `GetStreamingTaskOutput`
Paginate task output line by line.  Blocks waiting for new output until task is
finished.

### Register Service Endpoints
Register the `steve.LocalJobsV1Server` object to a new or existing gRPC server.

```go
import (
	"github.com/mailgun/holster/v4/steve"
	"google.golang.org/grpc"
)

// Define jobs.
jobMap := map[steve.JobId]steve.Job{
	"job1": NewJob1(),
	"job2": NewJob2(),
}

// Create gRPC server.
grpcSrv = grpc.NewServer()
jobsSrv := steve.NewLocalJobsV1Server(jobMap)
steve.RegisterJobsV1Server(grpcSrv, jobsSrv)
```

### Using gRPC Client
```go
import (
	"github.com/mailgun/holster/v4/steve"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

endpoint := "myserver:8081"
opts := []grpc.DialOption{
	grpc.WithInsecure(),
}
conn, err := grpc.Dial(endpoint, opts...)
client := steve.NewJobsV1Client(conn)

resp, err := client.HealthCheck(ctx, new(emptypb.Empty))
```

### Using jobs-cli Client
A generic CLI tool `jobs-cli` can perform common tasks via the Jobs API.

Note: This tool is meant to be used as an admin tool on a private trusted
network.  Therefore, it is configured to allow insecure TLS connection with the
server.

```sh
$ make build
$ ./jobs-cli -h
```

```sh
# List jobs.
$ ./jobs-cli --endpoint myserver:8081 ls
```

```sh
# Run a job.  Displays task output until finished.
$ ./jobs-cli --endpoint myserver:8081 run job1

# Run a job detached.  No task output.
$ ./jobs-cli --endpoint myserver:8081 run -d job1
```

Note: `run` command prints task id, which can be used by commands requiring it.

```sh
# Show task output.
$ ./jobs-cli --endpoint myserver:8081 logs <task-id>

# Follow running task output until finished.
$ ./jobs-cli --endpoint myserver:8081 logs -f <task-id>
```
