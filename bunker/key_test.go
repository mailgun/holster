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

import . "gopkg.in/check.v1"

type KeySuite struct {
}

var _ = Suite(&KeySuite{})

func (s *KeySuite) TestEncodeDecodeKey(c *C) {
	encKey, err := encodeKey(
		&bkey{"key-1", "cluster-1", false, true, false, ""},
		testHMACKey)
	c.Assert(err, IsNil)

	// success
	key, err := decodeKey(encKey, testHMACKey)
	c.Assert(err, IsNil)
	c.Assert(key.Key, Equals, "key-1")
	c.Assert(key.Cluster, Equals, "cluster-1")
	c.Assert(key.Packed, Equals, false)
	c.Assert(key.Compressed, Equals, true)
	c.Assert(key.Encrypted, Equals, false)

	// bad key
	key, err = decodeKey("blah", testHMACKey)
	c.Assert(err, NotNil)

	// bad signature (emulated by providing different HMAC key)
	key, err = decodeKey(encKey, []byte("blah"))
	c.Assert(err, NotNil)
}
