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

package metadataserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

	mpb "github.com/kurafs/kura/pkg/pb/metadata"
	spb "github.com/kurafs/kura/pkg/pb/storage"
	"google.golang.org/grpc"

	"github.com/kurafs/kura/pkg/log"
)

type testStorageServiceClient struct{}

var testGetFileResp []byte = []byte("test-get-file-resp")

func (t *testStorageServiceClient) GetFile(
	ctx context.Context, in *spb.GetFileRequest, opts ...grpc.CallOption,
) (*spb.GetFileResponse, error) {
	if in.Key != metadataFileKey {
		return &spb.GetFileResponse{File: testGetFileResp}, nil
	}

	metadata := &MetadataFile{
		Entries: make(map[string]mpb.FileMetadata),
	}
	serialized, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	return &spb.GetFileResponse{File: serialized}, nil
}

func (y *testStorageServiceClient) PutFile(
	ctx context.Context, in *spb.PutFileRequest, opts ...grpc.CallOption,
) (*spb.PutFileResponse, error) {
	return &spb.PutFileResponse{}, nil
}

func (y *testStorageServiceClient) DeleteFile(
	ctx context.Context, in *spb.DeleteFileRequest, opts ...grpc.CallOption,
) (*spb.DeleteFileResponse, error) {
	return &spb.DeleteFileResponse{}, nil
}

func TestGetFile(t *testing.T) {
	logger := log.New(log.Writer(ioutil.Discard))
	ctx := context.Background()

	testStorageClient := &testStorageServiceClient{}
	metadataServer := newMetadataServer(logger, testStorageClient)
	req := &mpb.GetFileRequest{Key: "get-file-req"}
	res, err := metadataServer.GetFile(ctx, req)
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

	testStorageClient := &testStorageServiceClient{}
	metadataServer := newMetadataServer(logger, testStorageClient)
	req := &mpb.PutFileRequest{Key: "put-file-req", File: []byte("file")}
	_, err := metadataServer.PutFile(ctx, req)
	if err != nil {
		t.Error(err)
	}
}

func TestDeleteFile(t *testing.T) {
	logger := log.New(log.Writer(ioutil.Discard))
	ctx := context.Background()

	testStorageClient := &testStorageServiceClient{}
	metadataServer := newMetadataServer(logger, testStorageClient)
	req := &mpb.DeleteFileRequest{Key: "delete-file-req"}
	_, err := metadataServer.DeleteFile(ctx, req)
	if err != nil {
		t.Error(err)
	}
}