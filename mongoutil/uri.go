package mongoutil

import (
	"bytes"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type Config struct {
	URI     string                 `json:"uri"`
	Options map[string]interface{} `json:"options"`
}

func (c Config) URIWithOptions() string {
	adjustedURI := c.URI
	options := make(map[string]string)

	// Parse options from the URI.
	qmIdx := strings.Index(c.URI, "?")
	if qmIdx > 0 {
		adjustedURI = c.URI[:qmIdx]
		for _, optNameEqVal := range strings.Split(c.URI[qmIdx+1:], "&") {
			eqIdx := strings.Index(optNameEqVal, "=")
			if eqIdx > 0 {
				options[optNameEqVal[:eqIdx]] = optNameEqVal[eqIdx+1:]
			}
		}
	}

	// Override URI options with config options.
	for optName, optVal := range c.Options {
		switch optVal := optVal.(type) {
		case int:
			options[toCamelCase(optName)] = strconv.Itoa(optVal)
		case float64:
			options[toCamelCase(optName)] = strconv.Itoa(int(optVal))
		case string:
			options[toCamelCase(optName)] = optVal
		}
	}

	// Construct a URI as recognized by mgo.Dial
	firstOpt := true
	var buf bytes.Buffer
	buf.WriteString(adjustedURI)

	optNames := make([]string, 0, len(options))
	for optName := range options {
		optNames = append(optNames, optName)
	}
	sort.Strings(optNames)

	for _, optName := range optNames {
		optVal := options[optName]
		if firstOpt {
			buf.WriteRune('?')
			firstOpt = false
		} else {
			buf.WriteRune('&')
		}
		buf.WriteString(optName)
		buf.WriteRune('=')
		buf.WriteString(optVal)
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
