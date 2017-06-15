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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// bkey (bunker key) groups together various pieces of information that are encoded into bunker keys
type bkey struct {
	Key        string `json:"k"`
	Cluster    string `json:"c"`
	Packed     bool   `json:"p"`
	Compressed bool   `json:"x"`
	Encrypted  bool   `json:"e"`
	Signature  string `json:"s"`
}

func encodeKey(key *bkey, hmacKey []byte) (string, error) {
	key.Signature = generateSignature(key, hmacKey)

	dump, err := json.Marshal(key)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(dump), nil
}

func decodeKey(encKey string, hmacKey []byte) (*bkey, error) {
	var dump []byte
	var err error

	// base64 decode
	if dump, err = base64.URLEncoding.DecodeString(encKey); err != nil {
		return nil, fmt.Errorf("failed to base64-decode: %v %v", encKey, err)
	}

	key := &bkey{}
	if err = json.Unmarshal(dump, key); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %s %v", dump, err)
	}

	// verify signature
	if key.Signature != generateSignature(key, hmacKey) {
		return nil, fmt.Errorf("signature didn't validate: %v", key)
	}

	return key, nil
}

func generateSignature(k *bkey, hmacKey []byte) string {
	toSign := fmt.Sprintf("k=%s;c=%s", k.Key, k.Cluster)

	if k.Packed {
		toSign += ";p=y"
	}

	mac := hmac.New(sha256.New, hmacKey)
	mac.Write([]byte(toSign))
	return hex.EncodeToString(mac.Sum(nil))[:10]
}
