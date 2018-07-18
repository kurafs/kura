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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/net/context"

	"github.com/kurafs/kura/pkg/log"
	mpb "github.com/kurafs/kura/pkg/pb/metadata"
	spb "github.com/kurafs/kura/pkg/pb/storage"
)

type MetadataFile struct {
	Entries map[string]mpb.FileMetadata
}

const (
	Key = iota
	Created
	LastModified
	Permissions
	Size
)

func BytesToMetadataFile(fileBytes []byte) (*MetadataFile, error) {
	if len(fileBytes) == 0 {
		return &MetadataFile{Entries: map[string]mpb.FileMetadata{}}, nil
	}

	var deserialized MetadataFile

	fmt.Println(fileBytes)
	err := json.Unmarshal(fileBytes, &deserialized)

	if err != nil {
		return nil, errors.New("Unable to deserialize metadata file")
	}

	fmt.Println(deserialized)

	return &deserialized, nil
}

func MetadataFileToBytes(metadata *MetadataFile) []byte {
	serialized, err := json.Marshal(metadata)

	if err != nil {
		fmt.Println("Unable to serialize metadata file")
		return nil
	}

	fmt.Println(serialized)

	return serialized
}

const metadataFileKey = "metadata"

type StorageClient struct {
	rpcClient spb.StorageServiceClient
	logger    *log.Logger
}

func NewStorageClient(rpcClient spb.StorageServiceClient, logger *log.Logger) *StorageClient {
	return &StorageClient{rpcClient, logger}
}

func (s *StorageClient) GetFile(key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	file, err := s.rpcClient.GetFile(ctx, &spb.GetFileRequest{Key: key})

	if err != nil {
		s.logger.Errorf("%v.GetFile(_) = _, %v\n", s.rpcClient, err)
		return nil, err
	}

	return file.File, nil
}

func (s *StorageClient) GetMetadataFile() (*MetadataFile, error) {
	b, err := s.GetFile(metadataFileKey)
	if err != nil {
		return nil, err
	}
	metadata, err := BytesToMetadataFile(b)
	if err != nil {
		return nil, err
	}
	return metadata, nil
}

func (s *StorageClient) PutFile(key string, file []byte) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	_, err := s.rpcClient.PutFile(ctx, &spb.PutFileRequest{Key: key, File: file})

	if err != nil {
		s.logger.Errorf("%v.PutFile(_) = _, %v\n", s.rpcClient, err)
		return false, err
	}

	return true, nil
}

func (s *StorageClient) PutMetadataFile(metadata *MetadataFile) (bool, error) {
	metadataBytes := MetadataFileToBytes(metadata)
	success, err := s.PutFile(metadataFileKey, metadataBytes)
	if err != nil {
		return false, err
	}
	return success, nil
}

func (s *StorageClient) DeleteFile(key string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, err := s.rpcClient.DeleteFile(ctx, &spb.DeleteFileRequest{Key: key})

	if err != nil {
		s.logger.Errorf("%v.DeleteFile(_) = _, %v\n", s.rpcClient, err)
		return false, err
	}

	return true, nil
}
