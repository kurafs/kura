package fuseserver

import (
	"context"
	"os"
	"sync"

	"google.golang.org/grpc"

	"github.com/kurafs/kura/pkg/fuse"
	"github.com/kurafs/kura/pkg/fuse/fs"
	"github.com/kurafs/kura/pkg/log"

	pb "github.com/kurafs/kura/pkg/pb/metadata"
)

type fuseServer struct {
	logger               *log.Logger
	metadataServerClient pb.MetadataServiceClient
	conn                 *grpc.ClientConn
	mu                   sync.RWMutex

	nodeCounter uint64 // HACK: This is not how we should be generating inode numbers.
}

var pkey []byte = []byte("LKHlhb899Y09olUi")

func newFUSEServer(logger *log.Logger, metadataServerAddr string) (fs.FS, error) {
	conn, err := grpc.Dial(metadataServerAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	client := pb.NewMetadataServiceClient(conn)
	server := fuseServer{
		logger:               logger,
		conn:                 conn,
		metadataServerClient: client,
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

	req := &pb.GetFileRequest{Key: name}
	_, err := d.fuseServer.metadataServerClient.GetFile(ctx, req) // TODO: We're discarding the response.
	if err != nil {
		return nil, fuse.ENOENT
	}

	return &File{name: name, parentDir: d}, nil
}

func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, res *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	d.fuseServer.mu.Lock()
	defer d.fuseServer.mu.Unlock()

	rq := &pb.PutFileRequest{Key: req.Name, File: []byte{}}
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

	req := &pb.GetDirectoryKeysRequest{}
	res, err := d.fuseServer.metadataServerClient.GetDirectoryKeys(ctx, req)
	if err != nil {
		return nil, err
	}

	dirents := make([]fuse.Dirent, 0)
	for i, key := range res.Keys {
		// TODO: Ensure unique inode numbers.
		dirents = append(dirents, fuse.Dirent{Inode: uint64(i + 42), Name: key, Type: fuse.DT_File})
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

	req := &pb.GetMetadataRequest{Key: f.name}
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

	req := &pb.GetFileRequest{Key: f.name}
	res, err := f.parentDir.fuseServer.metadataServerClient.GetFile(ctx, req)
	if err != nil {
		return nil, err // TODO(irfansharif): Propagate appropriate FUSE error.
	}

	decrypted, err := decrypt(pkey, string(res.File))
	if err != nil {
		f.parentDir.fuseServer.logger.Error(err.Error())
		return nil, err // TODO(irfansharif): Propagate appropriate FUSE error.
	}
	return []byte(decrypted), nil
}

func (f *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	f.parentDir.fuseServer.mu.Lock()
	defer f.parentDir.fuseServer.mu.Unlock()

	encrypted, err := encrypt(pkey, string(req.Data))
	if err != nil {
		f.parentDir.fuseServer.logger.Error(err.Error())
		return err // TODO(irfansharif): Propagate appropriate FUSE error.
	}
	rq := &pb.PutFileRequest{Key: f.name, File: []byte(encrypted)}
	_, err = f.parentDir.fuseServer.metadataServerClient.PutFile(ctx, rq)

	if err != nil {
		return err // TODO(irfansharif): Propagate appropriate FUSE error.
	}

	mrq := &pb.SetMetadataRequest{Key: f.name, Metadata: &pb.FileMetadata{Size: int64(len(req.Data))}}
	_, err = f.parentDir.fuseServer.metadataServerClient.SetMetadata(ctx, mrq)
	if err != nil {
		return err // TODO(irfansharif): Propagate appropriate FUSE error.
	}

	resp.Size = len(req.Data)
	return nil
}

func (d *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	d.fuseServer.mu.Lock()
	defer d.fuseServer.mu.Unlock()

	rq := &pb.DeleteFileRequest{Key: req.Name}
	_, err := d.fuseServer.metadataServerClient.DeleteFile(ctx, rq)
	if err != nil {
		return err // TODO(irfansharif): Propagate appropriate FUSE error.
	}

	return nil
}
