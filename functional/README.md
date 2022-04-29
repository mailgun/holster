# Functional Test Framework
`go test`-like functional testing framework.

## Why use `functional`?
`functional` is used when you want to rapidly develop code in the format of a
unit test using existing testing tools, but run it outside of the `go test`
environment.

`go test` doesn't support running tests programmatically from compiled code; it
requires the source code, which won't be available in production.

The original intended use case is to implement tests to run as a background job
within the Steve Job framework in the `steve` package.

One such use case: runtime health check.  An admin may remotely invoke a health
check job and watch it run.

Tools like Testify may be used for assertions and mocking.

## Run Tests
```go
import (
	"context"
	"time"

	"github.com/mailgun/holster/v4/functional"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Minute)
	defer cancel()

	tests := []functional.TestFunc{
		myTest1,
	}
	functional.RunSuite(ctx, "My suite", tests)
}

func myTest1(t *functional.T) {
	t.Log("Hello World.")
}
```

## Testify Assertions
Testify is compatible with the functional testing framework as-is.

```go
import (
	"github.com/mailgun/holster/v4/functional"
	"github.com/stretchr/testify/require"
)

func myTest1(t *functional.T) {
	retval := DoSomething()
	require.Equal(t, "OK", retval)
}
```

## Run in a Steve Job
Jobs can be run natively (in-process) or can be writen in a separate executable
and called by a job.

### In-process Execution
Run the functional test inside a Steve Job in-process.

```go
import (
	"context"
	"time"

	"github.com/mailgun/holster/v4/functional"
	"github.com/mailgun/holster/v4/steve"
	"github.com/stretchr/testify/require"
)

ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Minute)
defer cancel()
myAction := new(MyAction)
myJob := steve.NewEZJob(myAction)
runner := steve.NewRunner(1000)
taskId, err := runner.Run(ctx, myJob)

// ...

type MyAction struct{}

func (a *MyAction) Run(ctx context.Context, writer io.Writer) error {
	tests := []functional.TestFunc{myTest1}
	functional.RunSuite(ctx, "My suite", tests)
	return nil
}

func (a *MyAction) Status(status steve.Status) {
}
```

### Out-of-process Execution
Run the same test in a separate process.  This has the advantage of protection
to ensure any faulty tests would not bring down the parent process.  Also,
using `ExecJob` type will capture all STDOUT/STDERR as job output, whereas a
native job only captures what is sent to its `writer`..

```go
import (
	"context"
	"time"

	"github.com/mailgun/holster/v4/steve"
	"github.com/sirupsen/logrus"
)

ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Minute)
defer cancel()
myHandler := new(MyHandler)
myJob := steve.NewExecJob(myHandler, "mytest1")
runner := steve.NewRunner(1000)
taskId, err := runner.Run(ctx, myJob)

// ...

type MyHandler struct{}

func (h *MyHandler) Done(exitCode int) {
	logrus.Infof("Exit code: %d", exitCode)
}
```

Compile the test code to executable file `mytest1`:
```go
import (
	"github.com/mailgun/holster/v4/functional"
	"github.com/mailgun/holster/v4/steve"
)

func main() {
	tests := []functional.TestFunc{myTest1}
	functional.RunSuite(ctx, "My suite", tests)
}
```
