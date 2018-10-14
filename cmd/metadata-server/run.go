// Copyright 2018 The Kura Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metadataserver

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/kurafs/kura/pkg/cli"
	"github.com/kurafs/kura/pkg/log"
	mpb "github.com/kurafs/kura/pkg/pb/metadata"
	spb "github.com/kurafs/kura/pkg/pb/storage"
	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"
)

var MetadataServerCmd = &cli.Command{
	Run:       metadataServerCmdRun,
	UsageLine: "metadata-server [-storage-addr addr] [-port port]",
	Short:     "metadata-server command overview",
	Long: `
Metadata server detailed overview.
    `,
}

func metadataServerCmdRun(cmd *cli.Command, args []string) error {
	var (
		port        int
		storageAddr string
	)
	cmd.FlagSet.StringVar(&storageAddr, "storage-addr", "localhost:10669", "Address of the storage server [host:port]")
	cmd.FlagSet.IntVar(&port, "port", 10670, "Port on which the server will run on")
	if err := cmd.FlagSet.Parse(args); err != nil {
		return cli.CmdParseError(err)
	}

	writer := log.MultiWriter(ioutil.Discard, os.Stderr)
	writer = log.SynchronizedWriter(writer)
	logf := log.Ldate | log.Ltime | log.Lmicroseconds | log.Llongfile | log.LUTC | log.Lmode
	logger := log.New(log.Writer(writer), log.Flags(logf), log.SkipBasePath())

	wait, shutdown, err := Start(logger, port, storageAddr)
	if err != nil {
		return err
	}

	wait()
	shutdown()

	return nil
}

// TODO(irfansharif): Maybe use different type for storageAddr, to ensure both
// host/port.
func Start(logger *log.Logger, port int, storageAddr string) (wait func(), shutdown func(), err error) {
	var wg sync.WaitGroup

	lis, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		logger.Fatalf("failed to open TCP port: %v", err)
		return nil, nil, err
	}

	// Create a cmux; multiplex grpc and http over the same listener.
	mux := cmux.New(lis)

	// Match connections in order: First grpc, then everything else for web.
	//
	// TODO(irfansharif): This is finicky when working with TLS; check what
	// cockroachdb/cockroach does.
	grpcL := mux.Match(cmux.HTTP2HeaderField("content-type", "application/grpc"))
	httpL := mux.Match(cmux.Any())

	// Setup grpc connection with the storage server.
	// TODO(irfansharif): This is currently over an insecure connection, needs
	// to be secure.
	storageConn, err := grpc.Dial(storageAddr, grpc.WithInsecure())
	if err != nil {
		return nil, nil, err
	}

	storageClient := spb.NewStorageServiceClient(storageConn)
	metadataServer := newMetadataServer(logger, storageClient)

	grpcServer := grpc.NewServer()
	mpb.RegisterMetadataServiceServer(grpcServer, metadataServer)

	httpServer := http.Server{Handler: grpcweb.WrapServer(grpcServer)}

	wg.Add(1)
	go func() {
		defer wg.Done()

		logger.Infof("serving RPC server on port: %d", port)
		if err := grpcServer.Serve(grpcL); err != nil {
			logger.Errorf("grpc server error: %v", err)
		}

	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		logger.Infof("serving HTTP server on port: %d", port)
		if err := httpServer.Serve(httpL); err != nil {
			logger.Errorf("http server error: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := mux.Serve(); err != nil {
			logger.Errorf("cmux server error: %v", err)
		}
	}()

	shutdown = func() {
		storageConn.Close()
		lis.Close()
		grpcServer.Stop()
		httpServer.Shutdown(context.Background())
	}

	return wg.Wait, shutdown, nil
}
