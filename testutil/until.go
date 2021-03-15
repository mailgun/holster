package testutil

import (
	"fmt"
	"net"
	"time"
)

type TestingT interface {
	Errorf(format string, args ...interface{})
	FailNow()
}

type TestResults struct {
	T        TestingT
	Failures []string
}

func (s *TestResults) Errorf(format string, args ...interface{}) {
	s.Failures = append(s.Failures, fmt.Sprintf(format, args...))
}

func (s *TestResults) FailNow() {
	s.Report(s.T)
	s.T.FailNow()
}

func (s *TestResults) Report(t TestingT) {
	for _, failure := range s.Failures {
		t.Errorf(failure)
	}
}

// Return true if the test eventually passed, false if the test failed
func UntilPass(t TestingT, attempts int, duration time.Duration, callback func(t TestingT)) bool {
	results := TestResults{T: t}

	for i := 0; i < attempts; i++ {
		// Clear the failures before each attempt
		results.Failures = nil

		// Run the tests in the callback
		callback(&results)

		// If the test had no failures
		if len(results.Failures) == 0 {
			return true
		}
		// Sleep the duration
		time.Sleep(duration)
	}
	// We have exhausted our attempts and should report the failures and exit
	results.Report(t)
	return false
}

// Continues to attempt connecting to the specified tcp address until either
// a successful connect or attempts are exhausted
func UntilConnect(t TestingT, a int, d time.Duration, addr string) {
	for i := 0; i < a; i++ {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			continue
		}
		conn.Close()
		// Sleep the duration
		time.Sleep(d)
		return
	}
	t.Errorf("never connected to TCP server at '%s' after %d attempts", addr, a)
	t.FailNow()
}
