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
	"strings"
	"sync"

	"github.com/kurafs/kura/pkg/diskv"
	"github.com/kurafs/kura/pkg/log"
	pb "github.com/kurafs/kura/pkg/pb/storage"
	"golang.org/x/net/context"
)

type diskStore struct {
	mu     sync.RWMutex
	diskv  *diskv.Diskv
	logger *log.Logger
}

var _ pb.StorageServiceServer = &diskStore{}

func newServer(logger *log.Logger) *diskStore {
	dv := diskv.New(diskv.Options{
		BasePath:     "kura-disk-ss",
		CacheSizeMax: 1024 * 1024, // 1MB cache size
		AdvancedTransform: func(s string) *diskv.PathKey {
			path := strings.Split(s, "/")
			last := len(path) - 1
			return &diskv.PathKey{
				Path:     path[:last],
				FileName: path[last],
			}
		},
		InverseTransform: func(pk *diskv.PathKey) string {
			txt := pk.FileName[len(pk.FileName)-4:]
			if txt != ".txt" {
				panic("Invalid file found in storage folder!")
			}
			return strings.Join(pk.Path, "/") + pk.FileName
		},
	})
	return &diskStore{
		diskv:  dv,
		logger: logger,
	}
}

func (d *diskStore) GetFile(ctx context.Context, req *pb.GetFileRequest) (*pb.GetFileResponse, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	file, err := d.diskv.Read(req.Key)
	if err != nil {
		return nil, err
	}

	d.logger.Infof("Read %d bytes from file %s\n", len(file), req.Key)
	return &pb.GetFileResponse{File: file}, nil
}

func (d *diskStore) PutFile(ctx context.Context, req *pb.PutFileRequest) (*pb.PutFileResponse, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.diskv.Write(req.Key, req.File); err != nil {
		return nil, err
	}

	return &pb.PutFileResponse{}, nil
}

func (d *diskStore) DeleteFile(ctx context.Context, req *pb.DeleteFileRequest) (*pb.DeleteFileResponse, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.diskv.Has(req.Key) {
		return &pb.DeleteFileResponse{}, errors.New("File does not exist")
	}

	if err := d.diskv.Erase(req.Key); err != nil {
		return nil, err
	}

	d.logger.Info("Deleted %s", req.Key)
	return &pb.DeleteFileResponse{}, nil
}
