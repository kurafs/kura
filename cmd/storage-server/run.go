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

package storageserver

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"

	"github.com/kurafs/kura/pkg/cli"
	"github.com/kurafs/kura/pkg/diskv"
	"github.com/kurafs/kura/pkg/log"
	spb "github.com/kurafs/kura/pkg/pb/storage"
	"google.golang.org/grpc"
)

var StorageServerCmd = &cli.Command{
	Run:       storageServerCmdRun,
	UsageLine: "storage-server [-port port]",
	Short:     "storage-server command overview",
	Long: `
Storage server detailed overview.
    `,
}

func storageServerCmdRun(cmd *cli.Command, args []string) error {
	var port int
	cmd.FlagSet.IntVar(&port, "port", 10669, "Port which the server will run on")
	if err := cmd.FlagSet.Parse(args); err != nil {
		return cli.CmdParseError(err)
	}

	writer := log.MultiWriter(ioutil.Discard, os.Stderr)
	writer = log.SynchronizedWriter(writer)
	logf := log.Ldate | log.Ltime | log.Lmicroseconds | log.Llongfile | log.LUTC | log.Lmode
	logger := log.New(log.Writer(writer), log.Flags(logf), log.SkipBasePath())

	grpcL, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		logger.Fatalf("Failed to open TCP port: %v", err)
		return nil
	}

	store := diskv.New(diskv.Options{
		BasePath:     "kura-store",
		CacheSizeMax: 1024 * 1024, // 1MB cache size
		AdvancedTransform: func(s string) *diskv.PathKey {
			path := strings.Split(s, "/")
			last := len(path) - 1
			return &diskv.PathKey{
				Path:     path[:last],
				FileName: path[last],
			}
		},
		InverseTransform: func(pk *diskv.PathKey) string {
			return strings.Join(pk.Path, "/") + pk.FileName
		},
	})
	storageServer := newStorageServer(logger, store)

	grpcServer := grpc.NewServer()
	spb.RegisterStorageServiceServer(grpcServer, storageServer)

	logger.Infof("Serving on port: %d", port)
	if err := grpcServer.Serve(grpcL); err != nil {
		logger.Fatalf("Failed to serve: %v", err)
	}
	return nil
}
