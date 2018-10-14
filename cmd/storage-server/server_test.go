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

package storageserver

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/kurafs/kura/pkg/log"
	spb "github.com/kurafs/kura/pkg/pb/storage"
)

var testGetFileResp []byte = []byte("test-get-file-resp")
var deleteFileReqKey string = "delete-file-req"

type testStore struct{}

func (t *testStore) Read(key string) ([]byte, error) {
	return testGetFileResp, nil
}

func (t *testStore) Write(key string, val []byte) error {
	return nil
}

func (t *testStore) Has(key string) bool {
	if key == deleteFileReqKey {
		return true
	}
	return false
}

func (t *testStore) Erase(key string) error {
	return nil
}

func TestGetFile(t *testing.T) {
	logger := log.New(log.Writer(ioutil.Discard))
	ctx := context.Background()

	testStore := &testStore{}
	storageServer := newStorageServer(logger, testStore)
	req := &spb.GetFileRequest{Key: "get-file-req"}
	res, err := storageServer.GetFile(ctx, req)
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(res.File, testGetFileResp) {
		t.Error(fmt.Sprintf("expected res.File = %v, got %v", testGetFileResp, res.File))
	}
}

func TestPutFile(t *testing.T) {
	logger := log.New(log.Writer(ioutil.Discard))
	ctx := context.Background()

	testStore := &testStore{}
	storageServer := newStorageServer(logger, testStore)
	req := &spb.PutFileRequest{Key: "put-file-req", File: []byte("file")}
	_, err := storageServer.PutFile(ctx, req)
	if err != nil {
		t.Error(err)
	}
}

func TestDeleteFile(t *testing.T) {
	logger := log.New(log.Writer(ioutil.Discard))
	ctx := context.Background()

	testStore := &testStore{}
	storageServer := newStorageServer(logger, testStore)
	req := &spb.DeleteFileRequest{Key: deleteFileReqKey}
	_, err := storageServer.DeleteFile(ctx, req)
	if err != nil {
		t.Error(err)
	}
}
