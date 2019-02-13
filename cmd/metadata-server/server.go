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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kurafs/kura/pkg/log"
	mpb "github.com/kurafs/kura/pkg/pb/metadata"
	spb "github.com/kurafs/kura/pkg/pb/storage"
)

type Server struct {
	logger        *log.Logger
	storageClient spb.StorageServiceClient
	mu            sync.RWMutex
}

type MetadataFile struct {
	Entries map[string]mpb.FileMetadata
}

const metadataFileKey = "kura-metadata"

func newMetadataServer(logger *log.Logger, storageClient spb.StorageServiceClient) *Server {
	return &Server{
		logger:        logger,
		storageClient: storageClient,
	}
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
			Permissions:  0644,
			Size:         int64(len(req.File)),
		}
	}

	if err := s.setMetadataFile(ctx, metadata); err != nil {
		return nil, err
	}

	return &mpb.PutFileResponse{}, nil
}

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

	delete(metadata.Entries, req.Key)

	if err := s.setMetadataFile(ctx, metadata); err != nil {
		return nil, err
	}

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

	sort.Strings(keys)
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

func (s *Server) runGarbageCollection(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metadata, err := s.getMetadataFile(ctx)
	if err != nil {
		return nil
	}

	keyMap := make(map[string]bool)
	for k := range metadata.Entries {
		keyMap[k] = true
	}

	fileKeysReq := &spb.GetFileKeysRequest{}
	fileKeysRes, err := s.storageClient.GetFileKeys(ctx, fileKeysReq)
	if err != nil {
		return err
	}

<<<<<<< HEAD
	for _, key := range fileKeysRes.Keys {
		if !keyMap[key] {
			deleteReq := &mpb.DeleteFileRequest{Key: key}
			if _, err := s.storageClient.DeleteFile(ctx, &spb.DeleteFileRequest{Key: deleteReq.Key}); err != nil {
				return err
			}
=======
	for key := range fileKeysRes.Keys {
		actualKey := fileKeysRes.Keys[key]
		if !keyMap[actualKey] {
			deleteReq := &mpb.DeleteFileRequest{Key: actualKey}
			if _, err := s.storageClient.DeleteFile(ctx, &spb.DeleteFileRequest{Key: deleteReq.Key}); err != nil {
				return err
			}

			metadata, err := s.getMetadataFile(ctx)
			if err != nil {
				return err
			}

			delete(metadata.Entries, deleteReq.Key)

			if err := s.setMetadataFile(ctx, metadata); err != nil {
				return err
			}
>>>>>>> Enable GC on metadata server which synchronizes storage with it
		}
	}

	return nil
}

<<<<<<< HEAD
// Used for testing purposes only.
func TestForceGC(s *Server) error {
	return s.runGarbageCollection(context.Background())
=======
func (s *Server) ForceGarbageCollection(ctx context.Context, req *mpb.ForceGarbageCollectionRequest) (*mpb.ForceGarbageCollectionResponse, error) {
	err := s.runGarbageCollection(ctx)
	if err != nil {
		return nil, err
	}

	return &mpb.ForceGarbageCollectionResponse{}, nil
>>>>>>> Enable GC on metadata server which synchronizes storage with it
}
