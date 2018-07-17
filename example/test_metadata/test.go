package main

import (
	"context"
	"fmt"
	"io"
	"time"

	pb "github.com/kurafs/kura/pkg/rpc/metadata"
	"google.golang.org/grpc"
)

func getFile(c pb.MetadataServiceClient, key string) []byte {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stream, err := c.GetFile(ctx, &pb.GetFileRequest{Key: key})
	if err != nil {
		panic(err)
	}
	fileChunk := make([]byte, 0, 64*1024*1024)
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			return fileChunk
		}
		if err != nil {
			panic(err)
		}
		fileChunk = append(fileChunk, resp.FileChunk...)
	}
}

func putFile(c pb.MetadataServiceClient, file []byte, key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	stream, err := c.PutFile(ctx)
	if err != nil {
		panic(err)
	}
	chunkSize := 64 * 1024
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

		if err := stream.Send(&pb.PutFileRequest{Key: key, FileChunk: file[b:e]}); err != nil {
			panic(err)
		}
	}
	reply, err := stream.CloseAndRecv()
	if err != nil {
		panic(err)
	}
	return reply.Successful
}

func delFile(c pb.MetadataServiceClient, key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := c.DeleteFile(ctx, &pb.DeleteFileRequest{Key: key})
	if err != nil {
		panic(err)
	}
	return res.Successful
}

func getDirKeys(c pb.MetadataServiceClient) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := c.GetDirectoryKeys(ctx, &pb.GetDirectoryKeysRequest{})
	if err != nil {
		panic(err)
	}
	return res.Keys
}

func getMetadata(c pb.MetadataServiceClient, key string) *pb.FileMetadata {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := c.GetMetadata(ctx, &pb.GetMetadataRequest{Key: key})
	if err != nil {
		panic(err)
	}
	return res.Metadata
}

func setMetadata(c pb.MetadataServiceClient, key string, md pb.FileMetadata) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := c.SetMetadata(ctx, &pb.SetMetadataRequest{Key: key, Metadata: &md})
	if err != nil {
		panic(err)
	}
	return res.Successful
}

func main() {
	conn, err := grpc.Dial("localhost:10001", grpc.WithInsecure())
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	defer conn.Close()

	c := pb.NewMetadataServiceClient(conn)

	success := putFile(c, []byte("THIS IS A TEST FILE KEK"), "testfile1")
	if !success {
		panic("fek")
	}
	fmt.Println("wrote a file")

	readSuccess1 := getFile(c, "testfile1")
	fmt.Println(string(readSuccess1))

	deepSuccess := putFile(c, []byte("THIS IS A TEST FILE KEK IN THE SUB DIR"), "subdir/testfile1")
	if !deepSuccess {
		panic("deep fek")
	}

	readDeepSucc := getFile(c, "subdir/testfile1")
	fmt.Println(string(readDeepSucc))

	bigPayload := make([]byte, 1024*1024)
	for i := 0; i < len(bigPayload); i++ {
		bigPayload[i] = 5
	}

	bigSuccess := putFile(c, bigPayload, "bigtest")
	if !bigSuccess {
		panic("big fek")
	}

	readBig := getFile(c, "bigtest")
	fmt.Println(len(readBig))
	fmt.Println(len(readBig) == 50*64*1024*1024)

	pmd := getDirKeys(c)
	fmt.Println(pmd)

}
