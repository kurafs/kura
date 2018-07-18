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
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/kurafs/kura/pkg/log"
	mpb "github.com/kurafs/kura/pkg/pb/metadata"
	spb "github.com/kurafs/kura/pkg/pb/storage"
	"google.golang.org/grpc"
)

type Server struct {
	logger        *log.Logger
	storageClient spb.StorageServiceClient
	conn          *grpc.ClientConn
	mu            sync.RWMutex
}

type MetadataFile struct {
	Entries map[string]mpb.FileMetadata
}

const metadataFileKey = "kura-metadata"

func newMetaDataServer(logger *log.Logger, storageAddr string) (*Server, error) {
	conn, err := grpc.Dial(storageAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	client := spb.NewStorageServiceClient(conn)
	server := Server{
		logger:        logger,
		conn:          conn,
		storageClient: client,
	}

	return &server, nil
}

func (s *Server) GetFile(ctx context.Context, req *mpb.GetFileRequest) (*mpb.GetFileResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resp, err := s.storageClient.GetFile(ctx, &spb.GetFileRequest{Key: req.Key})
	if err != nil {
		return nil, err
	}

	return &mpb.GetFileResponse{File: resp.File}, nil
}

// TODO(irfansharif): This should be transactional, we write to two files and
// they should happen atomically.
func (s *Server) PutFile(ctx context.Context, req *mpb.PutFileRequest) (*mpb.PutFileResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Key == "" {
		return nil, errors.New("Empty file name")
	}

	if _, err := s.storageClient.PutFile(ctx, &spb.PutFileRequest{Key: req.Key, File: req.File}); err != nil {
		return nil, err
	}

	metadata, err := s.getMetadataFile(ctx)
	if err != nil {
		return nil, err
	}

	if val, ok := metadata.Entries[req.Key]; ok {
		metadata.Entries[req.Key] = mpb.FileMetadata{
			Created:      val.Created,
			LastModified: &mpb.FileMetadata_UnixTimestamp{Seconds: time.Now().Unix()},
			Permissions:  val.Permissions,
			Size:         int64(len(req.File)),
		}
	} else {
		// TODO(irfansharif): The PutFile RPC should accept associated metadata
		// as well.
		ts := time.Now().Unix()
		metadata.Entries[req.Key] = mpb.FileMetadata{
			Created:      &mpb.FileMetadata_UnixTimestamp{Seconds: ts},
			LastModified: &mpb.FileMetadata_UnixTimestamp{Seconds: ts},
			Permissions:  "default",
			Size:         int64(len(req.File)),
		}
	}

	if err := s.setMetadataFile(ctx, metadata); err != nil {
		return nil, err
	}

	return &mpb.PutFileResponse{}, nil
}

// TODO(irfansharif): This should be transactional, we write to two files and
// they should happen atomically.
func (s *Server) DeleteFile(ctx context.Context, req *mpb.DeleteFileRequest) (*mpb.DeleteFileResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.storageClient.DeleteFile(ctx, &spb.DeleteFileRequest{Key: req.Key}); err != nil {
		return nil, err
	}

	metadata, err := s.getMetadataFile(ctx)
	if err != nil {
		return nil, err
	}

	if _, ok := metadata.Entries[req.Key]; !ok {
		return nil, errors.New("tried to delete a metadata entry that doesn't exist")
	}
	delete(metadata.Entries, req.Key)

	if err := s.setMetadataFile(ctx, metadata); err != nil {
		return nil, err
	}
	// TODO(Franz): Do something to sync metadata file with storage if metadata
	// sync fails.
	return &mpb.DeleteFileResponse{}, nil
}

func (s *Server) GetMetadata(ctx context.Context, req *mpb.GetMetadataRequest) (*mpb.GetMetadataResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metadata, err := s.getMetadataFile(ctx)
	if err != nil {
		return nil, err
	}

	value := metadata.Entries[req.Key]
	return &mpb.GetMetadataResponse{Metadata: &value}, nil
}

func (s *Server) SetMetadata(ctx context.Context, req *mpb.SetMetadataRequest) (*mpb.SetMetadataResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	metadata, err := s.getMetadataFile(ctx)
	if err != nil {
		return nil, err
	}
	metadata.Entries[req.Key] = *req.Metadata

	if err := s.setMetadataFile(ctx, metadata); err != nil {
		return nil, err
	}

	return &mpb.SetMetadataResponse{}, nil
}

func (s *Server) GetDirectoryKeys(ctx context.Context, req *mpb.GetDirectoryKeysRequest) (*mpb.GetDirectoryKeysResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metadata, err := s.getMetadataFile(ctx)
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(metadata.Entries))
	for k := range metadata.Entries {
		keys = append(keys, k)
	}

	return &mpb.GetDirectoryKeysResponse{Keys: keys}, nil
}

func (s *Server) getMetadataFile(ctx context.Context) (*MetadataFile, error) {
	var metadata MetadataFile

	resp, err := s.storageClient.GetFile(ctx, &spb.GetFileRequest{Key: metadataFileKey})
	if err != nil {
		if strings.HasSuffix(err.Error(), "no such file or directory") {
			// We deal with the case where the storage server is a fresh one,
			// therefore one without the metadata file.
			//
			// TODO: Explicitly initialize storage server when connecting?
			metadata = MetadataFile{
				Entries: make(map[string]mpb.FileMetadata),
			}
			return &metadata, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(resp.File, &metadata); err != nil {
		return nil, errors.New("Unable to deserialize metadata file")
	}

	return &metadata, nil
}

func (s *Server) setMetadataFile(ctx context.Context, metadata *MetadataFile) error {
	reserialized, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	if _, err := s.storageClient.PutFile(ctx, &spb.PutFileRequest{Key: metadataFileKey, File: reserialized}); err != nil {
		return err
	}

	return nil
}
