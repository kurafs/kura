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
	"sync"

	"github.com/kurafs/kura/pkg/log"
	spb "github.com/kurafs/kura/pkg/pb/storage"
	"golang.org/x/net/context"
)

type Store interface {
	Read(key string) ([]byte, error)
	Write(key string, val []byte) error
	Has(key string) bool
	Erase(key string) error
	Keys() []string
}

type storageServer struct {
	mu     sync.RWMutex
	store  Store
	logger *log.Logger
}

var _ spb.StorageServiceServer = &storageServer{}

func newStorageServer(logger *log.Logger, store Store) *storageServer {
	return &storageServer{
		store:  store,
		logger: logger,
	}
}

func (s *storageServer) GetBlob(ctx context.Context, req *spb.GetBlobRequest) (*spb.GetBlobResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	blob, err := s.store.Read(req.Key)
	if err != nil {
		return nil, err
	}

	return &spb.GetBlobResponse{Data: blob}, nil
}

func (s *storageServer) PutBlob(ctx context.Context, req *spb.PutBlobRequest) (*spb.PutBlobResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.store.Write(req.Key, req.Data); err != nil {
		return nil, err
	}

	return &spb.PutBlobResponse{}, nil
}

func (s *storageServer) DeleteBlob(ctx context.Context, req *spb.DeleteBlobRequest) (*spb.DeleteBlobResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.store.Has(req.Key) {
		return &spb.DeleteBlobResponse{}, errors.New("blob does not exist")
	}

	if err := s.store.Erase(req.Key); err != nil {
		return nil, err
	}

	return &spb.DeleteBlobResponse{}, nil
}

func (s *storageServer) GetBlobKeys(ctx context.Context, req *spb.GetBlobKeysRequest) (*spb.GetBlobKeysResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	keys := make([]string, 0)
	for _, key := range s.store.Keys() {
		keys = append(keys, key)
	}

	return &spb.GetBlobKeysResponse{Keys: keys}, nil
}
