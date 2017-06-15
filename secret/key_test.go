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
package secret

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/mailgun/holster/random"
)

var _ = fmt.Printf // for testing

func TestNewKey(t *testing.T) {
	RandomProvider = &random.FakeRNG{}

	// get a new key
	gotKeyBytes, err := NewKey()
	if err != nil {
		t.Errorf("Got unexpected response from NewKey: %v", err)
	}

	// this is what we want
	wantKeyBytes := [32]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31}

	// check
	if *gotKeyBytes != wantKeyBytes {
		t.Errorf("Got: %v, Want: %v\n", gotKeyBytes, wantKeyBytes)
	}
}

func TestHexStringToKey(t *testing.T) {
	// build what we expect
	var wantKeyBytes [32]byte
	for i := range wantKeyBytes {
		wantKeyBytes[i] = byte(i)
	}

	// convert base64-encoded string to bytes
	gotKeyBytes, err := EncodedStringToKey("AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=")
	if err != nil {
		t.Errorf("Got unexpected response from HexStringToKey: %v", err)
	}

	// check
	if *gotKeyBytes != wantKeyBytes {
		t.Errorf("Got: %v, Want: %v", *gotKeyBytes, wantKeyBytes)
	}
}

func TestKeyToHexString(t *testing.T) {
	// convert bytes to base64-encoded string
	var keyBytes [32]byte
	for i := range keyBytes {
		keyBytes[i] = byte(i)
	}
	gotHexKey := KeyToEncodedString(&keyBytes)

	// check
	if g, w := gotHexKey, "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="; g != w {
		t.Errorf("Got: %v, Want: %v", g, w)
	}
}

func TestSealedDataToString(t *testing.T) {
	sb := &SealedBytes{
		Ciphertext: []byte{0, 1, 2},
		Nonce:      []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31},
	}

	gotSealedString, err := SealedDataToString(sb)
	if err != nil {
		t.Errorf("Unexpected response from SealedDataToString: %v", err)
	}

	if got, want := gotSealedString, "eyJDaXBoZXJ0ZXh0IjoiQUFFQyIsIk5vbmNlIjoiQUFFQ0F3UUZCZ2NJQ1FvTERBME9EeEFSRWhNVUZSWVhHQmthR3h3ZEhoOD0ifQ=="; got != want {
		t.Errorf("Got sealed string: %v, Want: %v", got, want)
	}
}

func TestStringToSealedData(t *testing.T) {
	ss := "eyJDaXBoZXJ0ZXh0IjoiQUFFQyIsIk5vbmNlIjoiQUFFQ0F3UUZCZ2NJQ1FvTERBME9EeEFSRWhNVUZSWVhHQmthR3h3ZEhoOD0ifQ=="

	gotSealedData, err := StringToSealedData(ss)
	if err != nil {
		t.Errorf("Unexpected response from StringToSealedData: %v", err)
	}

	if got, want := gotSealedData.CiphertextBytes(), []byte{0, 1, 2}; !bytes.Equal(got, want) {
		t.Errorf("Got sealed bytes: %v, Want: %v", got, want)
	}
	if got, want := gotSealedData.NonceBytes(), []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31}; !bytes.Equal(got, want) {
		t.Errorf("Got sealed bytes: %v, Want: %v", got, want)
	}
}
