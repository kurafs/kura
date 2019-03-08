package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kurafs/kura/pkg/streaming"

	cryptserver "github.com/kurafs/kura/cmd/crypt-server"
	identityserver "github.com/kurafs/kura/cmd/identity-server"
	metadataserver "github.com/kurafs/kura/cmd/metadata-server"
	storageserver "github.com/kurafs/kura/cmd/storage-server"
	"github.com/kurafs/kura/pkg/log"
	cpb "github.com/kurafs/kura/pkg/pb/crypt"
	ipb "github.com/kurafs/kura/pkg/pb/identity"
	mpb "github.com/kurafs/kura/pkg/pb/metadata"
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
	metadata := &mpb.Metadata{
		Size: int64(len(content)),
	}

	client := mpb.NewMetadataServiceClient(conn)

	preq := &mpb.PutFileRequest{Path: key, File: content, Metadata: metadata}
	_, err = client.PutFile(context.Background(), preq)
	if err != nil {
		t.Fatal(err)
	}

	greq := &mpb.GetFileRequest{Path: key}
	gresp, err := client.GetFile(context.Background(), greq)
	if err != nil {
		t.Fatal(err)
	}

	if gresp.Metadata.Size != int64(len(content)) {
		t.Fatalf("expected gresp.Metadata.Size = %v, got %v", len(content), gresp.Metadata.Size)
	}

	if !bytes.Equal(gresp.File, content) {
		t.Fatalf("expected %v, got %v", content, gresp.File)
	}

	mreq := &mpb.GetMetadataRequest{Path: key}
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
	metadata := &mpb.Metadata{
		Size: int64(len(content)),
	}

	client := mpb.NewMetadataServiceClient(conn)

	preq := &mpb.PutFileRequest{Path: key, File: content, Metadata: metadata}
	_, err = client.PutFile(context.Background(), preq)
	if err != nil {
		t.Fatal(err)
	}

	greq := &mpb.GetFileRequest{Path: key}
	gresp, err := client.GetFile(context.Background(), greq)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(gresp.File, content) {
		t.Fatalf("expected %v, got %v", content, gresp.File)
	}

	dreq := &mpb.DeleteFileRequest{Path: key}
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

	now := time.Now()
	metadata := &mpb.Metadata{
		Created:      &mpb.Metadata_UnixTimestamp{Seconds: now.Unix(), Nanoseconds: 0},
		LastModified: &mpb.Metadata_UnixTimestamp{Seconds: now.Unix(), Nanoseconds: 0},
		Permissions:  0600,
		Size:         int64(len(content)),
	}

	client := mpb.NewMetadataServiceClient(conn)

	preq := &mpb.PutFileRequest{Path: key, File: content, Metadata: metadata}
	_, err = client.PutFile(context.Background(), preq)
	if err != nil {
		t.Fatal(err)
	}

	greq := &mpb.GetFileRequest{Path: key}
	gresp, err := client.GetFile(context.Background(), greq)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(gresp.File, content) {
		t.Fatalf("expected %v, got %v", content, gresp.File)
	}

	if gresp.Metadata.Size != metadata.Size {
		t.Fatalf("expected mgresp.Metadata.Size = %v, got %v", metadata.Size, gresp.Metadata.Size)
	}
	if gresp.Metadata.Permissions != metadata.Permissions {
		t.Fatalf("expected mgresp.Metadata.Permissions = %v, got %v",
			metadata.Permissions, gresp.Metadata.Permissions)
	}
	if gresp.Metadata.Created.Seconds != metadata.Created.Seconds {
		t.Fatalf("expected mgresp.Metadata.Created.Seconds = %v, got %v",
			metadata.Created.Seconds, gresp.Metadata.Created.Seconds)
	}
	if gresp.Metadata.LastModified.Seconds != metadata.LastModified.Seconds {
		t.Fatalf("expected mgresp.Metadata.LastModified.Seconds = %v, got %v",
			metadata.Created.Seconds, gresp.Metadata.LastModified.Seconds)
	}

	msreq := &mpb.SetMetadataRequest{Path: key, Metadata: metadata}
	_, err = client.SetMetadata(context.Background(), msreq)
	if err != nil {
		t.Fatal(err)
	}

	mgreq := &mpb.GetMetadataRequest{Path: key}
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

	set := make(map[string]struct{})
	for i := 0; i < 10; i++ {
		key, content := fmt.Sprintf("key-%d", i), bytes.Repeat([]byte("-"), 1024*1024*2)
		set[key] = struct{}{}
		metadata := &mpb.Metadata{
			Size: int64(len(content)),
		}
		preq := &mpb.PutFileRequest{Path: "/" + key, File: content, Metadata: metadata}
		_, err = client.PutFile(context.Background(), preq)
		if err != nil {
			t.Fatal(err)
		}
	}

	dreq := &mpb.GetDirectoryEntriesRequest{Path: ""}
	dres, err := client.GetDirectoryEntries(context.Background(), dreq)
	if err != nil {
		t.Fatal(err)
	}

	if len(dres.Entries) != 10 {
		t.Fatalf("expected len(dres.Entries)=%d, got %d", 10, len(dres.Entries))
	}
	for i := 0; i < 10; i++ {
		_, ok := set[dres.Entries[i].Path]
		if !ok {
			t.Fatalf("%s not found in set", dres.Entries[i])
		}
		delete(set, dres.Entries[i].Path)
	}

	if len(set) != 0 {
		t.Fatal("not all files were retrieved")
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
	logger := log.Discarder()
	tdir, err := ioutil.TempDir("/tmp", "TestLargeFilePersistence")
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

	testContent := bytes.Repeat([]byte("abcd"), (streaming.Threshold/4)+1)

	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key-%d", i)
		metadata := &mpb.Metadata{
			Size: int64(len(testContent)),
		}
		chunker := streaming.NewChunker(testContent)
		stream, err := client.PutFileStream(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		chunker.Next()
		first := chunker.Value()
		if err = stream.Send(&mpb.PutFileStreamRequest{Path: key, Chunk: first, Metadata: metadata}); err != nil {
			t.Fatal(err)
		}
		for chunker.Next() {
			preq := &mpb.PutFileStreamRequest{Chunk: chunker.Value()}
			err = stream.Send(preq)
			if err != nil {
				t.Fatal(err)
			}
		}
		if _, err = stream.CloseAndRecv(); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key-%d", i)
		stream, err := client.GetFileStream(context.Background(), &mpb.GetFileStreamRequest{Path: key})
		if err != nil {
			t.Fatal(err)
		}
		first, err := stream.Recv()
		if err != nil {
			t.Fatal(err)
		}
		buff := make([]byte, 0, len(testContent))
		if first.Metadata == nil {
			t.Fatal("First message had no metadata")
		}
		buff = append(buff, first.Chunk...)
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			buff = append(buff, in.Chunk...)
		}
		t.Log(len(buff), len(testContent))
		if !bytes.Equal(buff, testContent) {
			t.Fatal("Did not retrieve correct data")
		}
	}

	shutdownStorage()
	shutdownMetadata()

	waitStorage()
	waitMetadata()
}

func TestCrypt(t *testing.T) {
	logger := log.Discarder()
	tdir, err := ioutil.TempDir("/tmp", "TestCrypt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)

	seed := "figar-bazor-vizob-jituh-bilif-silul-fahur-kabuv"
	var Xexpected, Yexpected big.Int
	Xexpected.SetString(
		"6054DF38F8E7531BE3D68554F8F07B759DAEC1AAEA5C0717E9D2C45921F0EE90", 16)
	Yexpected.SetString(
		"4BC16B1DE3F3AF6989B8124F6C8D236AF62E2859DBA455A6B9A3F0CF45624394", 16)

	if err := cryptserver.GenerateAndWriteKeys(logger, seed, tdir); err != nil {
		t.Fatal(err)
	}

	waitCrypt, shutdownCrypt, err := cryptserver.Start(logger, 10870, tdir)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := grpc.Dial("localhost:10870", grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := cpb.NewCryptServiceClient(conn)
	preq := &cpb.PublicKeyRequest{}
	pres, err := client.PublicKey(context.Background(), preq)
	if err != nil {
		t.Fatal(err)
	}

	var X, Y big.Int
	if err := X.UnmarshalText(pres.X); err != nil {
		t.Fatal(err)
	}
	if err := Y.UnmarshalText(pres.Y); err != nil {
		t.Fatal(err)
	}

	if X.Cmp(&Xexpected) != 0 {
		t.Fatal(fmt.Sprintf("got X = %s, expected %s",
			X.String(), Xexpected.String()))
	}
	if Y.Cmp(&Yexpected) != 0 {
		t.Fatal(fmt.Sprintf("got Y = %s, expected %s",
			Y.String(), Yexpected.String()))
	}

	shutdownCrypt()
	waitCrypt()
}

func TestEncryptAES(t *testing.T) {
	logger := log.Discarder()
	tdir, err := ioutil.TempDir("/tmp", "TestEncryptAES")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)

	seed := "figar-bazor-vizob-jituh-bilif-silul-fahur-kabuv"
	if err := cryptserver.GenerateAndWriteKeys(logger, seed, tdir); err != nil {
		t.Fatal(err)
	}

	waitCrypt, shutdownCrypt, err := cryptserver.Start(logger, 10870, tdir)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := grpc.Dial("localhost:10870", grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	prevRandReader := rand.Reader
	defer func() {
		rand.Reader = prevRandReader
	}()
	rand.Reader = bytes.NewReader(bytes.Repeat([]byte("="), 16))
	client := cpb.NewCryptServiceClient(conn)
	ereq := &cpb.EncryptionRequest{
		Plaintext: []byte("plaintext"),
		AesKey:    []byte("aes-key-12345678"),
	}
	eres, err := client.Encrypt(context.Background(), ereq)
	if err != nil {
		t.Fatal(err)
	}

	expectedEncodedCiphertext := "PT09PT09PT09PT09PT09PShTfHeVLm0dzw=="
	gotEncodedCiphertext := base64.URLEncoding.EncodeToString(eres.Ciphertext)

	if expectedEncodedCiphertext != gotEncodedCiphertext {
		t.Fatalf("expected ciphertext %s, got %s",
			expectedEncodedCiphertext, gotEncodedCiphertext)
	}

	shutdownCrypt()
	waitCrypt()
}

func TestDecryptAES(t *testing.T) {
	logger := log.Discarder()
	tdir, err := ioutil.TempDir("/tmp", "TestDecryptAES")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)

	seed := "figar-bazor-vizob-jituh-bilif-silul-fahur-kabuv"
	if err := cryptserver.GenerateAndWriteKeys(logger, seed, tdir); err != nil {
		t.Fatal(err)
	}

	waitCrypt, shutdownCrypt, err := cryptserver.Start(logger, 10870, tdir)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := grpc.Dial("localhost:10870", grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	prevRandReader := rand.Reader
	defer func() {
		rand.Reader = prevRandReader
	}()
	rand.Reader = bytes.NewReader(bytes.Repeat([]byte("="), 16))

	encodedCiphertext := "PT09PT09PT09PT09PT09PShTfHeVLm0dzw=="
	ciphertext, err := base64.URLEncoding.DecodeString(encodedCiphertext)
	if err != nil {
		t.Fatal(err)
	}

	client := cpb.NewCryptServiceClient(conn)
	dreq := &cpb.DecryptionRequest{
		Ciphertext: ciphertext,
		AesKey:     []byte("aes-key-12345678"),
	}
	dres, err := client.Decrypt(context.Background(), dreq)
	if err != nil {
		t.Fatal(err)
	}

	if string(dres.Plaintext) != "plaintext" {
		t.Fatalf("expected plaintext '%s', got %s",
			"plaintext", string(dres.Plaintext))
	}

	shutdownCrypt()
	waitCrypt()
}

func TestIdentityServer(t *testing.T) {
	logger := log.Discarder()
	tdir, err := ioutil.TempDir("/tmp", "TestIdentityServer")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)

	wait, shutdown, err := identityserver.Start(logger, 10770, tdir)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := grpc.Dial("localhost:10770", grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := ipb.NewIdentityServiceClient(conn)
	preq := &ipb.PutKeyRequest{
		Email:     "test@email.com",
		PublicKey: []byte("public-key"),
	}
	_, err = client.PutPublicKey(context.Background(), preq)
	if err != nil {
		t.Fatal(err)
	}

	greq := &ipb.GetKeyRequest{
		Email: "test@email.com",
	}
	gres, err := client.GetPublicKey(context.Background(), greq)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(gres.PublicKey, preq.PublicKey) {
		t.Fatal(errors.New("got incorrect public key"))
	}

	shutdown()
	wait()
}

func TestIdentityServerMissingUser(t *testing.T) {
	logger := log.Discarder()
	tdir, err := ioutil.TempDir("/tmp", "TestIdentityServer")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)

	wait, shutdown, err := identityserver.Start(logger, 10770, tdir)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := grpc.Dial("localhost:10770", grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := ipb.NewIdentityServiceClient(conn)
	greq := &ipb.GetKeyRequest{
		Email: "test@email.com",
	}
	_, err = client.GetPublicKey(context.Background(), greq)
	if err == nil {
		t.Fatal("expected 'user not found' error")
	}

	shutdown()
	wait()
}

func TestIdentityServerOverwriteRejected(t *testing.T) {
	logger := log.Discarder()
	tdir, err := ioutil.TempDir("/tmp", "TestIdentityServer")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)

	wait, shutdown, err := identityserver.Start(logger, 10770, tdir)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := grpc.Dial("localhost:10770", grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := ipb.NewIdentityServiceClient(conn)
	preq := &ipb.PutKeyRequest{
		Email:     "test@email.com",
		PublicKey: []byte("public-key"),
	}
	_, err = client.PutPublicKey(context.Background(), preq)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.PutPublicKey(context.Background(), preq)
	if err == nil {
		t.Fatal("expected error due to overwrite")
	}

	shutdown()
	wait()
}
