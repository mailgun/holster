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
package secret_test

import (
	"crypto/subtle"
	"fmt"
	"testing"

	"os"

	"bytes"
	"encoding/base64"

	"github.com/mailgun/holster/random"
	"github.com/mailgun/holster/secret"
)

var _ = fmt.Printf // for testing

func TestEncryptDecryptCycle(t *testing.T) {
	secret.RandomProvider = &random.FakeRNG{}

	key, err := secret.NewKey()
	if err != nil {
		t.Errorf("Got unexpected response from NewKey: %v", err)
	}

	s, err := secret.New(&secret.Config{KeyBytes: key})
	if err != nil {
		t.Errorf("Got unexpected response from NewWithKeyBytes: %v", err)
	}

	message := []byte("hello, box!")
	sealed, err := s.Seal(message)
	if err != nil {
		t.Errorf("Got unexpected response from Seal: %v", err)
	}

	out, err := s.Open(sealed)
	if err != nil {
		t.Errorf("Got unexpected response from Open: %v", err)
	}

	// compare the messages
	if subtle.ConstantTimeCompare(message, out) != 1 {
		t.Errorf("Contents do not match: %v, %v", message, out)
	}
}

func TestEncryptDecryptCyclePackage(t *testing.T) {
	secret.RandomProvider = &random.FakeRNG{}

	key, err := secret.NewKey()
	if err != nil {
		t.Errorf("Got unexpected response from NewKey: %v", err)
	}

	message := []byte("hello, box!")
	sealed, err := secret.Seal(message, key)
	if err != nil {
		t.Errorf("Got unexpected response from Seal: %v", err)
	}

	out, err := secret.Open(sealed, key)
	if err != nil {
		t.Errorf("Got unexpected response from Open: %v", err)
	}

	// compare the messages
	if subtle.ConstantTimeCompare(message, out) != 1 {
		t.Errorf("Contents do not match: %v, %v", message, out)
	}
}

func ExampleKeyGeneration() {
	// For consistency during tests, DO NOT USE IN PRODUCTION
	secret.RandomProvider = &random.FakeRNG{}

	// Create a new randomly generated key
	keyBytes, _ := secret.NewKey()
	fmt.Printf("New Key: %s\n", secret.KeyToEncodedString(keyBytes))

	// given key bytes, return an base64 encoded key
	encodedKey := secret.KeyToEncodedString(keyBytes)
	// given a base64 encoded key, return key bytes
	decodedKey, _ := secret.EncodedStringToKey(encodedKey)

	fmt.Printf("Key and Encoded/Decoded key are equal: %t", bytes.Equal((*keyBytes)[:], decodedKey[:]))

	// Output: New Key: AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=
	// Key and Encoded/Decoded key are equal: true
}

func ExampleEncryptUsage() {
	// For consistency during tests, DO NOT USE IN PRODUCTION
	secret.RandomProvider = &random.FakeRNG{}

	// Create a new randomly generated key
	key, err := secret.NewKey()

	// Store the key on disk for retrieval later
	fd, err := os.Create("/tmp/test-secret.key")
	if err != nil {
		panic(err)
	}
	fd.Write([]byte(secret.KeyToEncodedString(key)))
	fd.Close()

	// Read base64 encoded key in from disk
	s, err := secret.New(&secret.Config{KeyPath: "/tmp/test-secret.key"})
	if err != nil {
		panic(err)
	}

	// Encrypt the message using the key provided and a randomly generated nonce
	sealed, err := s.Seal([]byte("hello, world"))
	if err != nil {
		panic(err)
	}

	// Optionally base64 encode them and store them somewhere (like in a database)
	cipherText := base64.StdEncoding.EncodeToString(sealed.CiphertextBytes())
	nonce := base64.StdEncoding.EncodeToString(sealed.NonceBytes())
	fmt.Printf("Ciphertext: %s, Nonce: %s\n", cipherText, nonce)

	// Decrypt the message
	msg, err := s.Open(&secret.SealedBytes{
		Ciphertext: sealed.CiphertextBytes(),
		Nonce:      sealed.NonceBytes(),
	})
	fmt.Printf("Decrypted Plaintext: %s\n", string(msg))

	// Output: Ciphertext: Pg7RWodWBNwVViVfySz1RTaaVCOo5oJn1E7jWg==, Nonce: AAECAwQFBgcICQoLDA0ODxAREhMUFRYX
	// Decrypted Plaintext: hello, world
}

func ExampleEncryptFuncUsage() {
	// For consistency during tests, DO NOT USE IN PRODUCTION
	secret.RandomProvider = &random.FakeRNG{}

	// Create a new randomly generated key
	key, _ := secret.NewKey()

	// Encrypt the message using the key provided and a randomly generated nonce
	sealed, err := secret.Seal([]byte("hello, world"), key)
	if err != nil {
		panic(err)
	}

	// Optionally base64 encode them and store them somewhere (like in a database)
	cipherText := base64.StdEncoding.EncodeToString(sealed.CiphertextBytes())
	nonce := base64.StdEncoding.EncodeToString(sealed.NonceBytes())
	fmt.Printf("Ciphertext: %s, Nonce: %s\n", cipherText, nonce)

	// Decrypt the message
	msg, err := secret.Open(&secret.SealedBytes{
		Ciphertext: sealed.CiphertextBytes(),
		Nonce:      sealed.NonceBytes(),
	}, key)
	fmt.Printf("Decrypted Plaintext: %s\n", string(msg))

	// Output: Ciphertext: Pg7RWodWBNwVViVfySz1RTaaVCOo5oJn1E7jWg==, Nonce: AAECAwQFBgcICQoLDA0ODxAREhMUFRYX
	// Decrypted Plaintext: hello, world
}
