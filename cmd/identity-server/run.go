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

package identityserver

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/kurafs/kura/pkg/cli"
	"github.com/kurafs/kura/pkg/log"
	ipb "github.com/kurafs/kura/pkg/pb/identity"
	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"
)

var IdentityServerCmd = &cli.Command{
	Run:       identityServerCmdRun,
	UsageLine: "identity-server [-db-store <path>] [-port port]",
	Short:     "identity-server command overview",
	Long: `
Identity server detailed overview.
    `,
}

func identityServerCmdRun(cmd *cli.Command, args []string) error {
	var (
		port    int
		dbStore string

		logDirFlag         string
		suppressStderrFlag bool
		logModeFlag        logMode
		logFilterFlag      logFilter
		backtracePointFlag backtracePoints
	)
	cmd.FlagSet.IntVar(&port, "port", 10770, "Port which the server will run on")
	cmd.FlagSet.StringVar(&dbStore, "db-store", "kura-idb",
		"Folder to store database records to")

	cmd.FlagSet.StringVar(&logDirFlag, "log-dir", "",
		"Write log files to the specified directory")
	cmd.FlagSet.BoolVar(&suppressStderrFlag, "suppress-stderr", false,
		"Suppress standard error logging")
	cmd.FlagSet.Var(&logModeFlag, "log-mode",
		"Log mode for logs emitted globally (can be overridden using -log-filter)")
	cmd.FlagSet.Var(&logFilterFlag, "log-filter",
		"Comma-separated list of pattern:level settings for file-filtered logging")
	cmd.FlagSet.Var(&backtracePointFlag, "log-backtrace-at",
		"Comma-separated list of filename:N settings to emit backtraces")

	if err := cmd.FlagSet.Parse(args); err != nil {
		return cli.CmdParseError(err)
	}

	if logModeFlag.set {
		log.SetGlobalLogMode(log.Mode(logModeFlag.m))
	}
	for _, flm := range logFilterFlag {
		log.SetFileLogMode(flm.fname, flm.fmode)
	}
	for _, tp := range backtracePointFlag {
		log.SetTracePoint(tp)
	}

	writer := ioutil.Discard
	if logDirFlag != "" {
		writer = log.LogRotationWriter(logDirFlag, 50<<20 /* 50 MiB */)
	}
	if !suppressStderrFlag {
		writer = log.MultiWriter(writer, os.Stderr)
	}
	writer = log.SynchronizedWriter(writer)
	logf := log.Ldate | log.Ltime | log.Lmicroseconds | log.Llongfile | log.LUTC | log.Lmode
	logger := log.New(log.Writer(writer), log.Flags(logf), log.SkipBasePath())

	wait, shutdown, err := Start(logger, port, dbStore)
	if err != nil {
		return err
	}

	wait()
	shutdown()

	return nil
}

func Start(logger *log.Logger, port int, dbStore string) (wait func(), shutdown func(), err error) {
	if err := os.MkdirAll(dbStore, 0700); os.IsNotExist(err) {
		// Ignore.
	} else if err != nil {
		return nil, nil, err
	}

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

	db, err := bolt.Open(path.Join(dbStore, "identity.db"), 0600, nil)
	if err != nil {
		return nil, nil, err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("user-keys"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	identityServer := newIdentityServer(logger, db)
	grpcServer := grpc.NewServer()
	ipb.RegisterIdentityServiceServer(grpcServer, identityServer)
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
		lis.Close()
		grpcServer.Stop()
		httpServer.Shutdown(context.Background())
		grpcL.Close()
		db.Close()
	}

	return wg.Wait, shutdown, nil
}
