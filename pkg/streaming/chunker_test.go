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

import (
	"bytes"
	"testing"
)

func TestChunker(t *testing.T) {
	parts := 16
	extra := []byte("efghijk")
	chunk := bytes.Repeat([]byte("abcd"), ChunkSize/4)
	source := make([]byte, 0, len(chunk)*parts+len(extra))
	for i := 0; i < parts; i++ {
		source = append(source, chunk...)
	}
	source = append(source, extra...)

	chunker := NewChunker(source)
	for i := 0; i < parts; i++ {
		if !chunker.Next() {
			t.Error("Should have shown another chunk")
		}
		val := chunker.Value()
		if !bytes.Equal(val, chunk) {
			t.Error("Chunk should have been equivalent")
		}
	}
	if !chunker.Next() {
		t.Error("Last chunk was not found")
	}
	lastVal := chunker.Value()
	if !bytes.Equal(lastVal, extra) {
		t.Errorf("Trailing chunk should have been %s, got %s", extra, lastVal)
	}
	if chunker.Next() {
		t.Errorf("Shouldn't have gotten another chunk")
	}
}
