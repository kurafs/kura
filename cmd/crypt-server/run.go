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

package cryptserver

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"os"
	"path"
	"sync"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/kurafs/kura/pkg/cli"
	"github.com/kurafs/kura/pkg/log"
	cpb "github.com/kurafs/kura/pkg/pb/crypt"
	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"
)

var CryptServerCmd = &cli.Command{
	Run:       cryptServerCmdRun,
	UsageLine: "crypt-server [-gen-keys [-seed seed]] [-keys-dir <keys-dir>] [-port port]",
	Short:     "run the crypt-server, generate public/private key pair",
	Long: `
TODO: Crypt server detailed overview..
    `,
}

func cryptServerCmdRun(cmd *cli.Command, args []string) error {
	var (
		genKeysFlag bool
		seedFlag    string
		keysDirFlag string
		portFlag    int

		logDirFlag         string
		suppressStderrFlag bool
		logModeFlag        logMode
		logFilterFlag      logFilter
		backtracePointFlag backtracePoints
	)

	cmd.FlagSet.BoolVar(&genKeysFlag, "gen-keys", false,
		"Generate public/private keys at specified directory")
	cmd.FlagSet.StringVar(&seedFlag, "seed", "",
		"Secret seed containing a 128-bit secret in proquint format")
	cmd.FlagSet.StringVar(&keysDirFlag, "keys-dir", "kura-keys",
		"Filepath to store public/private key pair")
	cmd.FlagSet.IntVar(&portFlag, "port", 10870, "Port on which the server will run on")

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

	if cmd.FlagSet.NArg() > 0 {
		return cli.CmdParseError(
			errors.New(fmt.Sprintf("unrecognized arguments: %v",
				cmd.FlagSet.Args()[1:])))
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

	// TODO(irfansharif): Deal with the case where directory is present, but
	// files are not.
	_, err := os.Stat(keysDirFlag)
	keysExist := !os.IsNotExist(err)

	if genKeysFlag {
		if keysExist {
			errm := fmt.Sprintf("keys-dir %s already exists", keysDirFlag)
			logger.Errorf(errm)
			return errors.New(errm)
		}
		if err := GenerateAndWriteKeys(logger, seedFlag, keysDirFlag); err != nil {
			logger.Error(err.Error())
			return err
		}

		return nil
	}

	if !keysExist {
		errm := fmt.Sprintf("keys-dir %s not found", keysDirFlag)
		logger.Errorf(errm)
		return errors.New(errm)
	}

	wait, shutdown, err := Start(logger, portFlag, keysDirFlag)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	wait()
	shutdown()

	return nil
}

func Start(logger *log.Logger, port int, keysDir string) (
	wait func(), shutdown func(), err error,
) {
	pubf, err := ioutil.ReadFile(path.Join(keysDir, "public-key.kura"))
	if err != nil {
		return nil, nil, err
	}
	pubk := bytes.Split(pubf, []byte("\n"))

	prif, err := ioutil.ReadFile(path.Join(keysDir, "private-key.kura"))
	if err != nil {
		return nil, nil, err
	}

	var X, Y, D big.Int
	if err := X.UnmarshalText(pubk[0]); err != nil {
		return nil, nil, err
	}
	if err := Y.UnmarshalText(pubk[1]); err != nil {
		return nil, nil, err
	}
	if err := D.UnmarshalText(prif); err != nil {
		return nil, nil, err
	}

	priv := new(ecdsa.PrivateKey)
	priv.PublicKey.Curve = elliptic.P256()
	priv.D = &D
	priv.PublicKey.X, priv.PublicKey.Y = &X, &Y

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

	cryptServer := newCryptServer(logger, priv)

	grpcServer := grpc.NewServer()
	cpb.RegisterCryptServiceServer(grpcServer, cryptServer)

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
	}

	return wg.Wait, shutdown, nil
}
