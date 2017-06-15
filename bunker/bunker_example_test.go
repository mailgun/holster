package bunker_test

import (
	"fmt"

	"github.com/mailgun/holster"
	"github.com/mailgun/holster/bunker"
	"github.com/mailgun/sandra"
)

func Example_Bunker_Usage() {
	sandraConf := sandra.CassandraConfig{
		Nodes:            []string{holster.GetEnv("CASSANDRA_ENDPOINT", "127.0.0.1:9042")},
		Keyspace:         "bunker_test",
		DataCenter:       "datacenter1",
		ReadConsistency:  "one",
		WriteConsistency: "one",
	}

	conf := []bunker.ClusterConfig{
		{Name: "cluster-1", Weight: 1, Cassandra: sandraConf},
		{Name: "cluster-2", Weight: 1, Cassandra: sandraConf},
	}

	// Initialize the bunker singleton with an HMAC Key of "s3cr3t"
	if err := bunker.Init(conf, []byte("s3cr3t")); err != nil {
		fmt.Printf("Error During Init(): '%s'\n", err.Error())
		return
	}

	// Store hello world
	key, _ := bunker.Put("hello, world")

	// Retrieve "hello world"
	message, _ := bunker.Get(key)

	fmt.Printf("Get(): '%s'\n", message)

	// Delete the key
	bunker.Delete(key)

	// Should return empty message
	message, _ = bunker.Get(key)
	fmt.Printf("Deleted(): '%s'\n", message)

	// Output: Get(): 'hello, world'
	// Deleted(): ''
}
