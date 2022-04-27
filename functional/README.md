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
### In-process Execution
Run the functional test inside a Steve Job in-process.

```go
import (
	"github.com/mailgun/holster/v4/functional"
	"github.com/mailgun/holster/v4/steve"
	"github.com/stretchr/testify/require"
)

myAction := new(MyAction)
myJob := steve.NewEZJob(myAction)

runner := steve.NewRunner(1000)
taskId, err := runner.Run(ctx, myJob)

// ...

type MyAction struct{}

func (a *MyAction) Run(ctx context.Context, writer io.Writer) error {
	tests := []functional.TestFunc{myTest1}
	functional.RunSuite(ctx, "My suite", tests)
}

func (a *MyAction) Status(status steve.Status) {
}
```

### Out-of-process Execution
Run the same test in a separate process.  This has the advantage of protection
should ensure any faulty tests would not bring down the parent process.

```go
import (
	"github.com/mailgun/holster/v4/steve"
	"github.com/sirupsen/logrus"
)

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
