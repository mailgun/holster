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

import "unicode/utf8"

// saved messages are split in chunks of this size
const chunkSizeBytes = 250 * 1024 // 250KB

// split takes a message string, splits it into chunks of size specified by
// the CHUNK_SIZE_BYTES and returns a slice of those chunks.
func Split(message string) []string {
	var chunks []string

	messageLen := len(message)

	chunkStart := 0
	chunkEnd := chunkStart + GetChunkSizeBytes()

	for {
		// this is the last chunk, grab it
		if chunkEnd >= messageLen {
			chunks = append(chunks, message[chunkStart:messageLen])
			break
		}

		// is next character a rune start?
		runeStart := utf8.RuneStart(byte(message[chunkEnd]))

		if !runeStart {
			// advance the right border of the current chunk until we find the start of
			// next rune or reach the end of the message
			for {
				chunkEnd += 1
				if chunkEnd == messageLen || utf8.RuneStart(byte(message[chunkEnd])) {
					break
				}
			}
		}

		chunks = append(chunks, message[chunkStart:chunkEnd])

		// if it was the last chunk, exit
		if chunkEnd == messageLen {
			break
		}

		chunkStart = chunkEnd
		chunkEnd = chunkStart + GetChunkSizeBytes()
	}

	return chunks
}

// for mocking in tests
var GetChunkSizeBytes = func() int {
	return chunkSizeBytes
}
