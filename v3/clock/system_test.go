package clock

import (
	"time"

	. "gopkg.in/check.v1"
)

type SystemSuite struct{}

var _ = Suite(&SystemSuite{})

func (s *SystemSuite) TestSleep(c *C) {
	start := Now()

	// When
	Sleep(100 * time.Millisecond)

	// Then
	if Now().Sub(start) < 100*time.Millisecond {
		c.Error("Sleep did not last long enough")
	}
}

func (s *SystemSuite) TestAfter(c *C) {
	start := Now()

	// When
	end := <-After(100 * time.Millisecond)

	// Then
	if end.Sub(start) < 100*time.Millisecond {
		c.Error("Sleep did not last long enough")
	}
}

func (s *SystemSuite) TestAfterFunc(c *C) {
	start := Now()
	endCh := make(chan time.Time, 1)

	// When
	AfterFunc(100*time.Millisecond, func() { endCh <- time.Now() })

	// Then
	end := <-endCh
	if end.Sub(start) < 100*time.Millisecond {
		c.Error("Sleep did not last long enough")
	}
}

func (s *SystemSuite) TestNewTimer(c *C) {
	start := Now()

	// When
	t := NewTimer(100 * time.Millisecond)

	// Then
	end := <-t.C()
	if end.Sub(start) < 100*time.Millisecond {
		c.Error("Sleep did not last long enough")
	}
}

func (s *SystemSuite) TestTimerStop(c *C) {
	t := NewTimer(50 * time.Millisecond)

	// When
	active := t.Stop()

	// Then
	c.Assert(active, Equals, true)
	time.Sleep(100)
	select {
	case <-t.C():
		c.Error("Timer should not have fired")
	default:
	}
}

func (s *SystemSuite) TestTimerReset(c *C) {
	start := time.Now()
	t := NewTimer(300 * time.Millisecond)

	// When
	t.Reset(100 * time.Millisecond)

	// Then
	end := <-t.C()
	if end.Sub(start) > 150*time.Millisecond {
		c.Error("Waited too long")
	}
}

func (s *SystemSuite) TestNewTicker(c *C) {
	start := Now()

	// When
	t := NewTicker(100 * time.Millisecond)

	// Then
	end := <-t.C()
	if end.Sub(start) < 100*time.Millisecond {
		c.Error("Sleep did not last long enough")
	}
	end = <-t.C()
	if end.Sub(start) < 200*time.Millisecond {
		c.Error("Sleep did not last long enough")
	}

	t.Stop()
	time.Sleep(150)
	select {
	case <-t.C():
		c.Error("Ticker should not have fired")
	default:
	}
}

func (s *SystemSuite) TestTick(c *C) {
	start := Now()

	// When
	ch := Tick(100 * time.Millisecond)

	// Then
	end := <-ch
	if end.Sub(start) < 100*time.Millisecond {
		c.Error("Sleep did not last long enough")
	}
	end = <-ch
	if end.Sub(start) < 200*time.Millisecond {
		c.Error("Sleep did not last long enough")
	}
}

func (s *SystemSuite) TestNewStoppedTimer(c *C) {
	t := NewStoppedTimer()

	// When/Then
	select {
	case <-t.C():
		c.Error("Timer should not have fired")
	default:
	}
	c.Assert(t.Stop(), Equals, false)
}
