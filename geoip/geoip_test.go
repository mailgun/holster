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

package geoip_test

import (
	"testing"

	"github.com/mailgun/events"
	"github.com/mailgun/holster/geoip"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) {
	TestingT(t)
}

type GeoIPSuite struct {
}

var _ = Suite(&GeoIPSuite{})

func (s *GeoIPSuite) SetUpSuite(c *C) {
	geoip.DatabasePath = "./assets/GeoLite2-City.mmdb"
}

func (s *GeoIPSuite) TestGetEventFromIp(c *C) {
	c.Assert(geoip.GetEventFromIp(""), Equals, geoip.UnknownData)
	c.Assert(geoip.GetEventFromIp("10.0.0.1"), Equals, geoip.UnknownData)
	c.Assert(geoip.GetEventFromIp("127.0.0.1"), Equals, geoip.UnknownData)

	data := events.GeoLocation{
		Country: "US",
		Region:  "CA",
		City:    "Mountain View",
	}
	c.Assert(geoip.GetEventFromIp("173.194.35.210"), Equals, data)

	data = events.GeoLocation{
		Country: "GB",
		Region:  "ENG",
		City:    "London",
	}
	c.Assert(geoip.GetEventFromIp("81.2.69.142"), Equals, data)
}
