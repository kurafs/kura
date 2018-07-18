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
	"errors"
	"time"

	"github.com/kurafs/kura/pkg/log"
	mpb "github.com/kurafs/kura/pkg/pb/metadata"
	spb "github.com/kurafs/kura/pkg/pb/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	storageAddress string
	logger         *log.Logger
}

func NewServer(storageAddress string, logger *log.Logger) *Server {
	return &Server{storageAddress, logger}
}

func (s *Server) getNewStorage(conn *grpc.ClientConn) *StorageClient {
	rpcClient := spb.NewStorageServiceClient(conn)
	return NewStorageClient(rpcClient, s.logger)
}

func (s *Server) GetDirectoryKeys(ctx context.Context, req *mpb.GetDirectoryKeysRequest) (*mpb.GetDirectoryKeysResponse, error) {
	conn, err := grpc.Dial(s.storageAddress, grpc.WithInsecure())
	if err != nil {
		s.logger.Errorf("%v\n", err)
		return nil, err
	}
	defer conn.Close()
	client := s.getNewStorage(conn)

	metadata, err := client.GetMetadataFile()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(metadata.Entries))
	for k := range metadata.Entries {
		keys = append(keys, k)
	}

	return &mpb.GetDirectoryKeysResponse{Keys: keys}, nil
}

func (s *Server) GetMetadata(ctx context.Context, req *mpb.GetMetadataRequest) (*mpb.GetMetadataResponse, error) {
	conn, err := grpc.Dial(s.storageAddress, grpc.WithInsecure())
	if err != nil {
		s.logger.Errorf("%v\n", err)
		return nil, err
	}
	defer conn.Close()
	client := s.getNewStorage(conn)

	metadata, err := client.GetMetadataFile()
	if err != nil {
		return nil, err
	}
	m := metadata.Entries[req.Key]
	return &mpb.GetMetadataResponse{Metadata: &m}, nil
}

func (s *Server) SetMetadata(ctx context.Context, req *mpb.SetMetadataRequest) (*mpb.SetMetadataResponse, error) {
	conn, err := grpc.Dial(s.storageAddress, grpc.WithInsecure())
	if err != nil {
		s.logger.Errorf("%v\n", err)
		return nil, err
	}
	defer conn.Close()
	client := s.getNewStorage(conn)

	metadata, err := client.GetMetadataFile()
	if err != nil {
		return nil, err
	}
	metadata.Entries[req.Key] = *req.Metadata
	_, err = client.PutMetadataFile(metadata)
	if err != nil {
		return nil, err
	}

	return &mpb.SetMetadataResponse{}, nil
}

func (s *Server) GetFile(ctx context.Context, req *mpb.GetFileRequest) (*mpb.GetFileResponse, error) {
	conn, err := grpc.Dial(s.storageAddress, grpc.WithInsecure())
	if err != nil {
		s.logger.Errorf("%v\n", err)
		return nil, err
	}
	defer conn.Close()
	client := s.getNewStorage(conn)

	file, err := client.GetFile(req.Key)
	if err != nil {
		return nil, err
	}

	return &mpb.GetFileResponse{File: file}, nil
}

func (s *Server) PutFile(ctx context.Context, req *mpb.PutFileRequest) (*mpb.PutFileResponse, error) {
	fileBytes := req.File
	key := req.Key

	if key == "" {
		st := status.New(codes.InvalidArgument, "Empty Key String")
		return nil, st.Err()
	}

	conn, err := grpc.Dial(s.storageAddress, grpc.WithInsecure())
	if err != nil {
		s.logger.Errorf("%v\n", err)
		return nil, err
	}
	defer conn.Close()
	client := s.getNewStorage(conn)

	putServer, err := client.PutFile(key, fileBytes)
	if err != nil {
		s.logger.Errorf("%v\n", err)
		return nil, err
	}

	if !putServer {
		err := errors.New("Unable to save file")
		s.logger.Errorf("%v\n", err)
		return nil, err
	}

	metadata, err := client.GetMetadataFile()
	if err != nil {
		return nil, err
	}

	if val, ok := metadata.Entries[key]; ok {
		metadata.Entries[key] = mpb.FileMetadata{
			Created:      val.Created,
			LastModified: &mpb.FileMetadata_UnixTimestamp{Seconds: time.Now().Unix()},
			Permissions:  val.Permissions,
			Size:         val.Size,
		}
	} else {
		// We might actually need the storage time when it was written but IDK
		ts := time.Now().Unix()
		metadata.Entries[key] = mpb.FileMetadata{
			Created:      &mpb.FileMetadata_UnixTimestamp{Seconds: ts},
			LastModified: &mpb.FileMetadata_UnixTimestamp{Seconds: ts},
			Permissions:  "default",
			Size:         int64(len(fileBytes)),
		}
	}

	_, err = client.PutMetadataFile(metadata)
	if err != nil {
		s.logger.Errorf("%v\n", err)
		return nil, err
	}

	return &mpb.PutFileResponse{}, nil
}

func (s *Server) DeleteFile(ctx context.Context, req *mpb.DeleteFileRequest) (*mpb.DeleteFileResponse, error) {
	conn, err := grpc.Dial(s.storageAddress, grpc.WithInsecure())
	if err != nil {
		s.logger.Errorf("%v\n", err)
		return nil, err
	}
	defer conn.Close()
	client := s.getNewStorage(conn)

	_, err = client.DeleteFile(req.Key)
	if err != nil {
		return nil, err
	}

	metadata, err := client.GetMetadataFile()
	if err != nil {
		return nil, err
	}

	if _, ok := metadata.Entries[req.Key]; !ok {
		st := status.New(codes.Internal, "Tried to delete a metadata entry that doesn't exist")
		return nil, st.Err()
	}
	delete(metadata.Entries, req.Key)

	if _, err := client.PutMetadataFile(metadata); err != nil {
		return nil, err
	}

	// Do something to sync metadata file with storage if meadata sync fails

	return &mpb.DeleteFileResponse{}, nil
}
