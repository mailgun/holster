package clock

import (
	"fmt"
	"time"

	. "gopkg.in/check.v1"
)

type FrozenSuite struct {
	epoch time.Time
}

var _ = Suite(&FrozenSuite{})

func (s *FrozenSuite) SetUpSuite(c *C) {
	var err error
	s.epoch, err = time.Parse(time.RFC3339, "2009-02-19T00:00:00Z")
	c.Assert(err, IsNil)
}

func (s *FrozenSuite) SetUpTest(c *C) {
	Freeze(s.epoch)
}

func (s *FrozenSuite) TearDownTest(c *C) {
	Unfreeze()
}

func (s *FrozenSuite) TestNow(c *C) {
	c.Assert(Now(), Equals, s.epoch)
	Advance(42 * time.Millisecond)
	c.Assert(Now(), Equals, s.epoch.Add(42*time.Millisecond))
}

func (s *FrozenSuite) TestSleep(c *C) {
	hits := make(chan int, 100)

	delays := []int{60, 100, 90, 131, 999, 5}
	for i, tc := range []struct {
		desc string
		fn   func(delayMs int)
	}{{
		desc: "Sleep",
		fn: func(delay int) {
			Sleep(time.Duration(delay) * time.Millisecond)
			hits <- delay
		},
	}, {
		desc: "After",
		fn: func(delay int) {
			<-After(time.Duration(delay) * time.Millisecond)
			hits <- delay
		},
	}, {
		desc: "AfterFunc",
		fn: func(delay int) {
			AfterFunc(time.Duration(delay)*time.Millisecond,
				func() {
					hits <- delay
				})
		},
	}, {
		desc: "NewTimer",
		fn: func(delay int) {
			t := NewTimer(time.Duration(delay) * time.Millisecond)
			<-t.C()
			hits <- delay
		},
	}} {
		fmt.Printf("Test case #%d: %s", i, tc.desc)
		for _, delay := range delays {
			go tc.fn(delay)
		}
		// Spin-wait for all goroutines to fall asleep.
		ft := provider.(*frozenTime)
		for {
			if len(ft.timers) == len(delays) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		runningMs := 0
		for i, delayMs := range []int{5, 60, 90, 100, 131, 999} {
			fmt.Printf("Checking timer #%d, delay=%d\n", i, delayMs)
			delta := delayMs - runningMs - 1
			Advance(time.Duration(delta) * time.Millisecond)
			// Check before each timer deadline that it is not triggered yet.
			assertHits(c, hits, []int{})

			// When
			Advance(1 * time.Millisecond)

			// Then
			assertHits(c, hits, []int{delayMs})

			runningMs += delta + 1
		}

		Advance(1000 * time.Millisecond)
		assertHits(c, hits, []int{})
	}
}

// Timers scheduled to trigger at the same time do that in the order they were
// created.
func (s *FrozenSuite) TestSameTime(c *C) {
	var hits []int

	AfterFunc(100, func() { hits = append(hits, 3) })
	AfterFunc(100, func() { hits = append(hits, 1) })
	AfterFunc(99, func() { hits = append(hits, 2) })
	AfterFunc(100, func() { hits = append(hits, 5) })
	AfterFunc(101, func() { hits = append(hits, 4) })
	AfterFunc(101, func() { hits = append(hits, 6) })

	// When
	Advance(100)

	// Then
	c.Assert(hits, DeepEquals, []int{2, 3, 1, 5})
}

func (s *FrozenSuite) TestTimerStop(c *C) {
	hits := []int{}

	AfterFunc(100, func() { hits = append(hits, 1) })
	t := AfterFunc(100, func() { hits = append(hits, 2) })
	AfterFunc(100, func() { hits = append(hits, 3) })
	Advance(99)
	c.Assert(hits, DeepEquals, []int{})

	// When
	active1 := t.Stop()
	active2 := t.Stop()

	// Then
	c.Assert(active1, Equals, true)
	c.Assert(active2, Equals, false)
	Advance(1)
	c.Assert(hits, DeepEquals, []int{1, 3})
}

func (s *FrozenSuite) TestReset(c *C) {
	hits := []int{}

	t1 := AfterFunc(100, func() { hits = append(hits, 1) })
	t2 := AfterFunc(100, func() { hits = append(hits, 2) })
	AfterFunc(100, func() { hits = append(hits, 3) })
	Advance(99)
	c.Assert(hits, DeepEquals, []int{})

	// When
	active1 := t1.Reset(1) // Reset to the same time
	active2 := t2.Reset(7)

	// Then
	c.Assert(active1, Equals, true)
	c.Assert(active2, Equals, true)

	Advance(1)
	c.Assert(hits, DeepEquals, []int{3, 1})
	Advance(5)
	c.Assert(hits, DeepEquals, []int{3, 1})
	Advance(1)
	c.Assert(hits, DeepEquals, []int{3, 1, 2})
}

// Reset to the same time just puts the timer at the end of the trigger list
// for the date.
func (s *FrozenSuite) TestResetSame(c *C) {
	hits := []int{}

	t := AfterFunc(100, func() { hits = append(hits, 1) })
	AfterFunc(100, func() { hits = append(hits, 2) })
	AfterFunc(100, func() { hits = append(hits, 3) })
	AfterFunc(101, func() { hits = append(hits, 4) })
	Advance(9)

	// When
	active := t.Reset(91)

	// Then
	c.Assert(active, Equals, true)

	Advance(90)
	c.Assert(hits, DeepEquals, []int{})
	Advance(1)
	c.Assert(hits, DeepEquals, []int{2, 3, 1})
}

func (s *FrozenSuite) TestTicker(c *C) {
	t := NewTicker(100)

	Advance(99)
	assertNotFired(c, t.C())
	Advance(1)
	c.Assert(s.epoch.Add(100), Equals, <-t.C())
	Advance(750)
	c.Assert(s.epoch.Add(200), Equals, <-t.C())
	Advance(49)
	assertNotFired(c, t.C())
	Advance(1)
	c.Assert(s.epoch.Add(900), Equals, <-t.C())

	t.Stop()
	Advance(300)
	assertNotFired(c, t.C())
}

func (s *FrozenSuite) TestTickerZero(c *C) {
	defer func() {
		recover()
	}()

	NewTicker(0)
	c.Error("Should panic")
}

func (s *FrozenSuite) TestTick(c *C) {
	ch := Tick(100)

	Advance(99)
	assertNotFired(c, ch)
	Advance(1)
	c.Assert(s.epoch.Add(100), Equals, <-ch)
	Advance(750)
	c.Assert(s.epoch.Add(200), Equals, <-ch)
	Advance(49)
	assertNotFired(c, ch)
	Advance(1)
	c.Assert(s.epoch.Add(900), Equals, <-ch)
}

func (s *FrozenSuite) TestTickZero(c *C) {
	ch := Tick(0)
	c.Assert(ch, IsNil)
}

func (s *FrozenSuite) TestNewStoppedTimer(c *C) {
	t := NewStoppedTimer()

	// When/Then
	select {
	case <-t.C():
		c.Error("Timer should not have fired")
	default:
	}
	c.Assert(t.Stop(), Equals, false)
}

func assertHits(c *C, got <-chan int, want []int) {
	for i, w := range want {
		var g int
		select {
		case g = <-got:
		case <-time.After(100 * time.Millisecond):
			c.Errorf("Missing hit: want=%v", w)
			return
		}
		c.Assert(g, Equals, w, Commentf("Hit #%d", i))
	}
	for {
		select {
		case g := <-got:
			c.Errorf("Unexpected hit: %v", g)
		default:
			return
		}
	}
}

func assertNotFired(c *C, ch <-chan time.Time) {
	select {
	case <-ch:
		c.Error("Premature fire")
	default:
	}
}
