package fuseserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"
	"sync"
	"time"

	"github.com/kurafs/kura/pkg/streaming"

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
	aeskey  []byte
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

func wrap(ctx context.Context, cs cpb.CryptServiceClient) (*mpb.Metadata_Accessor, error) {
	preq := &cpb.PublicKeyRequest{}
	pres, err := cs.PublicKey(ctx, preq)
	if err != nil {
		return nil, err
	}

	h := sha256.New()
	h.Write(pres.X)
	h.Write(pres.Y)
	sum := h.Sum(nil)

	aeskey := make([]byte, 16)
	_, err = rand.Read(aeskey)
	if err != nil {
		return nil, err
	}

	ereq := &cpb.EncryptionRequest{Plaintext: aeskey}
	eres, err := cs.Encrypt(ctx, ereq)
	if err != nil {
		return nil, err
	}

	ma := mpb.Metadata_Accessor{
		IdentityHash: sum,
		EncryptedKey: eres.Ciphertext,
	}
	return &ma, nil
}

func (d *Dir) Create(
	ctx context.Context,
	req *fuse.CreateRequest,
	res *fuse.CreateResponse,
) (fs.Node, fs.Handle, error) {
	d.fserver.mu.Lock()
	defer d.fserver.mu.Unlock()

	ma, err := wrap(ctx, d.fserver.cryptServerClient)
	if err != nil {
		return nil, nil, err
	}

	alist := make([]*mpb.Metadata_Accessor, 0)
	alist = append(alist, ma)

	ts := time.Now().Unix()
	metadata := &mpb.Metadata{
		Created:      &mpb.Metadata_UnixTimestamp{Seconds: ts},
		LastModified: &mpb.Metadata_UnixTimestamp{Seconds: ts},
		Permissions:  uint32(req.Mode),
		Size:         int64(0),
		IsDirectory:  false,
		AccessList:   alist,
	}

	rq := &mpb.PutFileRequest{
		Path:     path.Join(d.path, req.Name),
		File:     []byte{},
		Metadata: metadata,
	}
	_, err = d.fserver.metadataServerClient.PutFile(ctx, rq)
	if err != nil {
		return nil, nil, err
	}

	file := File{
		path:        path.Join(d.path, req.Name),
		fserver:     d.fserver,
		inode:       fs.GenerateDynamicInode(d.inode, path.Join(d.path, req.Name)),
		bufferWrite: false,
		writeBuffer: make([]byte, 0, streaming.Threshold),
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

func (d *Dir) Rename(
	ctx context.Context,
	req *fuse.RenameRequest,
	newDir fs.Node,
) error {
	d.fserver.mu.Lock()
	defer d.fserver.mu.Unlock()

	dir := newDir.(*Dir)
	rq := &mpb.RenameRequest{
		OldPath: path.Join(d.path, req.OldName),
		NewPath: path.Join(dir.path, req.NewName),
	}
	_, err := d.fserver.metadataServerClient.Rename(ctx, rq)
	if err != nil {
		return err
	}

	return nil
}

// File implements both fs.Node and fs.Handle.
type File struct {
	path    string
	fserver *fuseServer
	inode   uint64

	// FUSE ends up splitting up larger writes so sometimes we need to buffer write requests
	// before flushing to storage
	writeBuffer []byte
	bufferWrite bool
}

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	f.fserver.mu.Lock()
	defer f.fserver.mu.Unlock()

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

	mreq := &mpb.GetMetadataRequest{Path: f.path}
	mres, err := f.fserver.metadataServerClient.GetMetadata(ctx, mreq)
	if err != nil {
		return nil, err
	}

	preq := &cpb.PublicKeyRequest{}
	pres, err := f.fserver.cryptServerClient.PublicKey(ctx, preq)
	if err != nil {
		return nil, err
	}

	h := sha256.New()
	h.Write(pres.X)
	h.Write(pres.Y)
	sum := h.Sum(nil)

	var ekey []byte
	for _, a := range mres.Metadata.AccessList {
		if bytes.Equal(a.IdentityHash, sum) {
			ekey = a.EncryptedKey
			break
		}
	}

	if ekey == nil {
		return nil, fmt.Errorf("key for file not found")
	}

	if mres.Metadata.Size < streaming.Threshold {
		req := &mpb.GetFileRequest{Path: f.path}
		res, err := f.fserver.metadataServerClient.GetFile(ctx, req)
		if err != nil {
			return nil, err
		}

		dreq := &cpb.DecryptionRequest{Ciphertext: ekey}
		dres, err := f.fserver.cryptServerClient.Decrypt(ctx, dreq)
		if err != nil {
			return nil, err
		}

		fkey := dres.Plaintext
		creq := &cpb.DecryptionRequest{Ciphertext: res.File, AesKey: fkey}
		cres, err := f.fserver.cryptServerClient.Decrypt(ctx, creq)
		if err != nil {
			return nil, err
		}

		return cres.Plaintext, nil
	}

	// TODO: Decrypt
	req := &mpb.GetFileStreamRequest{Path: f.path}
	stream, err := f.fserver.metadataServerClient.GetFileStream(ctx, req)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 0, streaming.Threshold)
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return buf, nil
		}
		if err != nil {
			return nil, err
		}
		buf = append(buf, in.Chunk...)
	}
}

func (f *File) Write(
	ctx context.Context,
	req *fuse.WriteRequest,
	resp *fuse.WriteResponse,
) error {
	f.fserver.mu.Lock()
	defer f.fserver.mu.Unlock()

	f.writeBuffer = append(f.writeBuffer, req.Data...)
	f.fserver.logger.Infof("Wrote %d bytes to buffer, first five bytes: %x", len(req.Data), req.Data[:5])

	// XXX "._" files need to written immediately... This results
	// in buffered writes having the first chunk being written to the path first
	if len(req.Data) < streaming.Threshold && req.Offset == 0 && !f.bufferWrite {
		// TODO(irfansharif): Cache file keys somewhere to not make this request
		// each time.
		mreq := &mpb.GetMetadataRequest{Path: f.path}
		mres, err := f.fserver.metadataServerClient.GetMetadata(ctx, mreq)
		if err != nil {
			return err
		}

		preq := &cpb.PublicKeyRequest{}
		pres, err := f.fserver.cryptServerClient.PublicKey(ctx, preq)
		if err != nil {
			return err
		}

		h := sha256.New()
		h.Write(pres.X)
		h.Write(pres.Y)
		sum := h.Sum(nil)

		var ekey []byte
		for _, a := range mres.Metadata.AccessList {
			if bytes.Equal(a.IdentityHash, sum) {
				ekey = a.EncryptedKey
				break
			}
		}

		if ekey == nil {
			return fmt.Errorf("key for file not found")
		}

		dreq := &cpb.DecryptionRequest{Ciphertext: ekey}
		dres, err := f.fserver.cryptServerClient.Decrypt(ctx, dreq)
		if err != nil {
			return err
		}

		fkey := dres.Plaintext
		ereq := &cpb.EncryptionRequest{Plaintext: req.Data, AesKey: fkey}
		eres, err := f.fserver.cryptServerClient.Encrypt(ctx, ereq)
		if err != nil {
			return err
		}

		metadata := &mpb.Metadata{
			Size:         int64(len(req.Data)),
			LastModified: &mpb.Metadata_UnixTimestamp{Seconds: time.Now().Unix()},
			Created:      mres.Metadata.Created,
			Permissions:  mres.Metadata.Permissions,
			IsDirectory:  false,
			AccessList:   mres.Metadata.AccessList,
		}

		rq := &mpb.PutFileRequest{
			Path:     f.path,
			File:     []byte(eres.Ciphertext),
			Metadata: metadata,
		}
		_, err = f.fserver.metadataServerClient.PutFile(ctx, rq)
		if err != nil {
			return err
		}
	} else {
		f.bufferWrite = true
	}
	resp.Size = len(req.Data)
	return nil
}

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	f.fserver.mu.Lock()
	defer f.fserver.mu.Unlock()
	f.bufferWrite = false
	f.writeBuffer = make([]byte, 0, streaming.Threshold)
	resp.Handle = fuse.HandleID(f.inode)
	return f, nil
}

func (f *File) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	f.fserver.mu.Lock()
	defer f.fserver.mu.Unlock()

	if !f.bufferWrite {
		f.writeBuffer = nil
		return nil
	}

	f.fserver.logger.Infof("Writing %d bytes to storage for %s", len(f.writeBuffer), f.path)

	mreq := &mpb.GetMetadataRequest{Path: f.path}
	mres, err := f.fserver.metadataServerClient.GetMetadata(ctx, mreq)
	if err != nil {
		return err
	}
	metadata := &mpb.Metadata{
		Size:         int64(len(f.writeBuffer)),
		LastModified: &mpb.Metadata_UnixTimestamp{Seconds: time.Now().Unix()},
		Created:      mres.Metadata.Created,
		Permissions:  mres.Metadata.Permissions,
		IsDirectory:  false,
		AccessList:   mres.Metadata.AccessList,
	}

	stream, err := f.fserver.metadataServerClient.PutFileStream(ctx)
	if err != nil {
		return err
	}

	// TODO: Encrypt
	chunker := streaming.NewChunker(f.writeBuffer)
	chunker.Next()
	fm := &mpb.PutFileStreamRequest{Path: f.path, Chunk: chunker.Value(), Metadata: metadata}
	if err = stream.Send(fm); err != nil {
		return err
	}
	for chunker.Next() {
		if err = stream.Send(&mpb.PutFileStreamRequest{Chunk: chunker.Value()}); err != nil {
			return err
		}
	}
	if _, err = stream.CloseAndRecv(); err != nil {
		return err
	}

	f.writeBuffer = nil
	return nil
}
