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
	"testing"

	"github.com/kurafs/kura/pkg/log"
	mpb "github.com/kurafs/kura/pkg/pb/metadata"
	spb "github.com/kurafs/kura/pkg/pb/storage"
	"google.golang.org/grpc"
)

type testStorageServiceClient struct {
	gcWasRun bool
}

var testGetFileResp []byte = []byte("test-get-file-resp")
var testGetFileKeysResp = []string{"key1", "key2"}
var testGetFileKeysAfterGCResp = []string{"key1"}

func (t *testStorageServiceClient) GetBlob(
	ctx context.Context, in *spb.GetBlobRequest, opts ...grpc.CallOption,
) (*spb.GetBlobResponse, error) {
	if in.Key != metadataFileKey {
		return &spb.GetBlobResponse{Data: testGetFileResp}, nil
	}

	metadata := &MetadataFile{
		Entries: make(map[string]mpb.Metadata),
	}
	serialized, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	return &spb.GetBlobResponse{Data: serialized}, nil
}

func (t *testStorageServiceClient) PutBlob(
	ctx context.Context, in *spb.PutBlobRequest, opts ...grpc.CallOption,
) (*spb.PutBlobResponse, error) {
	return &spb.PutBlobResponse{}, nil
}

func (t *testStorageServiceClient) DeleteBlob(
	ctx context.Context, in *spb.DeleteBlobRequest, opts ...grpc.CallOption,
) (*spb.DeleteBlobResponse, error) {
	return &spb.DeleteBlobResponse{}, nil
}

func (t *testStorageServiceClient) RenameBlob(
	ctx context.Context, in *spb.RenameBlobRequest, opts ...grpc.CallOption,
) (*spb.RenameBlobResponse, error) {
	return &spb.RenameBlobResponse{}, nil
}

func (y *testStorageServiceClient) GetBlobKeys(
	ctx context.Context, in *spb.GetBlobKeysRequest, opts ...grpc.CallOption,
) (*spb.GetBlobKeysResponse, error) {
	if y.gcWasRun {
		return &spb.GetBlobKeysResponse{Keys: testGetFileKeysAfterGCResp}, nil
	} else {
		return &spb.GetBlobKeysResponse{Keys: testGetFileKeysResp}, nil
	}
}

func (y *testStorageServiceClient) GetBlobStream(
	ctx context.Context, in *spb.GetBlobStreamRequest, opts ...grpc.CallOption,
) (spb.StorageService_GetBlobStreamClient, error) {
	return nil, nil
}

func (y *testStorageServiceClient) PutBlobStream(
	ctx context.Context, opts ...grpc.CallOption,
) (spb.StorageService_PutBlobStreamClient, error) {
	return nil, nil
}

func TestGetFile(t *testing.T) {
	logger := log.Discarder()
	ctx := context.Background()

	testStorageClient := &testStorageServiceClient{}
	metadataServer := newMetadataServer(logger, testStorageClient)
	req := &mpb.GetFileRequest{Path: "get-file-req"}
	res, err := metadataServer.GetFile(ctx, req)
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(res.File, testGetFileResp) {
		t.Error(fmt.Sprintf("expected res.File = %v, got %v", testGetFileResp, res.File))
	}
}

func TestPutFile(t *testing.T) {
	logger := log.Discarder()
	ctx := context.Background()

	testStorageClient := &testStorageServiceClient{}
	metadataServer := newMetadataServer(logger, testStorageClient)
	metadata := &mpb.Metadata{}
	req := &mpb.PutFileRequest{
		Path:     "put-file-req",
		File:     []byte("file"),
		Metadata: metadata,
	}
	_, err := metadataServer.PutFile(ctx, req)
	if err != nil {
		t.Error(err)
	}
}

func TestDeleteFile(t *testing.T) {
	logger := log.Discarder()
	ctx := context.Background()

	testStorageClient := &testStorageServiceClient{}
	metadataServer := newMetadataServer(logger, testStorageClient)
	req := &mpb.DeleteFileRequest{Path: "delete-file-req"}
	_, err := metadataServer.DeleteFile(ctx, req)
	if err != nil {
		t.Error(err)
	}
}

func TestGarbageCollection(t *testing.T) {
	logger := log.Discarder()
	ctx := context.Background()

	testStorageClient := &testStorageServiceClient{}
	metadataServer := newMetadataServer(logger, testStorageClient)
	metadata := &mpb.Metadata{}
	req := &mpb.PutFileRequest{
		Path:     "key1",
		File:     []byte("file"),
		Metadata: metadata,
	}
	_, err := metadataServer.PutFile(ctx, req)
	if err != nil {
		t.Error(err)
	}

	sreq := &spb.PutBlobRequest{Key: "key2", Data: []byte("file2")}
	_, err = testStorageClient.PutBlob(context.Background(), sreq)

	kreq := &spb.GetBlobKeysRequest{}
	kres, err := testStorageClient.GetBlobKeys(context.Background(), kreq)

	if err != nil {
		t.Error(err)
	}

	if len(kres.Keys) != 2 {
		t.Error(fmt.Sprintf("expected = %v keys, got %v", 2, len(kres.Keys)))
	}

	metadataServer.runGarbageCollection(context.Background())
	testStorageClient.gcWasRun = true

	kreq = &spb.GetBlobKeysRequest{}
	kres, err = testStorageClient.GetBlobKeys(context.Background(), kreq)

	if len(kres.Keys) != 1 {
		t.Error(fmt.Sprintf("expected = %v keys, got %v", 1, len(kres.Keys)))
	}
}
