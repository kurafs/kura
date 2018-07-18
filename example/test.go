package main

import (
	"fmt"
	"io/ioutil"
	"os"

	metadataserver "github.com/kurafs/kura/cmd/metadata-server"
	"github.com/kurafs/kura/pkg/log"
	mpb "github.com/kurafs/kura/pkg/pb/metadata"
	spb "github.com/kurafs/kura/pkg/pb/storage"
	"google.golang.org/grpc"
)

func main() {
	conn, err := grpc.Dial("localhost:10000", grpc.WithInsecure())
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	defer conn.Close()
	writer := log.MultiWriter(ioutil.Discard, os.Stderr)
	writer = log.SynchronizedWriter(writer)
	logf := log.Ldate | log.Ltime | log.Lmicroseconds | log.Llongfile | log.LUTC | log.Lmode
	logger := log.New(log.Writer(writer), log.Flags(logf), log.SkipBasePath())
	rpcClient := spb.NewStorageServiceClient(conn)
	client := metadataserver.NewStorageClient(rpcClient, logger)
	content, err := client.GetFile("test")
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	fmt.Println(string(content))

	bigContent, err := client.GetFile("Expense_Report_Tips_Intern.pdf")
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	fmt.Println(len(bigContent))

	testBytes := []byte("YOYOYOYOYOYOYOYO")
	resp, err := client.PutFile("testsend", testBytes)
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	fmt.Println(resp)
	testRead, err := client.GetFile("testsend")
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	fmt.Println(string(testRead))

	testDel, err := client.DeleteFile("testsend")
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	fmt.Println(testDel)

	testMetaData1 := mpb.FileMetadata{
		Created:      &mpb.FileMetadata_UnixTimestamp{Seconds: 56565},
		LastModified: &mpb.FileMetadata_UnixTimestamp{Seconds: 3023232},
		Permissions:  "everything",
		Size:         5353,
	}

	testMetaData2 := mpb.FileMetadata{
		Created:      &mpb.FileMetadata_UnixTimestamp{Seconds: 3434},
		LastModified: &mpb.FileMetadata_UnixTimestamp{Seconds: 2424242},
		Permissions:  "nothing",
		Size:         44,
	}

	testMap := map[string]mpb.FileMetadata{
		"hey":        testMetaData1,
		"hey/subhey": testMetaData2,
	}

	testData := metadataserver.MetadataFile{Entries: testMap}
	r, err := client.PutMetadataFile(&testData)
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	fmt.Println(r)
	fmt.Println("asd")

	met, err := client.GetMetadataFile()
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	fmt.Println("111")
	for k, v := range met.Entries {
		fmt.Println("wo")
		fmt.Println(k)
		fmt.Println(v.Created.Seconds)
		fmt.Println(v.LastModified.Seconds)
		fmt.Println(v.Permissions)
		fmt.Println(v.Size)
	}
}
