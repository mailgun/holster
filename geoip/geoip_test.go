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

package geoip

import (
	"testing"

	"fmt"

	"github.com/mailgun/events"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) {
	TestingT(t)
}

type GeoIPSuite struct {
}

var _ = Suite(&GeoIPSuite{})

func (s *GeoIPSuite) SetUpSuite(c *C) {
	DatabasePath = "./assets/GeoLite2-City.mmdb"
}

func (s *GeoIPSuite) TestGeoDataFromIp(c *C) {
	fmt.Println(DatabasePath)
	c.Assert(GeoDataFromIp(""), Equals, unknownData)
	c.Assert(GeoDataFromIp("10.0.0.1"), Equals, unknownData)
	c.Assert(GeoDataFromIp("127.0.0.1"), Equals, unknownData)

	data := events.GeoLocation{
		Country: "US",
		Region:  "CA",
		City:    "Mountain View",
	}
	c.Assert(GeoDataFromIp("173.194.35.210"), Equals, data)

	data = events.GeoLocation{
		Country: "GB",
		Region:  "ENG",
		City:    "London",
	}
	c.Assert(GeoDataFromIp("81.2.69.142"), Equals, data)
}
