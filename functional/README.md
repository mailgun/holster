# Functional Test Framework
`go test`-like functional testing framework.

Use to develop test cases that need to run outside of the `go test`
environment.

The original intended use case is to implement tests to be executed using the
Steve Job framework in the `steve` package.

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
