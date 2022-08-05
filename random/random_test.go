package random

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestString(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	for _, tc := range []struct {
		msg            string
		prefix         string
		length         int
		contains       string
		expectedLength int
	}{{
		msg:            "only made of alpha and numeric characters",
		prefix:         "",
		length:         10,
		contains:       AlphaRunes + NumericRunes,
		expectedLength: 10,
	}, {
		msg:            "with prefix and add to length",
		prefix:         "abc",
		length:         5,
		contains:       AlphaRunes + NumericRunes,
		expectedLength: 5 + 3, // = len("abc")
	}} {
		t.Run(tc.msg, func(t *testing.T) {
			res := String(tc.prefix, tc.length)
			assert.Equal(t, tc.expectedLength, len(res))
			assert.Contains(t, res, tc.prefix)
			for _, ch := range res {
				assert.Contains(t, tc.contains, fmt.Sprintf("%c", ch))
			}
		})
	}
}

func TestAlpha(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	for _, tc := range []struct {
		msg            string
		prefix         string
		length         int
		contains       string
		expectedLength int
	}{{
		msg:            "only made of alpha characters",
		prefix:         "",
		length:         10,
		contains:       AlphaRunes,
		expectedLength: 10,
	}, {
		msg:            "with prefix and add to length",
		prefix:         "abc",
		length:         5,
		contains:       AlphaRunes,
		expectedLength: 5 + 3, // = len("abc")
	}} {
		t.Run(tc.msg, func(t *testing.T) {
			res := Alpha(tc.prefix, tc.length)
			assert.Equal(t, tc.expectedLength, len(res))
			assert.Contains(t, res, tc.prefix)
			for _, ch := range res {
				assert.Contains(t, tc.contains, fmt.Sprintf("%c", ch))
			}
		})
	}
}

func TestItem(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	for _, tc := range []struct {
		msg      string
		items    []string
		expected string
	}{{
		msg:   "one of the given list",
		items: []string{"com", "net", "org"},
	}} {
		t.Run(tc.msg, func(t *testing.T) {
			res := Item(tc.items...)
			assert.Contains(t, tc.items, res)
		})
	}
}
