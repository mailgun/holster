# Functional Test Framework
`go test`-like functional testing framework.

## Why use `functional`?
`functional` is suggested when you want to rapidly develop code in the format
of a unit test or benchmark test using existing testing tools, but run it
outside of the `go test` environment.  This is handy for use cases needing data
inspection and having low tolerance for errors.

`go test` doesn't support running tests programmatically from compiled code; it
requires the source code, which won't/shouldn't be available in production.

One such use case: runtime health check.  An admin may remotely invoke a health
check job and watch it run.  `functional` can manage the health check logic
once the RPC request is received.

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
