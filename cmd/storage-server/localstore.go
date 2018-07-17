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
	"fmt"
	"io"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/kurafs/kura/pkg/diskv"
	"github.com/kurafs/kura/pkg/log"
	pb "github.com/kurafs/kura/pkg/rpc/storage"
	"golang.org/x/net/context"
)

// Apparently this is an optimal chunk size according to https://github.com/grpc/grpc.github.io/issues/371
const chunkSize = 64 * 1024

type localServer struct {
	diskv  *diskv.Diskv
	logger *log.Logger
}

var _ pb.StorageServiceServer = &localServer{}

func newServer(logger *log.Logger) *localServer {
	dv := diskv.New(diskv.Options{
		BasePath:     "kura",
		CacheSizeMax: 1024 * 1024, // 1MB cache size
		AdvancedTransform: func(s string) *diskv.PathKey {
			fmt.Println(s)
			split := strings.Split(s, "/")
			if len(split) == 1 {
				return &diskv.PathKey{
					Path:     []string{},
					FileName: s,
				}
			}
			fmt.Println(split)
			return &diskv.PathKey{
				Path:     split[:len(split)-1],
				FileName: split[len(split)-1],
			}
		},
		InverseTransform: func(pk *diskv.PathKey) string {
			if len(pk.Path) == 0 {
				return pk.FileName
			}
			return strings.Join(pk.Path, "/") + "/" + pk.FileName
		},
	})
	return &localServer{dv, logger}
}

func (l *localServer) GetFile(getReq *pb.GetFileRequest, stream pb.StorageService_GetFileServer) error {
	// We will probably need to stream the data off disk for bigger files
	file, err := l.diskv.Read(getReq.Key)
	if err != nil {
		return err
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

		if err := stream.Send(&pb.GetFileResponse{FileChunk: file[b:e]}); err != nil {
			return err
		}
	}
	l.logger.Infof("Read %d bytes from file %s\n", len(file), getReq.Key)

	return nil
}

func (l *localServer) PutFile(stream pb.StorageService_PutFileServer) error {
	in, err := stream.Recv()
	if err == io.EOF {
		// TODO (franz): IDK what the expected behaviour here should be
		return stream.SendAndClose(&pb.PutFileResponse{
			Successful: false,
		})
	}
	if err != nil {
		return err
	}
	fileBytes := in.FileChunk
	key := in.Key
	if key == "" {
		st := status.New(codes.InvalidArgument, "Empty Key String")
		return st.Err()
	}
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			l.logger.Infof("Writing %d bytes to %s\n", len(fileBytes), key)
			if err = l.diskv.Write(key, fileBytes); err != nil {
				l.logger.Error(err)
				st := status.New(codes.Internal, "Could not write to disk")
				return st.Err()
			}
			return stream.SendAndClose(&pb.PutFileResponse{
				Successful: true,
			})
		}
		if err != nil {
			l.logger.Error(err)
			return err
		}
		if in.Key != key {
			st := status.New(codes.Internal, "Key string changed while writing file")
			return st.Err()
		}
		l.logger.Infof("Got %d bytes from stream\n", len(in.FileChunk))
		fileBytes = append(fileBytes, in.FileChunk...)
	}
}

func (l *localServer) DeleteFile(ctx context.Context, delReq *pb.DeleteFileRequest) (*pb.DeleteFileResponse, error) {
	// Maybe we wanna throw an error here instead similar to how S3 behaves when you try to access a key you
	// don't have access to so that no information is leaked
	if !l.diskv.Has(delReq.Key) {
		return &pb.DeleteFileResponse{
			Successful: false,
		}, nil
	}
	if err := l.diskv.Erase(delReq.Key); err != nil {
		st := status.New(codes.Internal, "Unable to erase file from disk")
		return nil, st.Err()
	}
	l.logger.Info("Deleted %s", delReq.Key)
	return &pb.DeleteFileResponse{
		Successful: true,
	}, nil
}
