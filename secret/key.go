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
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// NewKey returns a new key that can be used to encrypt and decrypt messages.
func NewKey() (*[SecretKeyLength]byte, error) {
	// get 32-bytes of random from /dev/urandom
	bytes, err := RandomProvider.Bytes(SecretKeyLength)
	if err != nil {
		return nil, fmt.Errorf("unable to generate random: %v", err)
	}

	return KeySliceToArray(bytes)
}

// EncodedStringToKey converts a base64-encoded string into key bytes.
func EncodedStringToKey(encodedKey string) (*[SecretKeyLength]byte, error) {
	// decode base64-encoded key
	keySlice, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, err
	}

	// convert to array and return
	return KeySliceToArray(keySlice)
}

// KeyToEncodedString converts bytes into a base64-encoded string
func KeyToEncodedString(keybytes *[SecretKeyLength]byte) string {
	return base64.StdEncoding.EncodeToString(keybytes[:])
}

// Given SealedData returns equivalent URL safe base64 encoded string.
func SealedDataToString(sealedData SealedData) (string, error) {
	b, err := json.Marshal(sealedData)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(b), nil
}

// Given a URL safe base64 encoded string, returns SealedData.
func StringToSealedData(encodedBytes string) (SealedData, error) {
	bytes, err := base64.URLEncoding.DecodeString(encodedBytes)
	if err != nil {
		return nil, err
	}

	var sb SealedBytes
	err = json.Unmarshal(bytes, &sb)
	if err != nil {
		return nil, err
	}

	return &sb, nil
}
