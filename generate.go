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

package main

//go:generate protoc -I=pkg/rpc/storage --go_out=plugins=grpc:pkg/rpc/storage pkg/rpc/storage/storage.proto
//go:generate protoc -I=pkg/rpc/metadata --go_out=plugins=grpc:pkg/rpc/metadata pkg/rpc/metadata/metadata.proto
