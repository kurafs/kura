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

	"github.com/kurafs/kura/pkg/fuse"
	"github.com/kurafs/kura/pkg/fuse/fs"
	"github.com/kurafs/kura/pkg/log"
	cpb "github.com/kurafs/kura/pkg/pb/crypt"
	mpb "github.com/kurafs/kura/pkg/pb/metadata"
	"github.com/kurafs/kura/pkg/streaming"
	"google.golang.org/grpc"
)

// TODO(irfansharif): Methods returning Node should take care to return the same
// fs.Node when the result is logically the same instance. Without this, each
// fs.Node will get a new NodeID, causing spurious cache invalidations, extra
// lookups and aliasing anomalies.

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

func (f *fuseServer) Statfs(
	ctx context.Context,
	req *fuse.StatfsRequest,
	resp *fuse.StatfsResponse,
) error {
	resp.Blocks = 262144
	resp.Bfree = 262144
	resp.Bavail = 32768
	resp.Bsize = 1024 * 1024 // 1 MiB
	resp.Namelen = 128
	resp.Frsize = 1024 // 1 KiB
	return nil
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

	// FUSE ends up splitting up larger writes so sometimes we need to buffer
	// write requests before flushing to storage.
	buffer []byte
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

	// TODO(irfansharif): Investigate why these sometimes return nil. Happens
	// when operating with finder.
	if res.Metadata.LastModified != nil {
		a.Mtime = time.Unix(res.Metadata.LastModified.Seconds, 0)
	}
	if res.Metadata.Created != nil {
		a.Crtime = time.Unix(res.Metadata.Created.Seconds, 0)
	}

	return nil
}

func (f *File) Getattr(
	ctx context.Context,
	req *fuse.GetattrRequest,
	resp *fuse.GetattrResponse,
) error {
	return f.Attr(ctx, &resp.Attr)
}

func (f *File) Setattr(
	ctx context.Context,
	req *fuse.SetattrRequest,
	resp *fuse.SetattrResponse,
) error {
	f.fserver.mu.Lock()
	defer f.fserver.mu.Unlock()

	greq := &mpb.GetMetadataRequest{Path: f.path}
	gres, err := f.fserver.metadataServerClient.GetMetadata(ctx, greq)
	if err != nil {
		return err
	}

	if req.Valid.Mode() {
		gres.Metadata.Permissions = uint32(req.Mode)
	}
	if req.Valid.Size() {
		gres.Metadata.Size = int64(req.Size)
	}
	if req.Valid.Mtime() {
		gres.Metadata.LastModified = &mpb.Metadata_UnixTimestamp{Seconds: req.Mtime.Unix()}
	}

	sreq := &mpb.SetMetadataRequest{
		Path:     f.path,
		Metadata: gres.Metadata,
	}
	_, err = f.fserver.metadataServerClient.SetMetadata(ctx, sreq)
	if err != nil {
		return err
	}

	resp.Attr.Inode = f.inode
	resp.Attr.Mode = os.FileMode(gres.Metadata.Permissions)
	resp.Attr.Size = uint64(gres.Metadata.Size)
	resp.Attr.Mtime = time.Unix(gres.Metadata.LastModified.Seconds, 0)
	resp.Attr.Crtime = time.Unix(gres.Metadata.Created.Seconds, 0)
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

	// TODO(irfansharif): Decrypt streaming.
	req := &mpb.GetFileStreamRequest{Path: f.path}
	stream, err := f.fserver.metadataServerClient.GetFileStream(ctx, req)
	if err != nil {
		return nil, err
	}
	buffer := make([]byte, 0)
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return buffer, nil
		}
		if err != nil {
			return nil, err
		}
		buffer = append(buffer, in.Chunk...)
	}
}

func (f *File) Write(
	ctx context.Context,
	req *fuse.WriteRequest,
	resp *fuse.WriteResponse,
) error {
	f.fserver.mu.Lock()
	defer f.fserver.mu.Unlock()

	if req.Offset == 0 {
		f.buffer = append(f.buffer, req.Data...)
	} else {
		// NB: We do the simple thing of growing out the buffer (possibly more
		// than needed), copying data as needed, and trimming down our
		// reference.
		f.buffer = append(f.buffer, req.Data...)
		copy(f.buffer[req.Offset:], req.Data)
		f.buffer = f.buffer[:req.Offset+int64(len(req.Data))]
	}
	resp.Size = len(req.Data)
	return nil
}

func (f *File) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	f.fserver.mu.Lock()
	defer f.fserver.mu.Unlock()

	if f.buffer == nil {
		// Flush is (surprisingly) also called on Read requests. We do nothing.
		return nil
	}

	mreq := &mpb.GetMetadataRequest{Path: f.path}
	mres, err := f.fserver.metadataServerClient.GetMetadata(ctx, mreq)
	if err != nil {
		return err
	}

	metadata := &mpb.Metadata{
		Size:         int64(len(f.buffer)),
		LastModified: &mpb.Metadata_UnixTimestamp{Seconds: time.Now().Unix()},
		Created:      mres.Metadata.Created,
		Permissions:  mres.Metadata.Permissions,
		IsDirectory:  false,
		AccessList:   mres.Metadata.AccessList,
	}

	if len(f.buffer) < streaming.Threshold {
		// TODO(irfansharif): Cache file keys somewhere to not make this request
		// each time.
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
		ereq := &cpb.EncryptionRequest{Plaintext: f.buffer, AesKey: fkey}
		eres, err := f.fserver.cryptServerClient.Encrypt(ctx, ereq)
		if err != nil {
			return err
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
		f.buffer = nil
		return nil
	}

	stream, err := f.fserver.metadataServerClient.PutFileStream(ctx)
	if err != nil {
		return err
	}

	// TODO(irfansharif): Encrypt chunks.
	chunker := streaming.NewChunker(f.buffer)
	chunker.Next()
	fm := &mpb.PutFileStreamRequest{
		Path:     f.path,
		Chunk:    chunker.Value(),
		Metadata: metadata,
	}
	if err = stream.Send(fm); err != nil {
		return err
	}
	for chunker.Next() {
		req := &mpb.PutFileStreamRequest{
			Chunk: chunker.Value(),
		}
		if err = stream.Send(req); err != nil {
			return err
		}
	}
	if _, err = stream.CloseAndRecv(); err != nil {
		return err
	}
	f.buffer = nil
	return nil
}

// We provide sink implementations of xattributes to work with Finder. Not
// implementing these results in Finder creating "._" files that don't work well
// with FUSE.
func (f *File) Setxattr(
	ctx context.Context,
	req *fuse.SetxattrRequest,
) error {
	return nil
}

func (f *File) Getxattr(
	ctx context.Context,
	req *fuse.GetxattrRequest,
	resp *fuse.GetxattrResponse,
) error {
	return fuse.ErrNoXattr
}

func (f *File) Removexattr(
	ctx context.Context,
	req *fuse.RemovexattrRequest,
) error {
	return fuse.ErrNoXattr
}

func (f *File) Listxattr(
	ctx context.Context,
	req *fuse.ListxattrRequest,
	resp *fuse.ListxattrResponse,
) error {
	return fuse.ErrNoXattr
}

func (f *File) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	return nil
}
