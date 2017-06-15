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
	"github.com/mailgun/holster"
	"github.com/mailgun/holster/secret"
	"github.com/mailgun/sandra"
	. "gopkg.in/check.v1"
)

var testCassandraConf = sandra.CassandraConfig{
	Nodes:            []string{holster.GetEnv("CASSANDRA_ENDPOINT", "127.0.0.1:9042")},
	Keyspace:         "bunker_test",
	DataCenter:       "datacenter1",
	ReadConsistency:  "one",
	WriteConsistency: "one",
	TestMode:         true,
}

var testConf = []ClusterConfig{
	{
		Name: "cluster-1", Weight: 1, Cassandra: testCassandraConf,
	},
	{
		Name: "cluster-2", Weight: 1, Cassandra: testCassandraConf,
	},
}

var testHMACKey = []byte("s3cr3t")

type BunkerSuite struct {
	b *bunker
}

var _ = Suite(&BunkerSuite{})

func (s *BunkerSuite) SetUpSuite(c *C) {
	b, err := NewBunker(testConf, testHMACKey)
	if err != nil {
		c.Fatal(err)
	}
	s.b = b.(*bunker)
}

func (s *BunkerSuite) TestPackUnpackKeys(c *C) {
	cluster := s.b.clusters["cluster-1"]
	keys := []string{"key-1", "key-2", "key-3"}

	packedKeys, err := packKeys(cluster, keys, 0)
	c.Assert(err, IsNil)

	// unpack the keys and verify they are the same
	unpackedKeys, err := unpackKeys(cluster, packedKeys)
	c.Assert(err, IsNil)
	c.Assert(unpackedKeys, DeepEquals, keys)
}

func (s *BunkerSuite) TestBasicCRUD(c *C) {
	key, err := s.b.Put("hello, world")
	c.Assert(err, IsNil)

	message, err := s.b.Get(key)
	c.Assert(err, IsNil)
	c.Assert(message, Equals, "hello, world")

	err = s.b.Delete(key)
	c.Assert(err, IsNil)

	message, err = s.b.Get(key)
	c.Assert(err, NotNil)
}

func (s *BunkerSuite) TestMessageChunking(c *C) {
	// mock max chunk size so we can test chunking
	save := GetChunkSizeBytes
	defer func() { GetChunkSizeBytes = save }()
	GetChunkSizeBytes = func() int { return 1 }

	// this message has 3 chars so it'll be chunked into 3 pieces
	message := "hey"

	encKey, err := s.b.Put(message)
	c.Assert(err, IsNil)

	// decode the key and verify it is packed
	key, err := decodeKey(encKey, testHMACKey)
	c.Assert(err, IsNil)
	c.Assert(key.Packed, Equals, true)
}

func (s *BunkerSuite) TestGetAndDeleteChunkedMessage(c *C) {
	// set chunk size to 1 byte...
	save := GetChunkSizeBytes
	defer func() { GetChunkSizeBytes = save }()
	GetChunkSizeBytes = func() int { return 1 }

	// ...and re-run the BasicCRUD test
	s.TestBasicCRUD(c)
}

func (s *BunkerSuite) TestClustersFailover(c *C) {
	// replace clusters iterator with a fake one to make sure cluster-1 is always returned first
	savedIterator := s.b.clustersIterator
	s.b.clustersIterator = &fakeIterator{[]string{"cluster-1", "cluster-2"}}
	defer func() { s.b.clustersIterator = savedIterator }() // restore

	// replace the backend for cluster-1 with a fake one to make sure it always errors
	savedCluster := s.b.clusters["cluster-1"]
	s.b.clusters["cluster-1"] = &fakeCluster{}
	defer func() { s.b.clusters["cluster-1"] = savedCluster }() // restore

	// now verify a message can still be saved successfully
	_, err := s.b.Put("hello")
	c.Assert(err, IsNil)
}

func (s *BunkerSuite) TestAllClustersFailed(c *C) {
	// replace both backends with fake ones to make sure they always error
	savedClusters := s.b.clusters
	s.b.clusters = map[string]backend{"cluster-1": &fakeCluster{}, "cluster-2": &fakeCluster{}}
	defer func() { s.b.clusters = savedClusters }() // restore

	// verify an error is returned
	_, err := s.b.Put("hello")
	c.Assert(err, NotNil)
}

func (s *BunkerSuite) TestCompressDecompress(c *C) {
	compressed, err := compress("hello, world")
	c.Assert(err, IsNil)

	decompressed, err := decompress(compressed)
	c.Assert(err, IsNil)
	c.Assert(decompressed, Equals, "hello, world")
}

func (s *BunkerSuite) TestEncryptDecrypt(c *C) {
	key, err := secret.NewKey()
	c.Assert(err, IsNil)

	secretService, err := secret.New(&secret.Config{KeyBytes: key})
	c.Assert(err, IsNil)

	encrypted, err := encrypt("hello, world", secretService)
	c.Assert(err, IsNil)

	decrypted, err := decrypt(encrypted, secretService)
	c.Assert(err, IsNil)
	c.Assert(decrypted, Equals, "hello, world")
}
