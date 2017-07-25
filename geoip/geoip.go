//go:generate go run generate.go
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
	"fmt"
	"net"

	"github.com/mailgun/events"
	"github.com/oschwald/geoip2-golang"
)

const (
	DefaultUnknown = "Unknown"
)

var DatabasePath = "/var/mailgun/GeoLite2-City.mmdb"

var db *geoip2.Reader
var unknownData = events.GeoLocation{
	City:    DefaultUnknown,
	Country: DefaultUnknown,
	Region:  DefaultUnknown,
}

func GeoDataFromIp(ip string) events.GeoLocation {
	if ip == "" {
		return unknownData
	}

	if db == nil {
		loadDB()
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		fmt.Errorf("Invalid IP given: %s", ip)
		return unknownData
	}

	record, err := db.City(parsedIP)
	if err != nil {
		fmt.Errorf("Error: %s", err)
		return unknownData
	}

	// Not found in the database
	if record.City.GeoNameID == 0 {
		return unknownData
	}

	return events.GeoLocation{
		City:    record.City.Names["en"],
		Country: record.Country.IsoCode,
		Region:  record.Subdivisions[0].IsoCode,
	}
}

func loadDB() error {
	var err error
	db, err = geoip2.Open(DatabasePath)
	if err != nil {
		return fmt.Errorf("Failed to initialize GeoIP engine: %s", err)
	}
	return nil
}
