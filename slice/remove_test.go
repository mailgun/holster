package slice_test

import (
	"fmt"
	"testing"

	"github.com/mailgun/holster/v4/slice"
	"github.com/stretchr/testify/assert"
)

func ExampleRemove() {
	s := []string{"1", "2", "3"}
	i := []int{1, 2, 3}

	fmt.Println(slice.Remove(s, 0, 2))
	fmt.Println(slice.Remove(i, 1, 2))
	// Output:
	// [3]
	// [1 3]
}

func TestRemove(t *testing.T) {
	for _, test := range []struct {
		name     string
		i        int
		j        int
		inSlice  []int
		outSlice []int
	}{{
		name:     "empty slice",
		inSlice:  []int{},
		outSlice: []int{},
	}, {
		name:     "nil check",
		i:        1,
		j:        3,
		inSlice:  nil,
		outSlice: nil,
	}, {
		name:     "start index out of range should not panic or modify the slice",
		i:        -2,
		j:        2,
		inSlice:  []int{2},
		outSlice: []int{2},
	}, {
		name:     "end index out of range should not panic or modify the slice",
		i:        0,
		j:        5,
		inSlice:  []int{2},
		outSlice: []int{2},
	}, {
		name:     "invalid start and end index should not panic or modify the slice",
		i:        4,
		j:        7,
		inSlice:  []int{2},
		outSlice: []int{2},
	}, {
		name:     "single value remove",
		i:        0,
		j:        1,
		inSlice:  []int{1, 2, 3},
		outSlice: []int{2, 3},
	}, {
		name:     "middle value removed",
		i:        1,
		j:        2,
		inSlice:  []int{1, 2, 3},
		outSlice: []int{1, 3},
	}, {
		name:     "last value removed",
		i:        2,
		j:        3,
		inSlice:  []int{1, 2, 3},
		outSlice: []int{1, 2},
	}, {
		name:     "multi-value remove",
		i:        1,
		j:        3,
		inSlice:  []int{1, 2, 3},
		outSlice: []int{1},
	}, {
		name:     "end of slice gut check for invalid ending index",
		i:        2,
		j:        5, // This index being out of bounds shouldn't matter
		inSlice:  []int{1, 2, 3},
		outSlice: []int{1, 2},
	}} {
		t.Run(test.name, func(t *testing.T) {
			got := slice.Remove(test.inSlice, test.i, test.j)
			for i := range test.outSlice {
				assert.Equal(t, test.outSlice[i], got[i])
			}
		})
	}
}

var out []string

func BenchmarkRemove(b *testing.B) {
	o := []string{}
	in := []string{"localhost", "mg.example.com", "testlabs.io", "localhost", "m.example.com.br", "localhost", "localhost"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		o = slice.Remove(in, 2, 4)
	}
	out = o
}
