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
package collections

import (
	"testing"

	"github.com/mailgun/holster/v4/clock"
	"github.com/stretchr/testify/suite"
)

type TTLMapSuite struct {
	suite.Suite
}

func TestTTLMapSuite(t *testing.T) {
	suite.Run(t, new(TTLMapSuite))
}

func (s *TTLMapSuite) SetupTest() {
	clock.Freeze(clock.Date(2012, 3, 4, 5, 6, 7, 0, clock.UTC))
}

func (s *TTLMapSuite) TearDownSuite() {
	clock.Unfreeze()
}

func (s *TTLMapSuite) TestSetWrong() {
	m := NewTTLMap(1)

	err := m.Set("a", 1, -1)
	s.Require().EqualError(err, "ttlSeconds should be >= 0, got -1")

	err = m.Set("a", 1, 0)
	s.Require().EqualError(err, "ttlSeconds should be >= 0, got 0")

	_, err = m.Increment("a", 1, 0)
	s.Require().EqualError(err, "ttlSeconds should be >= 0, got 0")

	_, err = m.Increment("a", 1, -1)
	s.Require().EqualError(err, "ttlSeconds should be >= 0, got -1")
}

func (s *TTLMapSuite) TestRemoveExpiredEmpty() {
	m := NewTTLMap(1)
	m.RemoveExpired(100)
}

func (s *TTLMapSuite) TestRemoveLastUsedEmpty() {
	m := NewTTLMap(1)
	m.RemoveLastUsed(100)
}

func (s *TTLMapSuite) TestGetSetExpire() {
	m := NewTTLMap(1)

	err := m.Set("a", 1, 1)
	s.Require().Equal(nil, err)

	valI, exists := m.Get("a")
	s.Require().Equal(true, exists)
	s.Require().Equal(1, valI)

	clock.Advance(1 * clock.Second)

	_, exists = m.Get("a")
	s.Require().Equal(false, exists)
}

func (s *TTLMapSuite) TestSetOverwrite() {
	m := NewTTLMap(1)

	err := m.Set("o", 1, 1)
	s.Require().Equal(nil, err)

	valI, exists := m.Get("o")
	s.Require().Equal(true, exists)
	s.Require().Equal(1, valI)

	err = m.Set("o", 2, 1)
	s.Require().Equal(nil, err)

	valI, exists = m.Get("o")
	s.Require().Equal(true, exists)
	s.Require().Equal(2, valI)
}

func (s *TTLMapSuite) TestRemoveExpiredEdgeCase() {
	m := NewTTLMap(1)

	err := m.Set("a", 1, 1)
	s.Require().Equal(nil, err)

	clock.Advance(1 * clock.Second)

	err = m.Set("b", 2, 1)
	s.Require().Equal(nil, err)

	_, exists := m.Get("a")
	s.Require().Equal(false, exists)

	valI, exists := m.Get("b")
	s.Require().Equal(true, exists)
	s.Require().Equal(2, valI)

	s.Require().Equal(1, m.Len())
}

func (s *TTLMapSuite) TestRemoveOutOfCapacity() {
	m := NewTTLMap(2)

	err := m.Set("a", 1, 5)
	s.Require().Equal(nil, err)

	clock.Advance(1 * clock.Second)

	err = m.Set("b", 2, 6)
	s.Require().Equal(nil, err)

	err = m.Set("c", 3, 10)
	s.Require().Equal(nil, err)

	_, exists := m.Get("a")
	s.Require().Equal(false, exists)

	valI, exists := m.Get("b")
	s.Require().Equal(true, exists)
	s.Require().Equal(2, valI)

	valI, exists = m.Get("c")
	s.Require().Equal(true, exists)
	s.Require().Equal(3, valI)

	s.Require().Equal(2, m.Len())
}

func (s *TTLMapSuite) TestGetNotExists() {
	m := NewTTLMap(1)
	_, exists := m.Get("a")
	s.Require().Equal(false, exists)
}

func (s *TTLMapSuite) TestGetIntNotExists() {
	m := NewTTLMap(1)
	_, exists, err := m.GetInt("a")
	s.Require().Equal(nil, err)
	s.Require().Equal(false, exists)
}

func (s *TTLMapSuite) TestGetInvalidType() {
	m := NewTTLMap(1)
	err := m.Set("a", "banana", 5)
	s.Require().NoError(err)

	_, _, err = m.GetInt("a")
	s.Require().EqualError(err, "Expected existing value to be integer, got string")

	_, err = m.Increment("a", 4, 1)
	s.Require().EqualError(err, "Expected existing value to be integer, got string")
}

func (s *TTLMapSuite) TestIncrementGetExpire() {
	m := NewTTLMap(1)

	_, err := m.Increment("a", 5, 1)
	s.Require().NoError(err)
	val, exists, err := m.GetInt("a")

	s.Require().Equal(nil, err)
	s.Require().Equal(true, exists)
	s.Require().Equal(5, val)

	clock.Advance(1 * clock.Second)

	_, err = m.Increment("a", 4, 1)
	s.Require().NoError(err)
	val, exists, err = m.GetInt("a")

	s.Require().Equal(nil, err)
	s.Require().Equal(true, exists)
	s.Require().Equal(4, val)
}

func (s *TTLMapSuite) TestIncrementOverwrite() {
	m := NewTTLMap(1)

	_, err := m.Increment("a", 5, 1)
	s.Require().NoError(err)
	val, exists, err := m.GetInt("a")

	s.Require().Equal(nil, err)
	s.Require().Equal(true, exists)
	s.Require().Equal(5, val)

	_, err = m.Increment("a", 4, 1)
	s.Require().NoError(err)
	val, exists, err = m.GetInt("a")

	s.Require().Equal(nil, err)
	s.Require().Equal(true, exists)
	s.Require().Equal(9, val)
}

func (s *TTLMapSuite) TestIncrementOutOfCapacity() {
	m := NewTTLMap(1)

	_, err := m.Increment("a", 5, 1)
	s.Require().NoError(err)
	val, exists, err := m.GetInt("a")

	s.Require().Equal(nil, err)
	s.Require().Equal(true, exists)
	s.Require().Equal(5, val)

	_, err = m.Increment("b", 4, 1)
	s.Require().NoError(err)
	val, exists, err = m.GetInt("b")

	s.Require().Equal(nil, err)
	s.Require().Equal(true, exists)
	s.Require().Equal(4, val)

	_, exists, err = m.GetInt("a")

	s.Require().Equal(nil, err)
	s.Require().Equal(false, exists)
}

func (s *TTLMapSuite) TestIncrementRemovesExpired() {
	m := NewTTLMap(2)

	_, err := m.Increment("a", 1, 1)
	s.Require().NoError(err)
	_, err = m.Increment("b", 2, 2)
	s.Require().NoError(err)

	clock.Advance(1 * clock.Second)
	_, err = m.Increment("c", 3, 3)
	s.Require().NoError(err)

	_, exists, err := m.GetInt("a")

	s.Require().Equal(nil, err)
	s.Require().Equal(false, exists)

	val, exists, err := m.GetInt("b")
	s.Require().Equal(nil, err)
	s.Require().Equal(true, exists)
	s.Require().Equal(2, val)

	val, exists, err = m.GetInt("c")
	s.Require().Equal(nil, err)
	s.Require().Equal(true, exists)
	s.Require().Equal(3, val)
}

func (s *TTLMapSuite) TestIncrementRemovesLastUsed() {
	m := NewTTLMap(2)

	_, err := m.Increment("a", 1, 10)
	s.Require().NoError(err)
	_, err = m.Increment("b", 2, 11)
	s.Require().NoError(err)
	_, err = m.Increment("c", 3, 12)
	s.Require().NoError(err)

	_, exists, err := m.GetInt("a")

	s.Require().Equal(nil, err)
	s.Require().Equal(false, exists)

	val, exists, err := m.GetInt("b")
	s.Require().Equal(nil, err)
	s.Require().Equal(true, exists)

	s.Require().Equal(2, val)

	val, exists, err = m.GetInt("c")
	s.Require().Equal(nil, err)
	s.Require().Equal(true, exists)
	s.Require().Equal(3, val)
}

func (s *TTLMapSuite) TestIncrementUpdatesTtl() {
	m := NewTTLMap(1)

	_, err := m.Increment("a", 1, 1)
	s.Require().NoError(err)
	_, err = m.Increment("a", 1, 10)
	s.Require().NoError(err)

	clock.Advance(1 * clock.Second)

	val, exists, err := m.GetInt("a")
	s.Require().Equal(nil, err)
	s.Require().Equal(true, exists)
	s.Require().Equal(2, val)
}

func (s *TTLMapSuite) TestUpdate() {
	m := NewTTLMap(1)

	_, err := m.Increment("a", 1, 1)
	s.Require().NoError(err)
	_, err = m.Increment("a", 1, 10)
	s.Require().NoError(err)

	clock.Advance(1 * clock.Second)

	val, exists, err := m.GetInt("a")
	s.Require().Equal(nil, err)
	s.Require().Equal(true, exists)
	s.Require().Equal(2, val)
}

func (s *TTLMapSuite) TestCallOnExpire() {
	var called bool
	var key string
	var val interface{}
	m := NewTTLMap(1)
	m.OnExpire = func(k string, el interface{}) {
		called = true
		key = k
		val = el
	}

	err := m.Set("a", 1, 1)
	s.Require().Equal(nil, err)

	valI, exists := m.Get("a")
	s.Require().Equal(true, exists)
	s.Require().Equal(1, valI)

	clock.Advance(1 * clock.Second)

	_, exists = m.Get("a")
	s.Require().Equal(false, exists)
	s.Require().Equal(true, called)
	s.Require().Equal("a", key)
	s.Require().Equal(1, val)
}
