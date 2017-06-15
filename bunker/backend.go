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
	"time"

	"github.com/pborman/uuid"
	"github.com/mailgun/sandra"
)

// backend is an interface for bunker backends. It mainly exists in order to simplify testing
// so fake backends can be implemented.
type backend interface {
	put(string, time.Duration) (string, error)
	get(string) (string, error)
	delete(string) error
}

// cassandraCluster represents a Cassandra-backed bunker backend.
type cassandraCluster struct {
	db sandra.Cassandra
}

// newCassandraCluster creates a new Cassandra-backed bunker backend.
func newCassandraCluster(c sandra.CassandraConfig) (backend, error) {
	db, err := sandra.NewCassandra(c)

	if err != nil {
		return nil, fmt.Errorf("failed to init Cassandra cluster: %v", err)
	}

	cluster := &cassandraCluster{db}

	// init the schema
	if err = cluster.createSchema(); err != nil {
		return nil, err
	}

	return cluster, nil
}

// createSchema creates necessary tables unless they already exist. It does not create a keyspace.
func (c *cassandraCluster) createSchema() error {
	return c.db.ExecuteQuery(
		"create table if not exists messages (key varchar primary key, message varchar) with compaction = {'class': 'LeveledCompactionStrategy'}")
}

// put saves the provided message into Cassandra with the specified TTL and returns a key.
func (c *cassandraCluster) put(message string, ttl time.Duration) (string, error) {
	key := uuid.NewUUID().String()

	query := "insert into messages (key, message) values (?, ?)"

	if ttl > 0 {
		query += fmt.Sprintf(" using ttl %v", int(ttl.Seconds()))
	}

	if err := c.db.ExecuteQuery(query, key, message); err != nil {
		return "", err
	}

	return key, nil
}

// get retrieves a message from Cassandra by the provided key.
func (c *cassandraCluster) get(key string) (string, error) {
	var chunk string

	if err := c.db.ScanQuery("select message from messages where key = ?", []interface{}{key}, &chunk); err != nil {
		return "", err
	}

	return chunk, nil
}

// delete deletes a message from Cassandra by the provided key.
func (c *cassandraCluster) delete(key string) error {
	return c.db.ExecuteQuery("delete from messages where key = ?", key)
}

// truncate removes all data from the messages table, for testing only.
func (c *cassandraCluster) truncate() error {
	return c.db.ExecuteQuery("truncate messages")
}
