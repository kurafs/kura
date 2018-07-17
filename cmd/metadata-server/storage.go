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
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"golang.org/x/net/context"

	"github.com/kurafs/kura/pkg/log"
	mpb "github.com/kurafs/kura/pkg/rpc/metadata"
	spb "github.com/kurafs/kura/pkg/rpc/storage"
)

type MetadataFile struct {
	Entries map[string]mpb.FileMetadata
}

const chunkSize = 64 * 1024

const (
	colDelimiter   = byte('\x1e')
	entryDelimiter = byte('\x1f')
)

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
	fmt.Println(fileBytes)
	entryBytes := bytes.Split(fileBytes, []byte{entryDelimiter})
	entries := make(map[string]mpb.FileMetadata)
	for _, entry := range entryBytes {
		// TODO: More expressive errors when parsing
		cols := bytes.Split(entry, []byte{colDelimiter})
		fmt.Println(cols)
		if len(cols) != 5 {
			return nil, errors.New("Not enough columns")
		}
		key := string(cols[Key])
		fmt.Println(key)
		// Make these just uint64s for now
		created, err := strconv.ParseInt(string(cols[Created]), 10, 64)
		if err != nil {
			return nil, errors.New("Couldn't parse created")
		}
		fmt.Println(created)
		lastModified, err := strconv.ParseInt(string(cols[LastModified]), 10, 64)
		if err != nil {
			return nil, errors.New("Couldn't parse lastModified")
		}
		fmt.Println(lastModified)
		permissions := string(cols[Permissions])
		fmt.Println(permissions)
		size, err := strconv.ParseInt(string(cols[Size]), 10, 64)
		if err != nil {
			return nil, errors.New("Couldn't parse size")
		}
		fmt.Println(size)
		entries[key] = mpb.FileMetadata{
			Created:      &mpb.FileMetadata_UnixTimestamp{Seconds: created},
			LastModified: &mpb.FileMetadata_UnixTimestamp{Seconds: lastModified},
			Permissions:  permissions,
			Size:         size,
		}
	}
	return &MetadataFile{Entries: entries}, nil
}

func MetadataFileToBytes(metadata *MetadataFile) []byte {
	// TODO: Move this with the other constant into a file in RPC
	fileBytes := make([]byte, 0, chunkSize)
	for k, v := range metadata.Entries {
		// TODO: We probably want to encode the integers in a more efficient format
		fileBytes = append(fileBytes, []byte(k)...)
		fileBytes = append(fileBytes, colDelimiter)
		fileBytes = append(fileBytes, []byte(strconv.FormatInt(v.Created.Seconds, 10))...)
		fileBytes = append(fileBytes, colDelimiter)
		fileBytes = append(fileBytes, []byte(strconv.FormatInt(v.LastModified.Seconds, 10))...)
		fileBytes = append(fileBytes, colDelimiter)
		fileBytes = append(fileBytes, []byte(v.Permissions)...)
		fileBytes = append(fileBytes, colDelimiter)
		fileBytes = append(fileBytes, []byte(strconv.FormatInt(v.Size, 10))...)
		fileBytes = append(fileBytes, entryDelimiter)
	}
	fmt.Println(fileBytes)
	return fileBytes
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
	stream, err := s.rpcClient.GetFile(ctx, &spb.GetFileRequest{Key: key})
	if err != nil {
		s.logger.Errorf("%v.GetFile(_) = _, %v\n", s.rpcClient, err)
		return nil, err
	}
	fileBytes := make([]byte, 0, chunkSize)
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			return fileBytes, nil
		}
		if err != nil {
			s.logger.Errorf("%v.GetFile(_) = _, %v\n", s.rpcClient, err)
		}
		fileBytes = append(fileBytes, resp.FileChunk...)
	}
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

// TODO: Sometimes putting changes the structure of the metadata file... which causes it to parse incorrectly
func (s *StorageClient) PutFile(key string, file []byte) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	stream, err := s.rpcClient.PutFile(ctx)
	if err != nil {
		s.logger.Errorf("%v.PutFile(_) = _, %v\n", s.rpcClient, err)
		return false, err
	}
	numChunks := len(file) / chunkSize
	if len(file)%chunkSize != 0 {
		numChunks++
	}
	for i := 0; i < numChunks; i++ {
		b := i * chunkSize
		e := (i + 1) * chunkSize
		if e >= len(file) {
			e = len(file) - 1
		}

		if err := stream.Send(&spb.PutFileRequest{Key: key, FileChunk: file[b:e]}); err != nil {
			return false, err
		}
	}
	reply, err := stream.CloseAndRecv()
	if err != nil {
		s.logger.Errorf("%v.PutFile(_) = _, %v\n", s.rpcClient, err)
		return false, err
	}

	return reply.Successful, nil
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
	resp, err := s.rpcClient.DeleteFile(ctx, &spb.DeleteFileRequest{Key: key})
	if err != nil {
		s.logger.Errorf("%v.DeleteFile(_) = _, %v\n", s.rpcClient, err)
		return false, err
	}
	return resp.Successful, nil
}
