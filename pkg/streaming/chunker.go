// Copyright 2018 The Kura Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package streaming

// Chunker is an iterator which returns parts of a byte array
type Chunker struct {
	part   int
	source []byte
}

func NewChunker(source []byte) *Chunker {
	return &Chunker{part: -1, source: source}
}

// Value returns the current value of the Chunker
func (c *Chunker) Value() []byte {
	end := (c.part + 1) * ChunkSize
	if end >= len(c.source) {
		end = len(c.source)
	}
	return c.source[c.part*ChunkSize : end]
}

// Next advances the iterator to the next chunk
func (c *Chunker) Next() bool {
	// TODO(irfansharif): We need to start it with chunker.Next(), should be
	// positioned at the beginning.
	c.part++
	if c.part*ChunkSize >= len(c.source) {
		return false
	}
	return true
}
