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

	"github.com/kurafs/kura/pkg/cli"
	"github.com/kurafs/kura/pkg/log"
	pb "github.com/kurafs/kura/pkg/rpc/storage"
	"google.golang.org/grpc"
)

var StorageServerCmd = &cli.Command{
	Run:       storageServerCmdRun,
	UsageLine: "storage-server [-f] [-a arg]",
	Short:     "storage-server command overview",
	Long: `
Storage server detailed overview.
    `,
}

func storageServerCmdRun(cmd *cli.Command, args []string) error {
	var (
		port int
	)
	cmd.FlagSet.IntVar(&port, "port", 10669, "Port which the server will run on")
	if err := cmd.FlagSet.Parse(args); err != nil {
		return cli.CmdParseError(err)
	}

	writer := log.MultiWriter(ioutil.Discard, os.Stderr)
	writer = log.SynchronizedWriter(writer)
	logf := log.Ldate | log.Ltime | log.Lmicroseconds | log.Llongfile | log.LUTC | log.Lmode
	logger := log.New(log.Writer(writer), log.Flags(logf), log.SkipBasePath())

	portConf := fmt.Sprintf(":%d", port)
	lis, err := net.Listen("tcp", portConf)
	if err != nil {
		logger.Fatalf("failed to listen for reason %v", err)
		return nil
	}
	s := grpc.NewServer()
	pb.RegisterStorageServiceServer(s, newServer(logger))
	logger.Infof("serving on port %d\n", port)
	if err := s.Serve(lis); err != nil {
		logger.Fatalf("failed to serve for reason %v", err)
	}
	return nil
}
