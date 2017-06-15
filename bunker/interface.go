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

`bunker` is a Mailgun library that provides key/value storage API backed by Cassandra.
Its primary purpose is to store message bodies but it can be used as a generic key/value
storage system.
*/
package bunker

import (
	"fmt"
	"time"

	"github.com/mailgun/holster/secret"
	"github.com/mailgun/sandra"
)

// Bunker interace defines set of operations supported by bunker.
type Bunker interface {
	// Put saves a provided message into bunker and returns its bunker key.
	Put(message string) (string, error)

	// PutWithOptions does the same as Put but allows to specify save options, such as TTL or encryption.
	PutWithOptions(message string, opts PutOptions) (string, error)

	// Get retrieves a message from bunker by bunker key.
	Get(key string) (string, error)

	// Delete deletes a message from bunker by bunker key.
	Delete(key string) error

	SetSecretService(secret.SecretService)
}

// PutOptions packs parameters that can optionally be provided to tweak save behavior,
// such as TTL or encryption.
type PutOptions struct {
	// if non-nil, a saved message will expire after this duration
	TTL time.Duration

	// if true, a message will be compressed with zlib before saving
	Compress bool

	// if true, a message will be encrypted before saving
	Encrypt bool
}

// Cluster represents configuration of a single Cassandra cluster used as a bunker backend.
type ClusterConfig struct {
	// cluster name used for identification purposes
	Name string

	// cluster weight determines how much traffic this cluster is going to get relative to other clusters
	Weight int

	// database connection config
	Cassandra sandra.CassandraConfig
}

// DefaultBunker is the global bunker instance used by the package level Put, PutWithOptions, Get
// and Delete functions.
//
// It should be initialized by calling Init function prior to usage.
var DefaultBunker Bunker

// Init initializes the global DefaultBunker instance with a provided config.
func Init(clusterConfigs []ClusterConfig, hmacKey []byte) (err error) {
	if DefaultBunker, err = NewBunker(clusterConfigs, hmacKey); err != nil {
		return err
	}
	return nil
}

// Put saves a provided message into bunker and returns its bunker key.
//
// Put is a wrapper around DefaultBunker.PutWithOptions with empty options. DefaultBunker should be
// initialized by calling Init function prior to usage.
func Put(message string) (string, error) {
	return PutWithOptions(message, PutOptions{})
}

// PutWithOptions does the same as Put but allows to specify save options, such as TTL or encryption.
//
// PutWithOptions is a wrapper for DefaultBunker.PutWithOptions. DefaultBunker should be initialized
// by calling Init function prior to usage.
func PutWithOptions(message string, opts PutOptions) (string, error) {
	if DefaultBunker == nil {
		return "", fmt.Errorf("bunker is not initialized, call bunker.Init first")
	}
	return DefaultBunker.PutWithOptions(message, opts)
}

// Get retrieves a message from bunker by bunker key.
//
// Get is a wrapper for DefaultBunker.Get. DefaultBunker should be initialized by calling Init
// function prior to usage.
func Get(key string) (string, error) {
	if DefaultBunker == nil {
		return "", fmt.Errorf("bunker is not initialized, call bunker.Init first")
	}
	return DefaultBunker.Get(key)
}

// Delete deletes a message from bunker by bunker key.
//
// Delete is a wrapper around DefaultBunker.Delete. DefaultBunker should be initialized by
// calling Init function prior to usage.
func Delete(key string) error {
	if DefaultBunker == nil {
		return fmt.Errorf("bunker is not initialized, call bunker.Init first")
	}
	return DefaultBunker.Delete(key)
}

// SetSecretService configures DefaultBunker with the provided instance of SecretService.
func SetSecretService(s secret.SecretService) error {
	if DefaultBunker == nil {
		return fmt.Errorf("bunker is not initialized, call bunker.Init first")
	}
	DefaultBunker.SetSecretService(s)
	return nil
}
