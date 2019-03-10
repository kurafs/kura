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
	"fmt"
	"io"
	"path"
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
	Entries map[string]mpb.Metadata
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

	resp, err := s.storageClient.GetBlob(ctx, &spb.GetBlobRequest{Key: req.Path})
	if err != nil {
		return nil, err
	}

	metadata, err := s.getMetadataFile(ctx)
	if err != nil {
		return nil, err
	}

	entry := metadata.Entries[req.Path]
	return &mpb.GetFileResponse{File: resp.Data, Metadata: &entry}, nil
}

func (s *Server) GetFileStream(req *mpb.GetFileStreamRequest, stream mpb.MetadataService_GetFileStreamServer) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	storeStream, err := s.storageClient.GetBlobStream(stream.Context(), &spb.GetBlobStreamRequest{Key: req.Path})
	if err != nil {
		return err
	}

	metadata, err := s.getMetadataFile(stream.Context())
	if err != nil {
		return err
	}
	entry := metadata.Entries[req.Path]
	first, err := storeStream.Recv()
	if err = stream.Send(&mpb.GetFileStreamResponse{Chunk: first.Chunk, Metadata: &entry}); err != nil {
		return err
	}

	for {
		resp, err := storeStream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err = stream.Send(&mpb.GetFileStreamResponse{Chunk: resp.Chunk}); err != nil {
			return err
		}
	}
}

func (s *Server) PutFile(ctx context.Context, req *mpb.PutFileRequest) (*mpb.PutFileResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Path == "" {
		return nil, errors.New("Empty file name")
	}

	if _, err := s.storageClient.PutBlob(ctx, &spb.PutBlobRequest{Key: req.Path, Data: req.File}); err != nil {
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
	metadata.Entries[req.Path] = *entry
	if err := s.setMetadataFile(ctx, metadata); err != nil {
		return nil, err
	}

	return &mpb.PutFileResponse{}, nil
}

func (s *Server) PutFileStream(stream mpb.MetadataService_PutFileStreamServer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx := stream.Context()
	storeStream, err := s.storageClient.PutBlobStream(ctx)
	if err != nil {
		return err
	}

	in, err := stream.Recv()
	if err != nil {
		return err
	}
	// First message determines the key and metadata, it will be assumed that all subsequent
	// file keys are the same
	key := in.Path
	if err = storeStream.Send(&spb.PutBlobStreamRequest{Key: key, Chunk: in.Chunk}); err != nil {
		return err
	}
	entry := in.Metadata
	if entry == nil {
		return errors.New("empty metadata")
	}

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
		if err = storeStream.Send(&spb.PutBlobStreamRequest{Key: key, Chunk: in.Chunk}); err != nil {
			return err
		}
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

	if _, err := s.storageClient.DeleteBlob(ctx, &spb.DeleteBlobRequest{Key: req.Path}); err != nil {
		return nil, err
	}

	metadata, err := s.getMetadataFile(ctx)
	if err != nil {
		return nil, err
	}

	delete(metadata.Entries, req.Path)

	if err := s.setMetadataFile(ctx, metadata); err != nil {
		return nil, err
	}

	return &mpb.DeleteFileResponse{}, nil
}

func (s *Server) CreateDirectory(ctx context.Context, req *mpb.CreateDirectoryRequest) (*mpb.CreateDirectoryResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Path == "" {
		return nil, errors.New("dir name required")
	}

	if !req.Metadata.IsDirectory {
		return nil, errors.New("malformed metadata, IsDirectory is false despite being a dir")
	}

	metadata, err := s.getMetadataFile(ctx)
	if err != nil {
		return nil, err
	}

	metadata.Entries[req.Path] = *req.Metadata
	if err := s.setMetadataFile(ctx, metadata); err != nil {
		return nil, err
	}

	return &mpb.CreateDirectoryResponse{}, nil
}

func (s *Server) DeleteDirectory(ctx context.Context, req *mpb.DeleteDirectoryRequest) (*mpb.DeleteDirectoryResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	metadata, err := s.getMetadataFile(ctx)
	if err != nil {
		return nil, err
	}

	for path, entry := range metadata.Entries {
		if !strings.HasPrefix(path, req.Path+"/") {
			continue
		}

		delete(metadata.Entries, path)
		if entry.IsDirectory {
			continue
		}

		if _, err := s.storageClient.DeleteBlob(ctx, &spb.DeleteBlobRequest{Key: path}); err != nil {
			return nil, err
		}
	}

	delete(metadata.Entries, req.Path)
	if err := s.setMetadataFile(ctx, metadata); err != nil {
		return nil, err
	}

	return &mpb.DeleteDirectoryResponse{}, nil
}

func (s *Server) GetDirectoryEntries(ctx context.Context, req *mpb.GetDirectoryEntriesRequest) (*mpb.GetDirectoryEntriesResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metadata, err := s.getMetadataFile(ctx)
	if err != nil {
		return nil, err
	}

	entries := []*mpb.GetDirectoryEntriesResponse_DirectoryEntry{}
	for fpath, entry := range metadata.Entries {
		if !strings.HasPrefix(fpath, req.Path+"/") {
			continue
		}

		suffix := fpath[len(req.Path+"/"):]
		if strings.ContainsAny(suffix, "/") {
			continue
		}

		entry := mpb.GetDirectoryEntriesResponse_DirectoryEntry{
			Path:        path.Join(req.Path, suffix),
			IsDirectory: entry.IsDirectory,
		}
		entries = append(entries, &entry)
	}

	return &mpb.GetDirectoryEntriesResponse{Entries: entries}, nil
}

func (s *Server) Rename(ctx context.Context, req *mpb.RenameRequest) (*mpb.RenameResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	metadata, err := s.getMetadataFile(ctx)
	if err != nil {
		return nil, err
	}

	oentry, _ := metadata.Entries[req.OldPath]
	nentry, nexists := metadata.Entries[req.NewPath]

	// Cases:
	// OldPath      NewPathExists       NewPath     Result
	// -------------------------------------------------------
	// File         No                  N/A         Rename
	// File         Yes                 File        Overwrite
	// File         Yes                 Dir         Error!
	// Dir          No                  N/A         Rename
	// Dir          Yes                 File        Error!
	// Dir          Yes                 Dir         Overwrite if empty, else Error!

	// Cases for 'Error!'

	// Take care of renaming a directory into a file, or vice versa.
	if nexists && (oentry.IsDirectory != nentry.IsDirectory) {
		return nil, fmt.Errorf("can't rename dir to file or vice-versa")
	}

	// Take care of attempt to copy directory into itself.
	if nexists && nentry.IsDirectory && oentry.IsDirectory && req.OldPath == req.NewPath {
		return nil, fmt.Errorf("can't move directory into itself")
	}

	// Say we have the following tree structure, a and a' are the same name but
	// are distinguished to be able to talk about them.
	//
	// ├── a
	// └── b
	//     └── a'
	//        [...]
	//
	// If we try 'mv a b', it should fail if a' is non-empty. If a' is empty, we
	// should override to get the following:
	//
	// └── b
	//     └── a
	if nexists && nentry.IsDirectory && oentry.IsDirectory {
		// We check if a there exists a directory under nentry with the same
		// name as oentry, and if so, if said directory is empty.
		empty := true
		for path := range metadata.Entries {
			if strings.HasPrefix(path, req.NewPath+"/") {
				empty = false
				break
			}
		}
		if !empty {
			return nil, fmt.Errorf("directory not empty")
		}
	}

	// Cases for 'Rename'

	if !nexists {
		if !oentry.IsDirectory {
			// File rename.
			res, err := s.fileRenameLocked(ctx, req.OldPath, req.NewPath, metadata)
			return res, err
		}

		// Directory rename.
		res, err := s.directoryRenameLocked(ctx, req.OldPath, req.NewPath, metadata)
		return res, err
	}

	// Cases for 'Overwrite'

	if !oentry.IsDirectory {
		// File overwrite.
		res, err := s.fileRenameLocked(ctx, req.OldPath, req.NewPath, metadata)
		return res, err
	}

	// Directory overwrite.
	res, err := s.directoryRenameLocked(ctx, req.OldPath, req.NewPath, metadata)
	return res, err
}

func (s *Server) fileRenameLocked(ctx context.Context, oldPath, newPath string, metadata *MetadataFile) (*mpb.RenameResponse, error) {
	req := &spb.RenameBlobRequest{
		OldKey: oldPath,
		NewKey: newPath,
	}

	if _, err := s.storageClient.RenameBlob(ctx, req); err != nil {
		return nil, err
	}

	metadata.Entries[newPath] = metadata.Entries[oldPath]
	delete(metadata.Entries, oldPath)

	if err := s.setMetadataFile(ctx, metadata); err != nil {
		return nil, err
	}

	return &mpb.RenameResponse{}, nil
}

func (s *Server) directoryRenameLocked(ctx context.Context, oldPath, newPath string, metadata *MetadataFile) (*mpb.RenameResponse, error) {
	// First change the name of the directory stored in metadata itself.
	metadata.Entries[newPath] = metadata.Entries[oldPath]
	delete(metadata.Entries, oldPath)

	// Do all the subfiles/dirs.
	for cpath, entry := range metadata.Entries {
		if !strings.HasPrefix(cpath, oldPath+"/") {
			continue
		}

		npath := path.Join(newPath, cpath[len(oldPath+"/"):])
		if !entry.IsDirectory {
			req := &spb.RenameBlobRequest{
				OldKey: cpath,
				NewKey: npath,
			}

			if _, err := s.storageClient.RenameBlob(ctx, req); err != nil {
				return nil, err
			}
		}

		metadata.Entries[npath] = metadata.Entries[cpath]
		delete(metadata.Entries, cpath)
	}

	if err := s.setMetadataFile(ctx, metadata); err != nil {
		return nil, err
	}

	return &mpb.RenameResponse{}, nil
}

func (s *Server) GetMetadata(ctx context.Context, req *mpb.GetMetadataRequest) (*mpb.GetMetadataResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metadata, err := s.getMetadataFile(ctx)
	if err != nil {
		return nil, err
	}

	value := metadata.Entries[req.Path]
	return &mpb.GetMetadataResponse{Metadata: &value}, nil
}

func (s *Server) SetMetadata(ctx context.Context, req *mpb.SetMetadataRequest) (*mpb.SetMetadataResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	metadata, err := s.getMetadataFile(ctx)
	if err != nil {
		return nil, err
	}
	metadata.Entries[req.Path] = *req.Metadata

	if err := s.setMetadataFile(ctx, metadata); err != nil {
		return nil, err
	}

	return &mpb.SetMetadataResponse{}, nil
}

func (s *Server) getMetadataFile(ctx context.Context) (*MetadataFile, error) {
	var metadata MetadataFile

	resp, err := s.storageClient.GetBlob(ctx, &spb.GetBlobRequest{Key: metadataFileKey})
	if err != nil {
		if strings.HasSuffix(err.Error(), "no such file or directory") {
			// We deal with the case where the storage server is a fresh one,
			// therefore one without the metadata file.
			//
			// TODO: Explicitly initialize storage server when connecting?
			metadata = MetadataFile{
				Entries: make(map[string]mpb.Metadata),
			}
			return &metadata, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(resp.Data, &metadata); err != nil {
		return nil, errors.New("Unable to deserialize metadata file")
	}

	return &metadata, nil
}

func (s *Server) setMetadataFile(ctx context.Context, metadata *MetadataFile) error {
	reserialized, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	if _, err := s.storageClient.PutBlob(ctx, &spb.PutBlobRequest{Key: metadataFileKey, Data: reserialized}); err != nil {
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

	fileKeysReq := &spb.GetBlobKeysRequest{}
	fileKeysRes, err := s.storageClient.GetBlobKeys(ctx, fileKeysReq)
	if err != nil {
		return err
	}

	for _, key := range fileKeysRes.Keys {
		if !keyMap[key] {
			deleteReq := &mpb.DeleteFileRequest{Path: key}
			if _, err := s.storageClient.DeleteBlob(ctx,
				&spb.DeleteBlobRequest{Key: deleteReq.Path}); err != nil {
				return err
			}
		}
	}

	return nil
}
