package fuseserver

import (
	"context"
	"io"
	"os"
	"sync"
	"time"

	"github.com/kurafs/kura/pkg/fuse"
	"github.com/kurafs/kura/pkg/fuse/fs"
	"github.com/kurafs/kura/pkg/log"
<<<<<<< HEAD
	"github.com/kurafs/kura/pkg/streaming"

=======
	cpb "github.com/kurafs/kura/pkg/pb/crypt"
>>>>>>> master
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

	nodeCounter uint64 // HACK: This is not how we should be generating inode numbers.
}

func newFUSEServer(logger *log.Logger, metadataServerAddr, cryptServerAddr string) (fs.FS, error) {
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

	dir := Dir{fuseServer: f}
	return &dir, nil
}

// Dir implements both Node and Handle for the root directory.
type Dir struct {
	fuseServer *fuseServer
}

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = 1
	a.Mode = os.ModeDir | 0555
	return nil
}

func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	d.fuseServer.mu.Lock()
	defer d.fuseServer.mu.Unlock()

	req := &mpb.GetFileRequest{Key: name}
	_, err := d.fuseServer.metadataServerClient.GetFile(ctx, req) // TODO: We're discarding the response.
	if err != nil {
		return nil, fuse.ENOENT
	}

	return &File{name: name, parentDir: d}, nil
}

func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, res *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	d.fuseServer.mu.Lock()
	defer d.fuseServer.mu.Unlock()

	ts := time.Now().Unix()
	metadata := &mpb.FileMetadata{
		Created:      &mpb.FileMetadata_UnixTimestamp{Seconds: ts},
		LastModified: &mpb.FileMetadata_UnixTimestamp{Seconds: ts},
		Permissions:  0644,
		Size:         int64(0),
	}

	rq := &mpb.PutFileRequest{Key: req.Name, File: []byte{}, Metadata: metadata}
	_, err := d.fuseServer.metadataServerClient.PutFile(ctx, rq)
	if err != nil {
		return nil, nil, err // TODO(irfansharif): Propagate appropriate FUSE error.
	}

	d.fuseServer.nodeCounter += 1
	res.Node = fuse.NodeID(d.fuseServer.nodeCounter)
	res.OpenResponse.Handle = fuse.HandleID(d.fuseServer.nodeCounter)
	file := &File{name: req.Name, parentDir: d}
	return file, file, nil
}

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	d.fuseServer.mu.Lock()
	defer d.fuseServer.mu.Unlock()

	req := &mpb.GetDirectoryKeysRequest{}
	res, err := d.fuseServer.metadataServerClient.GetDirectoryKeys(ctx, req)
	if err != nil {
		return nil, err
	}

	dirents := make([]fuse.Dirent, 0)
	for i, key := range res.Keys {
		// TODO: Ensure unique inode numbers.
		dirents = append(dirents,
			fuse.Dirent{Inode: uint64(i + 42), Name: key, Type: fuse.DT_File})
	}

	return dirents, nil
}

// File implements both Node and Handle for the hello file.
type File struct {
	name      string
	parentDir *Dir
}

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	f.parentDir.fuseServer.mu.Lock()
	defer f.parentDir.fuseServer.mu.Unlock()

	req := &mpb.GetMetadataRequest{Key: f.name}
	res, err := f.parentDir.fuseServer.metadataServerClient.GetMetadata(ctx, req)
	if err != nil {
		return err // TODO(irfansharif): Propagate appropriate FUSE error.
	}

	a.Inode = 2   // TODO: Store this in metadata server.
	a.Mode = 0444 // TODO: Extract this from req.Metadata.Permissions.
	a.Size = uint64(res.Metadata.Size)

	return nil
}

func (f *File) ReadAll(ctx context.Context) ([]byte, error) {
	f.parentDir.fuseServer.mu.Lock()
	defer f.parentDir.fuseServer.mu.Unlock()

	// TODO (Dendrimer): Use this as a chance to refresh metadata cache
	metaReq := &mpb.GetMetadataRequest{Key: f.name}
	metaResp, err := f.parentDir.fuseServer.metadataServerClient.GetMetadata(ctx, metaReq)
	if err != nil {
		return nil, err // TODO(irfansharif): Propagate appropriate FUSE error.
	}
	fileSize := metaResp.Metadata.Size

	var contents string
	if fileSize < streaming.Threshold {
		req := &mpb.GetFileRequest{Key: f.name}
		res, err := f.parentDir.fuseServer.metadataServerClient.GetFile(ctx, req)
		if err != nil {
			return nil, err // TODO(irfansharif): Propagate appropriate FUSE error.
		}
		contents = string(res.File)
	} else {
		req := &mpb.GetFileStreamRequest{Key: f.name}
		stream, err := f.parentDir.fuseServer.metadataServerClient.GetFileStream(ctx, req)
		if err != nil {
			return nil, err
		}
		// TODO(Denrimer): Is it wise to be allocating huge amounts of memory
		// up front? If we can decrypt from a stream then we don't have to do this.
		buf := make([]byte, 0, fileSize)
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			buf = append(buf, in.FileChunk...)
		}
		contents = string(buf)
	}

<<<<<<< HEAD
	decrypted, err := decrypt(pkey, contents)
=======
	creq := &cpb.DecryptionRequest{Ciphertext: res.File}
	cres, err := f.parentDir.fuseServer.cryptServerClient.Decrypt(ctx, creq)
>>>>>>> master
	if err != nil {
		f.parentDir.fuseServer.logger.Error(err.Error())
		return nil, err // TODO(irfansharif): Propagate appropriate FUSE error.
	}

	return cres.Plaintext, nil
}

func (f *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	f.parentDir.fuseServer.mu.Lock()
	defer f.parentDir.fuseServer.mu.Unlock()

	creq := &cpb.EncryptionRequest{Plaintext: req.Data}
	cres, err := f.parentDir.fuseServer.cryptServerClient.Encrypt(ctx, creq)
	if err != nil {
		f.parentDir.fuseServer.logger.Error(err.Error())
		return err // TODO(irfansharif): Propagate appropriate FUSE error.
	}

<<<<<<< HEAD
	encBytes := []byte(encrypted)
	if len(encBytes) < streaming.Threshold {
		rq := &mpb.PutFileRequest{Key: f.name, File: encBytes}
		_, err = f.parentDir.fuseServer.metadataServerClient.PutFile(ctx, rq)

		if err != nil {
			return err // TODO(irfansharif): Propagate appropriate FUSE error.
		}
	} else {
		stream, err := f.parentDir.fuseServer.metadataServerClient.PutFileStream(ctx)
		if err != nil {
			return err // TODO: Propogate appropriate FUSE error
		}
		numChunks := len(encBytes) / streaming.ChunkSize
		if len(encBytes)%streaming.ChunkSize != 0 {
			numChunks++
		}
		for i := 0; i < numChunks; i++ {
			b := i * streaming.ChunkSize
			e := (i + 1) * streaming.ChunkSize
			if e >= len(encBytes) {
				e = len(encBytes) - 1
			}

			if err := stream.Send(&mpb.PutFileStreamRequest{Key: f.name, FileChunk: encBytes[b:e]}); err != nil {
				return err
			}
		}
=======
	ts := time.Now().Unix()
	metadata := &mpb.FileMetadata{
		Size:         int64(len(req.Data)),
		LastModified: &mpb.FileMetadata_UnixTimestamp{Seconds: ts},
	}

	rq := &mpb.PutFileRequest{
		Key:      f.name,
		File:     []byte(cres.Ciphertext),
		Metadata: metadata,
>>>>>>> master
	}
	_, err = f.parentDir.fuseServer.metadataServerClient.PutFile(ctx, rq)

	if err != nil {
		return err // TODO(irfansharif): Propagate appropriate FUSE error.
	}

	resp.Size = len(req.Data)
	return nil
}

func (d *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	d.fuseServer.mu.Lock()
	defer d.fuseServer.mu.Unlock()

	rq := &mpb.DeleteFileRequest{Key: req.Name}
	_, err := d.fuseServer.metadataServerClient.DeleteFile(ctx, rq)
	if err != nil {
		return err // TODO(irfansharif): Propagate appropriate FUSE error.
	}

	return nil
}
