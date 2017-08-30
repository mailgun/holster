/*
Copyright 2017 Mailgun Technologies Inc

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package holster

import (
	"time"

	"sync/atomic"

	"github.com/ahmetalpbalkan/go-linq"
	"github.com/pkg/errors"
	. "gopkg.in/check.v1"
)

type WaitGroupTestSuite struct{}

var _ = Suite(&WaitGroupTestSuite{})

func (s *WaitGroupTestSuite) SetUpSuite(c *C) {
}

func (s *WaitGroupTestSuite) TestRun(c *C) {
	var wg WaitGroup

	items := []error{
		errors.New("Error 1"),
		errors.New("Error 2"),
	}

	// Iterate over a thing and doing some long running thing for each
	for _, item := range items {
		wg.Run(func(item interface{}) error {
			// Do some long running thing
			time.Sleep(time.Nanosecond * 50)
			// Return an error for testing
			return item.(error)
		}, item)
	}

	errs := wg.Wait()
	c.Assert(errs, NotNil)
	c.Assert(len(errs), Equals, 2)
	c.Assert(linq.From(errs).Contains(items[0]), Equals, true)
	c.Assert(linq.From(errs).Contains(items[1]), Equals, true)
}

func (s *WaitGroupTestSuite) TestLoop(c *C) {
	pipe := make(chan int32, 0)
	var wg WaitGroup
	var count int32

	wg.Loop(func() bool {
		select {
		case inc, ok := <-pipe:
			if !ok {
				return false
			}
			atomic.AddInt32(&count, inc)
		}
		return true
	})

	// Feed the loop some numbers and close the pipe
	pipe <- 1
	pipe <- 5
	pipe <- 10
	close(pipe)

	// Wait for the routine to end
	// no error collection when using Loop()
	errs := wg.Wait()
	c.Assert(errs, IsNil)
	c.Assert(count, Equals, int32(16))
}

func (s *WaitGroupTestSuite) TestUntil(c *C) {
	pipe := make(chan int32, 0)
	var wg WaitGroup
	var count int32

	wg.Until(func(done chan struct{}) bool {
		select {
		case inc := <-pipe:
			atomic.AddInt32(&count, inc)
		case <-done:
			return false
		}
		return true
	})

	// Feed the loop some numbers and close the pipe
	pipe <- 1
	pipe <- 5
	pipe <- 10

	// Wait for the routine to end
	wg.Stop()
	c.Assert(count, Equals, int32(16))
}
