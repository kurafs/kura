package fuseserver

import (
	"context"
	"os"
	"path"
	"sync"
	"time"

	"github.com/kurafs/kura/pkg/fuse"
	"github.com/kurafs/kura/pkg/fuse/fs"
	"github.com/kurafs/kura/pkg/log"
	cpb "github.com/kurafs/kura/pkg/pb/crypt"
	mpb "github.com/kurafs/kura/pkg/pb/metadata"
	"google.golang.org/grpc"
)

type fuseServer struct {
	logger               *log.Logger
	metadataServerClient mpb.MetadataServiceClient
	metadataConn         *grpc.ClientConn
	cryptServerClient    cpb.CryptServiceClient
	cryptConn            *grpc.ClientConn
	mu                   sync.RWMutex
}

func newFUSEServer(
	logger *log.Logger,
	metadataServerAddr,
	cryptServerAddr string,
) (fs.FS, error) {
	metadataConn, err := grpc.Dial(metadataServerAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	metadataClient := mpb.NewMetadataServiceClient(metadataConn)

	cryptConn, err := grpc.Dial(cryptServerAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	cryptClient := cpb.NewCryptServiceClient(cryptConn)
	server := fuseServer{
		logger:               logger,
		metadataConn:         metadataConn,
		metadataServerClient: metadataClient,
		cryptConn:            cryptConn,
		cryptServerClient:    cryptClient,
	}

	return &server, nil
}

func (f *fuseServer) Root() (fs.Node, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	dir := Dir{
		fserver: f,
		path:    "kura-root",
		inode:   fs.GenerateDynamicInode(0, "kura-root"),
	}
	return &dir, nil
}

// Dir implements both fs.Node and fs.Handle for the root directory.
type Dir struct {
	fserver *fuseServer
	path    string
	inode   uint64
}

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	d.fserver.mu.Lock()
	defer d.fserver.mu.Unlock()

	if d.path == "kura-root" {
		a.Inode = d.inode
		a.Mode = os.ModeDir | 0555
		a.Size = 0
		return nil
	}

	req := &mpb.GetMetadataRequest{Path: d.path}
	res, err := d.fserver.metadataServerClient.GetMetadata(ctx, req)
	if err != nil {
		return err
	}

	a.Inode = d.inode
	a.Mode = os.ModeDir | os.FileMode(res.Metadata.Permissions)
	a.Size = uint64(res.Metadata.Size)
	a.Mtime = time.Unix(res.Metadata.LastModified.Seconds, 0)
	a.Crtime = time.Unix(res.Metadata.Created.Seconds, 0)
	// TODO(irfansharif): Include AccessTime.
	return nil
}

func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	d.fserver.mu.Lock()
	defer d.fserver.mu.Unlock()

	req := &mpb.GetDirectoryEntriesRequest{Path: d.path}
	res, err := d.fserver.metadataServerClient.GetDirectoryEntries(ctx, req)
	if err != nil {
		return nil, err
	}

	fpath := path.Join(d.path, name)
	for _, entry := range res.Entries {
		if entry.Path != fpath {
			continue
		}
		if entry.IsDirectory {
			dir := Dir{
				fserver: d.fserver,
				path:    fpath,
				inode:   fs.GenerateDynamicInode(d.inode, fpath),
			}
			return &dir, nil
		} else {
			file := File{
				path:    fpath,
				fserver: d.fserver,
				inode:   fs.GenerateDynamicInode(d.inode, fpath),
			}
			return &file, nil
		}
	}

	return nil, fuse.ENOENT
}

func (d *Dir) Create(
	ctx context.Context,
	req *fuse.CreateRequest,
	res *fuse.CreateResponse,
) (fs.Node, fs.Handle, error) {
	d.fserver.mu.Lock()
	defer d.fserver.mu.Unlock()

	ts := time.Now().Unix()
	metadata := &mpb.Metadata{
		Created:      &mpb.Metadata_UnixTimestamp{Seconds: ts},
		LastModified: &mpb.Metadata_UnixTimestamp{Seconds: ts},
		Permissions:  uint32(req.Mode),
		Size:         int64(0),
		IsDirectory:  false,
	}

	rq := &mpb.PutFileRequest{
		Path:     path.Join(d.path, req.Name),
		File:     []byte{},
		Metadata: metadata,
	}
	_, err := d.fserver.metadataServerClient.PutFile(ctx, rq)
	if err != nil {
		return nil, nil, err
	}

	file := File{
		path:    path.Join(d.path, req.Name),
		fserver: d.fserver,
		inode:   fs.GenerateDynamicInode(d.inode, path.Join(d.path, req.Name)),
	}
	res.Node = fuse.NodeID(file.inode)
	res.OpenResponse.Handle = fuse.HandleID(file.inode)
	return &file, &file, nil
}

func (d *Dir) Mkdir(
	ctx context.Context,
	req *fuse.MkdirRequest,
) (fs.Node, error) {
	d.fserver.mu.Lock()
	defer d.fserver.mu.Unlock()

	ts := time.Now().Unix()
	metadata := &mpb.Metadata{
		Created:      &mpb.Metadata_UnixTimestamp{Seconds: ts},
		LastModified: &mpb.Metadata_UnixTimestamp{Seconds: ts},
		Permissions:  uint32(req.Mode),
		Size:         int64(0),
		IsDirectory:  true,
	}
	rq := &mpb.CreateDirectoryRequest{
		Path:     path.Join(d.path, req.Name),
		Metadata: metadata,
	}
	_, err := d.fserver.metadataServerClient.CreateDirectory(ctx, rq)
	if err != nil {
		return nil, err
	}

	dir := Dir{
		fserver: d.fserver,
		path:    path.Join(d.path, req.Name),
		inode:   fs.GenerateDynamicInode(d.inode, path.Join(d.path, req.Name)),
	}
	return &dir, nil
}

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	d.fserver.mu.Lock()
	defer d.fserver.mu.Unlock()

	req := &mpb.GetDirectoryEntriesRequest{Path: d.path}
	res, err := d.fserver.metadataServerClient.GetDirectoryEntries(ctx, req)
	if err != nil {
		return nil, err
	}

	dirents := make([]fuse.Dirent, 0)
	for _, entry := range res.Entries {
		var etype fuse.DirentType
		if entry.IsDirectory {
			etype = fuse.DT_Dir
		} else {
			etype = fuse.DT_File
		}

		dirents = append(dirents,
			fuse.Dirent{
				Inode: fs.GenerateDynamicInode(d.inode, entry.Path),
				Name:  entry.Path[len(d.path+"/"):],
				Type:  etype,
			})
	}

	return dirents, nil
}

func (d *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	d.fserver.mu.Lock()
	defer d.fserver.mu.Unlock()

	if req.Dir {
		rq := &mpb.DeleteDirectoryRequest{Path: path.Join(d.path, req.Name)}
		_, err := d.fserver.metadataServerClient.DeleteDirectory(ctx, rq)
		if err != nil {
			return err
		}
	} else {
		rq := &mpb.DeleteFileRequest{Path: path.Join(d.path, req.Name)}
		_, err := d.fserver.metadataServerClient.DeleteFile(ctx, rq)
		if err != nil {
			return err
		}
	}

	return nil
}

// File implements both fs.Node and fs.Handle.
type File struct {
	path    string
	fserver *fuseServer
	inode   uint64
}

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	f.fserver.mu.Lock()
	defer f.fserver.mu.Unlock()

	return f.AttrLocked(ctx, a)
}

func (f *File) AttrLocked(ctx context.Context, a *fuse.Attr) error {
	req := &mpb.GetMetadataRequest{Path: f.path}
	res, err := f.fserver.metadataServerClient.GetMetadata(ctx, req)
	if err != nil {
		return err
	}

	a.Inode = f.inode
	a.Mode = os.FileMode(res.Metadata.Permissions)
	a.Size = uint64(res.Metadata.Size)
	a.Mtime = time.Unix(res.Metadata.LastModified.Seconds, 0)
	a.Crtime = time.Unix(res.Metadata.Created.Seconds, 0)
	return nil
}

func (f *File) ReadAll(ctx context.Context) ([]byte, error) {
	f.fserver.mu.Lock()
	defer f.fserver.mu.Unlock()

	req := &mpb.GetFileRequest{Path: f.path}
	res, err := f.fserver.metadataServerClient.GetFile(ctx, req)
	if err != nil {
		return nil, err
	}

	creq := &cpb.DecryptionRequest{Ciphertext: res.File}
	cres, err := f.fserver.cryptServerClient.Decrypt(ctx, creq)
	if err != nil {
		return nil, err
	}

	return cres.Plaintext, nil
}

func (f *File) Write(
	ctx context.Context,
	req *fuse.WriteRequest,
	resp *fuse.WriteResponse,
) error {
	f.fserver.mu.Lock()
	defer f.fserver.mu.Unlock()

	creq := &cpb.EncryptionRequest{Plaintext: req.Data}
	cres, err := f.fserver.cryptServerClient.Encrypt(ctx, creq)
	if err != nil {
		return err
	}

	var attr fuse.Attr
	f.AttrLocked(ctx, &attr)

	metadata := &mpb.Metadata{
		Size:         int64(len(req.Data)),
		LastModified: &mpb.Metadata_UnixTimestamp{Seconds: time.Now().Unix()},
		Created:      &mpb.Metadata_UnixTimestamp{Seconds: attr.Crtime.Unix()},
		Permissions:  uint32(attr.Mode),
		IsDirectory:  false,
	}

	rq := &mpb.PutFileRequest{
		Path:     f.path,
		File:     []byte(cres.Ciphertext),
		Metadata: metadata,
	}
	_, err = f.fserver.metadataServerClient.PutFile(ctx, rq)
	if err != nil {
		return err
	}

	resp.Size = len(req.Data)
	return nil
}
