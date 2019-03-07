package fuseserver

import (
	"context"
	"fmt"
	"os"
	"sync"
	"syscall"
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

	node := Node{fuseServer: f, Mode: os.ModeDir | 0555}
	return &node, nil
}

// Dir implements both Node and Handle for the root directory.
type Node struct {
	fuseServer *fuseServer
	parent     *Node
	name       string
	Mode       os.FileMode
}

func (node *Node) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = node.Mode

	if node.Mode.IsDir() {
		a.Inode = 1
	} else {
		req := &mpb.GetMetadataRequest{Key: node.name}
		res, err := node.parent.fuseServer.metadataServerClient.GetMetadata(ctx, req)
		if err != nil {
			return err // TODO(irfansharif): Propagate appropriate FUSE error.
		}

		a.Inode = 2 // TODO: Store this in metadata server.
		a.Size = uint64(res.Metadata.Size)
	}
	return nil
}

func (node *Node) Lookup(ctx context.Context, name string) (fs.Node, error) {
	fmt.Println("lookup key ", name)
	if !node.Mode.IsDir() {
		return nil, fuse.Errno(syscall.ENOTDIR)
	}

	node.fuseServer.mu.Lock()
	defer node.fuseServer.mu.Unlock()

	req := &mpb.GetMetadataRequest{Key: name}
	res, err := node.fuseServer.metadataServerClient.GetMetadata(ctx, req)
	if err != nil {
		return nil, fuse.ENOENT
	}

	var mode os.FileMode = 0444
	if res.Metadata.IsDirectory {
		mode = os.ModeDir | 0555
	}

	return &Node{name: name, parent: node, Mode: mode, fuseServer: node.fuseServer}, nil
}

func (node *Node) Create(ctx context.Context, req *fuse.CreateRequest, res *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	fmt.Println("in create, node name: ", node.name)
	if !node.Mode.IsDir() {
		return nil, nil, fuse.Errno(syscall.ENOTDIR)
	}

	if req.Mode.IsDir() {
		return nil, nil, fuse.Errno(syscall.EISDIR)
	} else if !req.Mode.IsRegular() {
		return nil, nil, fuse.Errno(syscall.EINVAL)
	}

	node.fuseServer.mu.Lock()
	defer node.fuseServer.mu.Unlock()

	ts := time.Now().Unix()
	metadata := &mpb.FileMetadata{
		Created:         &mpb.FileMetadata_UnixTimestamp{Seconds: ts},
		LastModified:    &mpb.FileMetadata_UnixTimestamp{Seconds: ts},
		Permissions:     0644,
		Size:            int64(0),
		ParentDirectory: node.name,
	}

	rq := &mpb.PutFileRequest{Key: req.Name, File: []byte{}, Metadata: metadata}
	_, err := node.fuseServer.metadataServerClient.PutFile(ctx, rq)
	if err != nil {
		return nil, nil, err // TODO(irfansharif): Propagate appropriate FUSE error.
	}

	node.fuseServer.nodeCounter += 1
	res.Node = fuse.NodeID(node.fuseServer.nodeCounter)
	res.OpenResponse.Handle = fuse.HandleID(node.fuseServer.nodeCounter)
	file := &Node{name: req.Name, parent: node, Mode: 0444, fuseServer: node.fuseServer}
	return file, file, nil
}

func (node *Node) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	fmt.Println("in mkdir")
	//node.fuseServer.mu.Lock()
	//defer node.fuseServer.mu.Unlock()

	if !node.Mode.IsDir() {
		return nil, fuse.Errno(syscall.ENOTDIR)
	}

	rq := &mpb.CreateDirectoryRequest{Key: req.Name}
	_, err := node.fuseServer.metadataServerClient.CreateDirectory(ctx, rq)

	if err != nil {
		fmt.Println("got mkdir err: ", err)
		return nil, err
	}

	newNode := &Node{name: req.Name, parent: node, Mode: os.ModeDir | 0555, fuseServer: node.fuseServer}
	return newNode, nil
}

func (node *Node) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	fmt.Println("read dir all dir name: ", node.name, ", len: ", len(node.name))
	if !node.Mode.IsDir() {
		return nil, fuse.Errno(syscall.ENOTDIR)
	}

	node.fuseServer.mu.Lock()
	defer node.fuseServer.mu.Unlock()

	req := &mpb.GetDirectoryKeysRequest{Directory: node.name}
	res, err := node.fuseServer.metadataServerClient.GetDirectoryKeys(ctx, req)
	if err != nil {
		fmt.Println("got error: ", err)
		return nil, err
	}

	dirents := make([]fuse.Dirent, 0)
	fmt.Println("num entries: ", len(res.Entries))
	for i, entry := range res.Entries {
		// TODO: Ensure unique inode numbers.
		entryType := fuse.DT_File
		fmt.Println("entry: ", entry.Name, ", isDir: ", entry.IsDirectory)
		if entry.IsDirectory {
			entryType = fuse.DT_Dir
		}

		dirents = append(dirents,
			fuse.Dirent{Inode: uint64(i + 42), Name: entry.Name, Type: entryType})
	}

	return dirents, nil
}

func (node *Node) ReadAll(ctx context.Context) ([]byte, error) {
	if node.Mode.IsDir() {
		return nil, fuse.Errno(syscall.ENOTDIR)
	}

	node.parent.fuseServer.mu.Lock()
	defer node.parent.fuseServer.mu.Unlock()

	req := &mpb.GetFileRequest{Key: node.name}
	res, err := node.parent.fuseServer.metadataServerClient.GetFile(ctx, req)
	if err != nil {
		return nil, err // TODO(irfansharif): Propagate appropriate FUSE error.
	}

	creq := &cpb.DecryptionRequest{Ciphertext: res.File}
	cres, err := node.parent.fuseServer.cryptServerClient.Decrypt(ctx, creq)
	if err != nil {
		node.parent.fuseServer.logger.Error(err.Error())
		return nil, err // TODO(irfansharif): Propagate appropriate FUSE error.
	}

	return cres.Plaintext, nil
}

func (node *Node) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	if !node.Mode.IsRegular() {
		return fuse.Errno(syscall.EINVAL)
	}

	node.parent.fuseServer.mu.Lock()
	defer node.parent.fuseServer.mu.Unlock()

	creq := &cpb.EncryptionRequest{Plaintext: req.Data}
	cres, err := node.parent.fuseServer.cryptServerClient.Encrypt(ctx, creq)
	if err != nil {
		node.parent.fuseServer.logger.Error(err.Error())
		return err // TODO(irfansharif): Propagate appropriate FUSE error.
	}

	ts := time.Now().Unix()
	metadata := &mpb.FileMetadata{
		Size:            int64(len(req.Data)),
		LastModified:    &mpb.FileMetadata_UnixTimestamp{Seconds: ts},
		ParentDirectory: node.name,
	}

	rq := &mpb.PutFileRequest{
		Key:      node.name,
		File:     []byte(cres.Ciphertext),
		Metadata: metadata,
	}
	_, err = node.parent.fuseServer.metadataServerClient.PutFile(ctx, rq)

	if err != nil {
		return err // TODO(irfansharif): Propagate appropriate FUSE error.
	}

	resp.Size = len(req.Data)
	return nil
}

func (node *Node) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	if !node.Mode.IsDir() {
		return fuse.Errno(syscall.ENOTDIR)
	}

	node.fuseServer.mu.Lock()
	defer node.fuseServer.mu.Unlock()

	if req.Dir {
		rq := &mpb.DeleteFileRequest{Key: req.Name}
		_, err := node.fuseServer.metadataServerClient.DeleteFile(ctx, rq)
		if err != nil {
			return err // TODO(irfansharif): Propagate appropriate FUSE error.
		}
	} else {
		rq := &mpb.DeleteDirectoryRequest{Key: req.Name}
		_, err := node.fuseServer.metadataServerClient.DeleteDirectory(ctx, rq)
		if err != nil {
			return err // TODO(irfansharif): Propagate appropriate FUSE error.
		}
	}

	return nil
}
