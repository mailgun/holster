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
package bunker

import . "gopkg.in/check.v1"

type WeighedIteratorSuite struct{}

var _ = Suite(&WeighedIteratorSuite{})

func (s *WeighedIteratorSuite) TestIterator(c *C) {
	iter := newWeighedIterator(
		map[string]int{"cluster-1": 7, "cluster-2": 2, "cluster-3": 1})

	shuffled := iter.Next()

	// make sure the returned slice has 3 elements
	c.Assert(len(shuffled), Equals, 3)

	// make sure every cluster name is present in the slice
	for _, name := range []string{"cluster-1", "cluster-2", "cluster-3"} {
		present := false
		for _, n := range shuffled {
			if name == n {
				present = true
			}
		}
		c.Assert(present, Equals, true)
	}
}

// fakeIterator always returns names in the fixed order. Used in tests.
type fakeIterator struct {
	names []string
}

func (i *fakeIterator) Next() []string {
	return i.names
}
