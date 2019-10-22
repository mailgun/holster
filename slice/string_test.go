package slice_test

import (
	"strings"
	"testing"

	"github.com/mailgun/holster/slice"
	"github.com/stretchr/testify/assert"
)

func TestContainsString(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		str      string
		modifier func(string) string
		want     bool
	}{
		{
			name:     "Slice contains a specific string",
			slice:    []string{"aa", "bb", "CC"},
			modifier: strings.ToLower,
			str:      "CC",
			want:     true,
		},
		{
			name:  "Slice contains a string, but it is with upper cases and modifier is nil",
			slice: []string{"aa", "bb", "CC"},
			str:   "cc",
			want:  false,
		},
		{
			name:     "Slice contains a string with upper cases and modifier ToLower is provided",
			slice:    []string{"AA", "bb", "cc"},
			modifier: strings.ToLower,
			str:      "aa",
			want:     true,
		},
		{
			name:  "Slice does not contains string",
			slice: []string{"AA", "bb", "cc"},
			str:   "notExist",
			want:  false,
		},
		{
			name:  "Empty slice",
			slice: []string{},
			str:   "notExist",
			want:  false,
		},
	}

	for _, tt := range tests {
		got := slice.ContainsString(tt.str, tt.slice, tt.modifier)
		assert.Equal(t, tt.want, got)
	}
}

func TestContainsStringIgnoreCase(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		str   string
		want  bool
	}{
		{
			name:  "Slice contains a specific string, but with different upper case",
			slice: []string{"aa", "bb", "cC"},
			str:   "Cc",
			want:  true,
		},
		{
			name:  "Slice contains a string, but it is with upper case",
			slice: []string{"aa", "bb", "CC"},
			str:   "cc",
			want:  true,
		},
		{
			name:  "Slice contains a string, but it is with lower cases",
			slice: []string{"aa", "bb", "cc"},
			str:   "AA",
			want:  true,
		},
		{
			name:  "Slice does not contains string",
			slice: []string{"AA", "bb", "cc"},
			str:   "notExist",
			want:  false,
		},
		{
			name:  "Empty slice",
			slice: []string{},
			str:   "notExist",
			want:  false,
		},
	}

	for _, tt := range tests {
		got := slice.ContainsStringEqualFold(tt.str, tt.slice)
		assert.Equal(t, tt.want, got)
	}
}
