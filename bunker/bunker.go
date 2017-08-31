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
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/mailgun/holster/secret"
	"github.com/sirupsen/logrus"
)

type bunker struct {
	clusters         map[string]backend
	clustersIterator weighedIterator
	secretService    secret.SecretService
	hmacKey          []byte
}

// NewBunker creates a new bunker instance and initializes it with a provided config.
func NewBunker(clusterConfigs []ClusterConfig, hmacKey []byte) (Bunker, error) {
	// maps cluster names to actual clusters
	clusters := make(map[string]backend)

	// maps cluster names to their weights, needed for weighed iterator
	namesToWeights := make(map[string]int)

	for _, conf := range clusterConfigs {
		cluster, err := newCassandraCluster(conf.Cassandra)
		if err != nil {
			return nil, err
		}
		clusters[conf.Name] = cluster
		namesToWeights[conf.Name] = conf.Weight
	}

	return &bunker{
		clusters:         clusters,
		clustersIterator: newWeighedIterator(namesToWeights),
		hmacKey:          hmacKey,
	}, nil
}

// Put saves a provided message into bunker and returns its bunker key.
func (b *bunker) Put(message string) (string, error) {
	return b.PutWithOptions(message, PutOptions{})
}

// PutWithOptions does the same as Put but allows to specify save options, such as TTL or encryption.
func (b *bunker) PutWithOptions(message string, opts PutOptions) (string, error) {
	for _, clusterName := range b.clustersIterator.Next() {
		key, err := b.tryPut(clusterName, message, opts)
		if err != nil {
			logrus.Errorf("Cluster %v failed: %v", clusterName, err)
			continue
		}
		return key, nil
	}

	// all clusters failed, ta-dah!
	return "", fmt.Errorf("all clusters failed")
}

func (b *bunker) tryPut(clusterName, message string, opts PutOptions) (string, error) {
	var keys []string
	var finalKey string
	var err error

	cluster := b.clusters[clusterName]

	// optional compression
	compressed := false
	if opts.Compress {
		if message, err = compress(message); err != nil {
			return "", err
		}
		compressed = true
	}

	// optional encryption
	encrypted := false
	if opts.Encrypt {
		if message, err = encrypt(message, b.secretService); err != nil {
			return "", err
		}
		encrypted = true
	}

	// large messages are split in chunks
	for _, chunk := range Split(message) {
		k, err := cluster.put(chunk, opts.TTL)
		if err != nil {
			return "", err
		}
		keys = append(keys, k)
	}

	// pack the keys if needed
	packed := false
	if len(keys) > 1 {
		if finalKey, err = packKeys(cluster, keys, opts.TTL); err != nil {
			return "", err
		}
		packed = true
	} else {
		finalKey = keys[0]
	}

	return encodeKey(&bkey{finalKey, clusterName, packed, compressed, encrypted, ""}, b.hmacKey)
}

// Get retrieves a message from bunker by bunker key.
func (b *bunker) Get(encKey string) (string, error) {
	// decode the key
	key, err := decodeKey(encKey, b.hmacKey)
	if err != nil {
		return "", fmt.Errorf("failed to decode key: %v %v", encKey, err)
	}

	// determine a cluster that owns this key
	cluster := b.clusters[key.Cluster]

	// unpack the keys
	keys := []string{key.Key}
	if key.Packed {
		if keys, err = unpackKeys(cluster, keys[0]); err != nil {
			return "", err
		}
	}

	var chunks []string

	// pull out message chunk by chunk
	for _, k := range keys {
		chunk, err := cluster.get(k)
		if err != nil {
			return "", err
		}
		chunks = append(chunks, chunk)
	}

	// merge all chunks
	message := strings.Join(chunks, "")

	if key.Encrypted {
		if message, err = decrypt(message, b.secretService); err != nil {
			return "", err
		}
	}

	if key.Compressed {
		if message, err = decompress(message); err != nil {
			return "", err
		}
	}

	return message, nil
}

// Delete deletes a message from bunker by bunker key.
func (b *bunker) Delete(encKey string) error {
	// decode the key
	key, err := decodeKey(encKey, b.hmacKey)
	if err != nil {
		return fmt.Errorf("failed to decode key: %v %v", encKey, err)
	}

	// determine a cluster owning this key
	cluster := b.clusters[key.Cluster]

	// unpack the keys
	keys := []string{key.Key}
	if key.Packed {
		if keys, err = unpackKeys(cluster, keys[0]); err != nil {
			return err
		}
		// append the packed key itself so we will delete it too
		keys = append(keys, key.Key)
	}

	// delete message chunk by chunk
	for _, k := range keys {
		if err := cluster.delete(k); err != nil {
			return err
		}
	}

	return nil
}

// SetSecretService configures bunker with the provided SecretService.
func (b *bunker) SetSecretService(s secret.SecretService) {
	b.secretService = s
}

// packKeys takes keys of all saved chunks and saves them into a database under a new key.
//
// Large message may be split into many chunks and thus have many keys resulting in a huge
// encoded bunker key. Packing sets an upper limit on a number of keys a message can be
// identified with.
func packKeys(cluster backend, keys []string, ttl time.Duration) (string, error) {
	newKey, err := cluster.put(strings.Join(keys, ","), ttl)
	if err != nil {
		return "", err
	}
	return newKey, nil
}

// unpackKeys retrieves keys of all chunks from a database by the provided keys.
func unpackKeys(cluster backend, key string) ([]string, error) {
	packedKeys, err := cluster.get(key)
	if err != nil {
		return nil, err
	}
	return strings.Split(packedKeys, ","), nil
}

func compress(message string) (string, error) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write([]byte(message)); err != nil {
		return "", err
	}
	w.Close()
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func decompress(compressedMessage string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(compressedMessage)
	if err != nil {
		return "", err
	}
	buf := bytes.NewBuffer(b)
	r, err := zlib.NewReader(buf)
	defer r.Close()
	if err != nil {
		return "", err
	}
	read, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(read), nil
}

func encrypt(message string, secretService secret.SecretService) (string, error) {
	if secretService == nil {
		return "", fmt.Errorf("secretService is nil")
	}
	sealedData, err := secretService.Seal([]byte(message))
	if err != nil {
		return "", err
	}
	encMessage, err := secret.SealedDataToString(sealedData)
	if err != nil {
		return "", err
	}
	return encMessage, nil
}

func decrypt(encMessage string, secretService secret.SecretService) (string, error) {
	if secretService == nil {
		return "", fmt.Errorf("secretService is nil")
	}
	sealedData, err := secret.StringToSealedData(encMessage)
	if err != nil {
		return "", err
	}
	message, err := secretService.Open(sealedData)
	if err != nil {
		return "", err
	}
	return string(message), nil
}
