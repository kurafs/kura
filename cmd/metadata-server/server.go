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
	"io"
	"sort"
	"strings"
	"sync"

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

	metadata, err := s.getMetadataFile(ctx)
	if err != nil {
		return nil, err
	}

	entry := metadata.Entries[req.Key]
	return &mpb.GetFileResponse{File: resp.File, Metadata: &entry}, nil
}

type chunkMessage struct {
	chunk []byte
	err   error
}

func (s *Server) GetFileStream(req *mpb.GetFileStreamRequest, stream mpb.MetadataService_GetFileStreamServer) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	storeStream, err := s.storageClient.GetFileStream(stream.Context(), &spb.GetFileStreamRequest{Key: req.Key})
	ch := make(chan *chunkMessage)
	go func() {
		if err != nil {
			ch <- &chunkMessage{err: err}
			close(ch)
			return
		}
		for {
			resp, err := storeStream.Recv()
			if err == io.EOF {
				close(ch)
				return
			}
			if err != nil {
				ch <- &chunkMessage{err: err}
				close(ch)
				return
			}
			ch <- &chunkMessage{chunk: resp.FileChunk}
		}
	}()

	for cm := range ch {
		if cm.err != nil {
			return cm.err
		}
		if err := stream.Send(&mpb.GetFileStreamResponse{FileChunk: cm.chunk}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) PutFile(ctx context.Context, req *mpb.PutFileRequest) (*mpb.PutFileResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.storageClient.PutFile(ctx, &spb.PutFileRequest{Key: req.Key, File: req.File}); err != nil {
		return nil, err
	}

	metadata, err := s.getMetadataFile(ctx)
	if err != nil {
		return nil, err
	}

	entry := req.Metadata
	if entry == nil {
		return nil, errors.New("empty metadata")
	}
	metadata.Entries[req.Key] = *entry
	if err := s.setMetadataFile(ctx, metadata); err != nil {
		return nil, err
	}

	return &mpb.PutFileResponse{}, nil
}

func (s *Server) PutFileStream(stream mpb.MetadataService_PutFileStreamServer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx := stream.Context()
	storeStream, err := s.storageClient.PutFileStream(ctx)
	if err != nil {
		return err
	}

	in, err := stream.Recv()
	if err != nil {
		return err
	}
	// First message determines the key and metadata, it will be assumed that all subsequent
	// file keys are the same
	key := in.Key
	if err = storeStream.Send(&spb.PutFileStreamRequest{Key: key, FileChunk: in.FileChunk}); err != nil {
		return err
	}
	entry := in.Metadata
	if entry == nil {
		return errors.New("empty metadata")
	}
	fileSize := len(in.FileChunk)

	for {
		in, err = stream.Recv()
		if err == io.EOF {
			if _, err = storeStream.CloseAndRecv(); err != nil {
				return err
			}
			break
		}
		if err != nil {
			return err
		}
		if err = storeStream.Send(&spb.PutFileStreamRequest{Key: key, FileChunk: in.FileChunk}); err != nil {
			return err
		}
		fileSize += len(in.FileChunk)
	}

	metadata, err := s.getMetadataFile(ctx)
	if err != nil {
		return err
	}

	metadata.Entries[key] = *entry
	if err := s.setMetadataFile(ctx, metadata); err != nil {
		return err
	}

	return stream.SendAndClose(&mpb.PutFileStreamResponse{})
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

	for _, key := range fileKeysRes.Keys {
		if !keyMap[key] {
			deleteReq := &mpb.DeleteFileRequest{Key: key}
			if _, err := s.storageClient.DeleteFile(ctx,
				&spb.DeleteFileRequest{Key: deleteReq.Key}); err != nil {
				return err
			}
		}
	}

	return nil
}
