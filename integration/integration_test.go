package integration

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	metadataserver "github.com/kurafs/kura/cmd/metadata-server"
	storageserver "github.com/kurafs/kura/cmd/storage-server"
	"github.com/kurafs/kura/pkg/log"
	mpb "github.com/kurafs/kura/pkg/pb/metadata"
	spb "github.com/kurafs/kura/pkg/pb/storage"
	"google.golang.org/grpc"
)

func TestFilePersistence(t *testing.T) {
	logger := log.Discarder()
	tdir, err := ioutil.TempDir("/tmp", "TestFilePersistence")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)

	waitStorage, shutdownStorage, err := storageserver.Start(logger, 10669, tdir)
	if err != nil {
		t.Fatal(err)
	}

	waitMetadata, shutdownMetadata, err := metadataserver.Start(logger, 10670, "localhost:10669")
	if err != nil {
		t.Fatal(err)
	}

	conn, err := grpc.Dial("localhost:10670", grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	key, content := "key", bytes.Repeat([]byte("-"), 1024*1024*2)

	client := mpb.NewMetadataServiceClient(conn)

	preq := &mpb.PutFileRequest{Key: key, File: content}
	_, err = client.PutFile(context.Background(), preq)
	if err != nil {
		t.Fatal(err)
	}

	greq := &mpb.GetFileRequest{Key: key}
	gresp, err := client.GetFile(context.Background(), greq)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(gresp.File, content) {
		t.Fatalf("expected %v, got %v", content, gresp.File)
	}

	mreq := &mpb.GetMetadataRequest{Key: key}
	mresp, err := client.GetMetadata(context.Background(), mreq)
	if err != nil {
		t.Fatal(err)
	}

	if mresp.Metadata.Size != int64(len(content)) {
		t.Fatalf("expected mresp.Metadata.Size = %v, got %v", len(content), mresp.Metadata.Size)
	}

	shutdownStorage()
	shutdownMetadata()

	waitStorage()
	waitMetadata()
}

func TestFileDeletion(t *testing.T) {
	logger := log.Discarder()
	tdir, err := ioutil.TempDir("/tmp", "TestFileDeletion")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)

	waitStorage, shutdownStorage, err := storageserver.Start(logger, 10669, tdir)
	if err != nil {
		t.Fatal(err)
	}

	waitMetadata, shutdownMetadata, err := metadataserver.Start(logger, 10670, "localhost:10669")
	if err != nil {
		t.Fatal(err)
	}

	conn, err := grpc.Dial("localhost:10670", grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	key, content := "key", bytes.Repeat([]byte("-"), 1024*1024*2)
	client := mpb.NewMetadataServiceClient(conn)

	preq := &mpb.PutFileRequest{Key: key, File: content}
	_, err = client.PutFile(context.Background(), preq)
	if err != nil {
		t.Fatal(err)
	}

	greq := &mpb.GetFileRequest{Key: key}
	gresp, err := client.GetFile(context.Background(), greq)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(gresp.File, content) {
		t.Fatalf("expected %v, got %v", content, gresp.File)
	}

	dreq := &mpb.DeleteFileRequest{Key: key}
	_, err = client.DeleteFile(context.Background(), dreq)
	if err != nil {
		t.Fatal(err)
	}

	gresp, err = client.GetFile(context.Background(), greq)
	if err == nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(err.Error(), "no such file or directory") {
		t.Fatalf("expected err '%s', got '%s'", "no such file or directory", err.Error())
	}

	shutdownStorage()
	shutdownMetadata()

	waitStorage()
	waitMetadata()
}

func TestSetMetadata(t *testing.T) {
	logger := log.Discarder()
	tdir, err := ioutil.TempDir("/tmp", "TestSetMetadata")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)

	waitStorage, shutdownStorage, err := storageserver.Start(logger, 10669, tdir)
	if err != nil {
		t.Fatal(err)
	}

	waitMetadata, shutdownMetadata, err := metadataserver.Start(logger, 10670, "localhost:10669")
	if err != nil {
		t.Fatal(err)
	}

	conn, err := grpc.Dial("localhost:10670", grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	key, content := "key", bytes.Repeat([]byte("-"), 1024*1024*2)

	client := mpb.NewMetadataServiceClient(conn)

	preq := &mpb.PutFileRequest{Key: key, File: content}
	_, err = client.PutFile(context.Background(), preq)
	if err != nil {
		t.Fatal(err)
	}

	greq := &mpb.GetFileRequest{Key: key}
	gresp, err := client.GetFile(context.Background(), greq)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(gresp.File, content) {
		t.Fatalf("expected %v, got %v", content, gresp.File)
	}

	now := time.Now()
	metadata := &mpb.FileMetadata{
		Created:      &mpb.FileMetadata_UnixTimestamp{Seconds: now.Unix(), Nanoseconds: 0},
		LastModified: &mpb.FileMetadata_UnixTimestamp{Seconds: now.Unix(), Nanoseconds: 0},
		Permissions:  0600,
		Size:         int64(len(content)),
	}
	msreq := &mpb.SetMetadataRequest{Key: key, Metadata: metadata}
	_, err = client.SetMetadata(context.Background(), msreq)
	if err != nil {
		t.Fatal(err)
	}

	mgreq := &mpb.GetMetadataRequest{Key: key}
	mgresp, err := client.GetMetadata(context.Background(), mgreq)
	if err != nil {
		t.Fatal(err)
	}

	if mgresp.Metadata.Size != metadata.Size {
		t.Fatalf("expected mgresp.Metadata.Size = %v, got %v", metadata.Size, mgresp.Metadata.Size)
	}
	if mgresp.Metadata.Permissions != metadata.Permissions {
		t.Fatalf("expected mgresp.Metadata.Permissions = %v, got %v",
			metadata.Permissions, mgresp.Metadata.Permissions)
	}
	if mgresp.Metadata.Created.Seconds != metadata.Created.Seconds {
		t.Fatalf("expected mgresp.Metadata.Created.Seconds = %v, got %v",
			metadata.Created.Seconds, mgresp.Metadata.Created.Seconds)
	}
	if mgresp.Metadata.LastModified.Seconds != metadata.LastModified.Seconds {
		t.Fatalf("expected mgresp.Metadata.LastModified.Seconds = %v, got %v",
			metadata.Created.Seconds, mgresp.Metadata.LastModified.Seconds)
	}

	shutdownStorage()
	shutdownMetadata()

	waitStorage()
	waitMetadata()
}

func TestDirKeys(t *testing.T) {
	logger := log.Discarder()
	tdir, err := ioutil.TempDir("/tmp", "TestDirKeys")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)

	waitStorage, shutdownStorage, err := storageserver.Start(logger, 10669, tdir)
	if err != nil {
		t.Fatal(err)
	}

	waitMetadata, shutdownMetadata, err := metadataserver.Start(logger, 10670, "localhost:10669")
	if err != nil {
		t.Fatal(err)
	}

	conn, err := grpc.Dial("localhost:10670", grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := mpb.NewMetadataServiceClient(conn)

	for i := 0; i < 10; i++ {
		key, content := fmt.Sprintf("key-%d", i), bytes.Repeat([]byte("-"), 1024*1024*2)
		preq := &mpb.PutFileRequest{Key: key, File: content}
		_, err = client.PutFile(context.Background(), preq)
		if err != nil {
			t.Fatal(err)
		}
	}

	dreq := &mpb.GetDirectoryKeysRequest{}
	dres, err := client.GetDirectoryKeys(context.Background(), dreq)
	if err != nil {
		t.Fatal(err)
	}

	if len(dres.Keys) != 10 {
		t.Fatalf("expected len(dres.Keys)=%d, got %d", 10, len(dres.Keys))
	}
	for i := 0; i < 10; i++ {
		if dres.Keys[i] != fmt.Sprintf("key-%d", i) {
			t.Fatalf("expected %s, got %s", fmt.Sprintf("key-%d", i), dres.Keys[i])
		}
	}

	shutdownStorage()
	shutdownMetadata()

	waitStorage()
	waitMetadata()
}

func TestGarbageCollection(t *testing.T) {
	logger := log.Discarder()
	tdir, err := ioutil.TempDir("/tmp", "TestDirKeys")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)

	waitStorage, shutdownStorage, err := storageserver.Start(logger, 10669, tdir)
	if err != nil {
		t.Fatal(err)
	}

	waitMetadata, shutdownMetadata, err := metadataserver.Start(logger, 10670, "localhost:10669")
	if err != nil {
		t.Fatal(err)
	}

	conn, err := grpc.Dial("localhost:10670", grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := mpb.NewMetadataServiceClient(conn)

	for i := 0; i < 10; i++ {
		key, content := fmt.Sprintf("key-%d", i), bytes.Repeat([]byte("-"), 1024*1024*2)
		preq := &mpb.PutFileRequest{Key: key, File: content}
		_, err = client.PutFile(context.Background(), preq)
		if err != nil {
			t.Fatal(err)
		}
	}

	dreq := &mpb.GetDirectoryKeysRequest{}
	dres, err := client.GetDirectoryKeys(context.Background(), dreq)
	if err != nil {
		t.Fatal(err)
	}

	if len(dres.Keys) != 10 {
		t.Fatalf("expected len(dres.Keys)=%d, got %d", 10, len(dres.Keys))
	}
	for i := 0; i < 10; i++ {
		if dres.Keys[i] != fmt.Sprintf("key-%d", i) {
			t.Fatalf("expected %s, got %s", fmt.Sprintf("key-%d", i), dres.Keys[i])
		}
	}

	storageConn, err := grpc.Dial("localhost:10669", grpc.WithInsecure())
	defer storageConn.Close()
	storageClient := spb.NewStorageServiceClient(storageConn)

	// there should be 15 files in the storage layer, 5 of which
	// do not appear recorded in the metadata server (invalid)
	for i := 11; i < 15; i++ {
		key, content := fmt.Sprintf("key-%d", i), bytes.Repeat([]byte("-"), 1024*1024*2)
		sreq := &spb.PutFileRequest{Key: key, File: content}
		_, err = storageClient.PutFile(context.Background(), sreq)
		if err != nil {
			t.Fatal(err)
		}
	}

	sreq := &spb.GetFileKeysRequest{}
	sres, err := storageClient.GetFileKeys(context.Background(), sreq)
	if err != nil {
		t.Fatal(err)
	}

	if len(sres.Keys) != 15 {
		t.Fatalf("expected len(sres.Keys)=%d, got %d", 15, len(sres.Keys))
	}

	gcReq := &mpb.ForceGarbageCollectionRequest{}
	_, err = client.ForceGarbageCollection(context.Background(), gcReq)
	if err != nil {
		t.Fatal(err)
	}

	sreq = &spb.GetFileKeysRequest{}
	sres, err = storageClient.GetFileKeys(context.Background(), sreq)
	if err != nil {
		t.Fatal(err)
	}

	// the 5 unsynchronized files in the storage layer should be eradicated
	// total 11 files (10 original that were synchronized + metadata file)
	if len(sres.Keys) != 11 {
		t.Fatalf("expected len(sres.Keys)=%d, got %d", 11, len(sres.Keys))
	}

	shutdownStorage()
	shutdownMetadata()

	waitStorage()
	waitMetadata()
}

func TestNestedFilePersistence(t *testing.T) {
	t.Skip("TODO(irfansharif): Unsupported.")
}

func TestLargeFilePersistence(t *testing.T) {
	t.Skip("TODO(irfansharif): gRPC exhaustion occurs at 4 MiB, add streaming.")
}
