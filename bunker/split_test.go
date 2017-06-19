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
package bunker_test

import (
	"github.com/mailgun/holster/bunker"
	. "gopkg.in/check.v1"
)

type SplitSuite struct{}

var _ = Suite(&SplitSuite{})

func (s *SplitSuite) TestSplit(c *C) {
	save := bunker.GetChunkSizeBytes
	defer func() { bunker.GetChunkSizeBytes = save }()

	// splitting in chunks of 1 byte
	bunker.GetChunkSizeBytes = func() int { return 1 }

	// each ASCII character takes 1 byte so there should be 5 chunks in the word "hello"
	c.Assert(bunker.Split("hello"), DeepEquals, []string{"h", "e", "l", "l", "o"})

	// each Cyrillic character takes 2 bytes but we cannot "cut" character byte representation
	// in half so the algorithm grab the whole character if the split position is in the
	// middle of its byte representation, so 6 chunks for the word "привет" (means "hi" in Russian)
	c.Assert(bunker.Split("привет"), DeepEquals, []string{"п", "р", "и", "в", "е", "т"})

	// now splitting in 2 byte chunks
	bunker.GetChunkSizeBytes = func() int { return 2 }

	// ASCII characters is 1 byte, so each chunk fits two
	c.Assert(bunker.Split("hello"), DeepEquals, []string{"he", "ll", "o"})

	// Cyrillic characters is 2 byte so now each character occupies the whole chunk
	c.Assert(bunker.Split("привет"), DeepEquals, []string{"п", "р", "и", "в", "е", "т"})

	// some mixed ASCII and Cyrillic characters
	c.Assert(bunker.Split("привет, world"), DeepEquals, []string{"п", "р", "и", "в", "е", "т", ", ", "wo", "rl", "d"})
}
