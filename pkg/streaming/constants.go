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

// ChunkSize is the size of a file chunk in bytes for streaming transfer
// Apparently this is an optimal chunk size according to https://github.com/grpc/grpc.github.io/issues/371
const ChunkSize = 64 * 1024

// Threshold is the minimum size for a file to be streamed instead of transfered in one message
const Threshold = 4 * 1024 * 1024 // Size in bytes
