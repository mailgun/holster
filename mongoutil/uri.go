package mongoutil

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

type Config struct {
	Servers  []string                 `json:"servers"`
	Database string                   `json:"database"`
	URI      string                   `json:"uri"`
	Options  []map[string]interface{} `json:"options"`
}

func MongoURI() string {
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		return "mongodb://127.0.0.1:27017/mg_test"
	}
	return mongoURI
}

func (c Config) URIWithOptions() string {
	URI := c.URI

	// Create an URI using the Servers list and Database if provided
	if len(c.Servers) != 0 && c.Database != "" {
		URI = fmt.Sprintf("mongodb://%s/%s", strings.Join(c.Servers, ","), c.Database)
	}

	type opt struct {
		key   string
		value string
	}
	adjustedURI := URI
	var options []opt

	// Parse options from the URI.
	qmIdx := strings.Index(URI, "?")
	if qmIdx > 0 {
		adjustedURI = URI[:qmIdx]
		for _, pair := range strings.Split(URI[qmIdx+1:], "&") {
			eqIdx := strings.Index(pair, "=")
			if eqIdx > 0 {
				options = append(options, opt{key: pair[:eqIdx], value: pair[eqIdx+1:]})
			}
		}
	}

	// NOTE: The options are an ordered list because mongo cares
	// about the order of some options like replica tag order.

	// Override URI options with config options.
	for _, o := range c.Options {
		for optName, optVal := range o {
			switch optVal := optVal.(type) {
			case int:
				options = append(options, opt{key: toCamelCase(optName), value: strconv.Itoa(optVal)})
			case float64:
				options = append(options, opt{key: toCamelCase(optName), value: strconv.Itoa(int(optVal))})
			case string:
				options = append(options, opt{key: toCamelCase(optName), value: optVal})
			}
		}
	}

	// Construct a URI as recognized by mgo.Dial
	firstOpt := true
	var buf bytes.Buffer
	buf.WriteString(adjustedURI)

	for i := range options {
		o := options[i]
		if firstOpt {
			buf.WriteRune('?')
			firstOpt = false
		} else {
			buf.WriteRune('&')
		}
		buf.WriteString(o.key)
		buf.WriteRune('=')
		buf.WriteString(o.value)
	}
	return buf.String()
}

func toCamelCase(s string) string {
	var buf bytes.Buffer
	capitalize := false
	for _, ch := range s {
		if ch == '_' {
			capitalize = true
			continue
		}
		if capitalize {
			capitalize = false
			buf.WriteRune(unicode.ToUpper(ch))
			continue
		}
		buf.WriteRune(ch)
	}
	return buf.String()
}
