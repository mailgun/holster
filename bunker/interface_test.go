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
	"github.com/mailgun/holster/secret"
	. "gopkg.in/check.v1"
)

type InterfaceSuite struct {
}

var _ = Suite(&InterfaceSuite{})

func (s *InterfaceSuite) TestPutWithOptions(c *C) {
	if err := Init(testConf, testHMACKey); err != nil {
		c.Fatal(err)
	}

	secretKey, err := secret.NewKey()
	c.Assert(err, IsNil)

	secretService, err := secret.New(&secret.Config{KeyBytes: secretKey})
	c.Assert(err, IsNil)

	err = SetSecretService(secretService)
	c.Assert(err, IsNil)

	key, err := PutWithOptions("all options", PutOptions{Compress: true, Encrypt: true})
	c.Assert(err, IsNil)

	message, err := Get(key)
	c.Assert(err, IsNil)
	c.Assert(message, Equals, "all options")

	err = Delete(key)
	c.Assert(err, IsNil)

	message, err = Get(key)
	c.Assert(err, NotNil)
}

func (s *InterfaceSuite) TestNotInitialized(c *C) {
	DefaultBunker = nil

	_, err := Put("hello")
	c.Assert(err, NotNil)

	_, err = Get("key")
	c.Assert(err, NotNil)

	err = Delete("key")
	c.Assert(err, NotNil)

	secretKey, err := secret.NewKey()
	c.Assert(err, IsNil)

	secretService, err := secret.New(&secret.Config{KeyBytes: secretKey})
	c.Assert(err, IsNil)

	err = SetSecretService(secretService)
	c.Assert(err, NotNil)
}
