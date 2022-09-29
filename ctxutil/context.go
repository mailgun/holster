package ctxutil

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
)

// WithTimeout calls context.WithTimeout and logs details of the
// deadline origin.
func WithDeadline(ctx context.Context, deadline time.Time) (context.Context, context.CancelFunc) {
	_, fn, line, _ := runtime.Caller(1)

	logrus.WithContext(ctx).
		WithFields(logrus.Fields{
			"deadline": deadline.Format(time.RFC3339),
			"source":   fmt.Sprintf("%s:%d", fn, line),
		}).
		Debug("Set context deadline")

	return context.WithDeadline(ctx, deadline)
}

// WithTimeout calls context.WithTimeout and logs details of the
// deadline origin.
func WithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	deadline := time.Now().Add(timeout)
	_, fn, line, _ := runtime.Caller(1)

	logrus.WithContext(ctx).
		WithFields(logrus.Fields{
			"deadline": deadline.Format(time.RFC3339),
			"source":   fmt.Sprintf("%s:%d", fn, line),
		}).
		Debug("Set context deadline")

	return context.WithTimeout(ctx, timeout)
}
