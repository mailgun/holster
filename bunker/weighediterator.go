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
	"math/rand"
	"time"
)

// weighedIterator produces a slice of permutations of a slice of names it was
// initialized with based on their weights on every `Next()` call.
type weighedIterator interface {
	Next() []string
}

type iterator struct {
	names        []string
	weighedNames []string
}

func newWeighedIterator(namesToWeights map[string]int) weighedIterator {
	// random generator will be needed when picking clusters based on weight
	rand.Seed(time.Now().UnixNano())

	var names []string
	var weighedNames []string
	for name, weight := range namesToWeights {
		names = append(names, name)
		for i := 0; i < weight; i++ {
			weighedNames = append(weighedNames, name)
		}
	}

	return &iterator{names, weighedNames}
}

// Next returns a slice of permutations of `names` the iterator was initialized with,
// in the random order determined by their weights.
//
// A name with a higher weight is more likely to precede in the slice.
func (i *iterator) Next() []string {
	var names []string

	// shuffle the slice of all weighed names
	permutations := rand.Perm(len(i.weighedNames))

	// iterate over the slice of all weighed names in the order determined by
	// the permutations
	for _, idx := range permutations {
		currentName := i.weighedNames[idx]

		// first check if the current name is not already present in the final slice
		skip := false
		for _, name := range names {
			if currentName == name {
				skip = true
				break
			}
		}

		// the name is already in `names` so let's skip it
		if skip {
			continue
		}

		names = append(names, currentName)

		// if the final `names` slice already has all names in it, no need to iterate
		// further, let's exit early
		if len(names) == len(i.names) {
			break
		}
	}

	return names
}
