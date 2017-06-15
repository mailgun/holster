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

import (
	"fmt"
	"testing"
	"time"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) {
	TestingT(t)
}

type CassandraClusterSuite struct {
	cluster *cassandraCluster
}

var _ = Suite(&CassandraClusterSuite{})

func (s *CassandraClusterSuite) SetUpSuite(c *C) {
	cluster, err := newCassandraCluster(testCassandraConf)
	if err != nil {
		c.Fatal(err)
	}
	s.cluster = cluster.(*cassandraCluster)
}

func (s *CassandraClusterSuite) SetUpTest(c *C) {
	s.cluster.truncate()
}

func (s *CassandraClusterSuite) TestCRUD(c *C) {
	key, err := s.cluster.put("hello, bunker!", 0)
	c.Assert(err, IsNil)
	c.Assert(key, NotNil)

	message, err := s.cluster.get(key)
	c.Assert(err, IsNil)
	c.Assert(message, Equals, "hello, bunker!")

	err = s.cluster.delete(key)
	c.Assert(err, IsNil)

	message, err = s.cluster.get(key)
	c.Assert(err, NotNil)
	c.Assert(message, Equals, "")
}

// fakeCluster returns errors on every operation. Used in tests.
type fakeCluster struct {
}

func (c *fakeCluster) put(message string, ttl time.Duration) (string, error) {
	return "", fmt.Errorf("fakeCluster.put() error")
}

func (c *fakeCluster) get(key string) (string, error) {
	return "", fmt.Errorf("fakeCluster.get() error")
}

func (c *fakeCluster) delete(key string) error {
	return fmt.Errorf("fakeCluster.delete() error")
}
