package clock

import (
	"strconv"
)

// Allows seamless JSON encoding/decoding of rfc822 formatted timestamps.
// https://www.ietf.org/rfc/rfc822.txt section 5.
type RFC822Time struct {
	Time
}

// NewRFC822Time creates RFC822Time from a standard Time. The created value is
// truncated down to second precision because RFC822 does not allow for better.
func NewRFC822Time(t Time) RFC822Time {
	return RFC822Time{Time: t.Truncate(Second)}
}

// ParseRFC822Time parses an RFC822 time string.
func ParseRFC822Time(s string) (Time, error) {
	t, err := parseRFC822TimeAbreviated(s)
	if err == nil {
		return t, err
	}
	if parseErr, ok := err.(*ParseError); !ok {
		return Time{}, parseErr
	}

	return parseRFC822Time(s)
}

// parseRFC822TimeAbbreviated attempts to parse a an RFC822 time with the month abbreviated.
func parseRFC822TimeAbreviated(s string) (Time, error) {
	t, err := Parse("Mon, 2 Jan 2006 15:04:05 MST", s)
	if err == nil {
		return t, nil
	}
	if parseErr, ok := err.(*ParseError); !ok || (parseErr.LayoutElem != "MST" && parseErr.LayoutElem != "Mon") {
		return Time{}, parseErr
	}
	if t, err = Parse("Mon, 2 Jan 2006 15:04:05 -0700", s); err == nil {
		return t, nil
	}
	if parseErr, ok := err.(*ParseError); !ok || (parseErr.LayoutElem != "" && parseErr.LayoutElem != "Mon") {
		return Time{}, parseErr
	}
	if t, err = Parse("Mon, 2 Jan 2006 15:04:05 -0700 (MST)", s); err == nil {
		return t, err
	}
	if parseErr, ok := err.(*ParseError); !ok || parseErr.LayoutElem != "Mon" {
		return Time{}, err
	}
	if t, err = Parse("2 Jan 2006 15:04:05 MST", s); err == nil {
		return t, err
	}
	if parseErr, ok := err.(*ParseError); !ok || parseErr.LayoutElem != "MST" {
		return Time{}, parseErr
	}
	if t, err = Parse("2 Jan 2006 15:04:05 -0700", s); err == nil {
		return t, err
	}
	if parseErr, ok := err.(*ParseError); !ok || parseErr.LayoutElem != "" {
		return Time{}, parseErr
	}
	if t, err = Parse("2 Jan 2006 15:04:05 -0700 (MST)", s); err == nil {
		return t, err
	}
	return Time{}, err
}

// parseRFC822Time attempts to parse an RFC822 time with the full month name.
func parseRFC822Time(s string) (Time, error) {
	t, err := Parse("Mon, 2 January 2006 15:04:05 MST", s)
	if err == nil {
		return t, nil
	}
	if parseErr, ok := err.(*ParseError); !ok || (parseErr.LayoutElem != "MST" && parseErr.LayoutElem != "Mon") {
		return Time{}, parseErr
	}
	if t, err = Parse("Mon, 2 January 2006 15:04:05 -0700", s); err == nil {
		return t, nil
	}
	if parseErr, ok := err.(*ParseError); !ok || (parseErr.LayoutElem != "" && parseErr.LayoutElem != "Mon") {
		return Time{}, parseErr
	}
	if t, err = Parse("Mon, 2 January 2006 15:04:05 -0700 (MST)", s); err == nil {
		return t, err
	}
	if parseErr, ok := err.(*ParseError); !ok || parseErr.LayoutElem != "Mon" {
		return Time{}, err
	}
	if t, err = Parse("2 January 2006 15:04:05 MST", s); err == nil {
		return t, err
	}
	if parseErr, ok := err.(*ParseError); !ok || parseErr.LayoutElem != "MST" {
		return Time{}, parseErr
	}
	if t, err = Parse("2 January 2006 15:04:05 -0700", s); err == nil {
		return t, err
	}
	if parseErr, ok := err.(*ParseError); !ok || parseErr.LayoutElem != "" {
		return Time{}, parseErr
	}
	if t, err = Parse("2 January 2006 15:04:05 -0700 (MST)", s); err == nil {
		return t, err
	}
	return Time{}, err
}

// NewRFC822Time creates RFC822Time from a Unix timestamp (seconds from Epoch).
func NewRFC822TimeFromUnix(timestamp int64) RFC822Time {
	return RFC822Time{Time: Unix(timestamp, 0).UTC()}
}

func (t RFC822Time) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(t.Format(RFC1123))), nil
}

func (t *RFC822Time) UnmarshalJSON(s []byte) error {
	q, err := strconv.Unquote(string(s))
	if err != nil {
		return err
	}
	parsed, err := ParseRFC822Time(q)
	if err != nil {
		return err
	}
	t.Time = parsed
	return nil
}

func (t RFC822Time) String() string {
	return t.Format(RFC1123)
}

func (t RFC822Time) StringWithOffset() string {
	return t.Format(RFC1123Z)
}
