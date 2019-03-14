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
	"errors"
	"io"
	"math/rand"
	"sync"

	"github.com/kurafs/kura/pkg/log"
	spb "github.com/kurafs/kura/pkg/pb/storage"
	"github.com/kurafs/kura/pkg/streaming"
	"golang.org/x/net/context"
)

type Store interface {
	Read(key string) ([]byte, error)
	Write(key string, val []byte) error
	Has(key string) bool
	Erase(key string) error
	Keys() []string
	Rename(oldKey, newKey string) error
}

type storageServer struct {
	mu     sync.RWMutex
	stores []Store
	logger *log.Logger
}

var _ spb.StorageServiceServer = &storageServer{}

func newStorageServer(logger *log.Logger, stores []Store) *storageServer {
	return &storageServer{
		stores: stores,
		logger: logger,
	}
}

func (s *storageServer) GetBlob(ctx context.Context, req *spb.GetBlobRequest) (*spb.GetBlobResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	index := rand.Intn(len(s.stores))
	blob, err := s.stores[index].Read(req.Key)
	if err != nil {
		return nil, err
	}

	return &spb.GetBlobResponse{Data: blob}, nil
}

func (s *storageServer) GetBlobStream(req *spb.GetBlobStreamRequest, stream spb.StorageService_GetBlobStreamServer) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// TODO(arhamahmed): stream storage reads
	index := rand.Intn(len(s.stores))
	blob, err := s.stores[index].Read(req.Key)
	if err != nil {
		return err
	}

	chunker := streaming.NewChunker(blob)
	for chunker.Next() {
		if err := stream.Send(&spb.GetBlobStreamResponse{Chunk: chunker.Value()}); err != nil {
			return err
		}
	}

	return nil
}

func (s *storageServer) PutBlob(ctx context.Context, req *spb.PutBlobRequest) (*spb.PutBlobResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	didErr := false
	for _, store := range s.stores {
		if err := store.Write(req.Key, req.Data); err != nil {
			didErr = true
			break
		}
	}

	if didErr {
		for _, store := range s.stores {
			if store.Has(req.Key) {
				if err := store.Erase(req.Key); err != nil {
					return nil, errors.New("Failed writing to all storage sinks")
				}
			}
		}
	}

	return &spb.PutBlobResponse{}, nil
}

func (s *storageServer) PutBlobStream(stream spb.StorageService_PutBlobStreamServer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	in, err := stream.Recv()
	if err != nil {
		return err
	}
	// First message determines the key, it will be assumed that all subsequent
	// file keys are the same
	key := in.Key
	fileBytes := in.Chunk

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		fileBytes = append(fileBytes, in.Chunk...)
	}

	// TODO(arhamahmed): call stream writes to storage
	didErr := false
	for _, store := range s.stores {
		if err := store.Write(key, fileBytes); err != nil {
			didErr = true
			break
		}
	}

	if didErr {
		for _, store := range s.stores {
			if store.Has(key) {
				if err := store.Erase(key); err != nil {
					return errors.New("Failed writing to all storage sinks")
				}
			}
		}
	}
	return stream.SendAndClose(&spb.PutBlobStreamResponse{})
}

func (s *storageServer) DeleteBlob(ctx context.Context, req *spb.DeleteBlobRequest) (*spb.DeleteBlobResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, store := range s.stores {
		if !store.Has(req.Key) {
			return &spb.DeleteBlobResponse{}, errors.New("Blob does not exist")
		}

		if err := store.Erase(req.Key); err != nil {
			return nil, err
		}
	}

	return &spb.DeleteBlobResponse{}, nil
}

func (s *storageServer) RenameBlob(ctx context.Context, req *spb.RenameBlobRequest) (*spb.RenameBlobResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, store := range s.stores {
		if err := store.Rename(req.OldKey, req.NewKey); err != nil {
			return nil, err
		}
	}

	return &spb.RenameBlobResponse{}, nil
}

func (s *storageServer) GetBlobKeys(ctx context.Context, req *spb.GetBlobKeysRequest) (*spb.GetBlobKeysResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index := rand.Intn(len(s.stores))
	keys := s.stores[index].Keys()

	return &spb.GetBlobKeysResponse{Keys: keys}, nil
}
