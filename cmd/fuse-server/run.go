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

package fuseserver

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/kurafs/kura/pkg/cli"
	"github.com/kurafs/kura/pkg/fuse"
	"github.com/kurafs/kura/pkg/fuse/fs"
	"github.com/kurafs/kura/pkg/log"
)

var FuseServerCmd = &cli.Command{
	Run:       fuseServerCmdRun,
	UsageLine: "fuse-server [-metadata-server addr] [-crypt-server addr] [-unmount] [logger flags] <mount-point>",
	Short:     "run the fuse server, mounting kura at specified mount point",
	Long: `
Fuse server detailed overview. TODO.
    `,
}

func fuseServerCmdRun(cmd *cli.Command, args []string) error {
	var (
		metadataServerFlag string
		cryptServerFlag    string
		unmountFlag        bool

		logDirFlag         string
		suppressStderrFlag bool
		logModeFlag        logMode
		logFilterFlag      logFilter
		backtracePointFlag backtracePoints
	)

	cmd.FlagSet.StringVar(&metadataServerFlag, "metadata-server", "localhost:10670",
		"Address of the metadata server [host:port]")
	cmd.FlagSet.StringVar(&cryptServerFlag, "crypt-server", "localhost:10870",
		"Address of the metadata server [host:port]")
	cmd.FlagSet.BoolVar(&unmountFlag, "unmount", false,
		"Unmount filesystem at specified directory")
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

	if cmd.FlagSet.NArg() > 1 {
		return cli.CmdParseError(
			errors.New(fmt.Sprintf("unrecognized arguments: %v", cmd.FlagSet.Args()[1:])))
	}
	if cmd.FlagSet.NArg() == 0 {
		return cli.CmdParseError(errors.New("unspecified mount-point"))
	}
	mountPoint := cmd.FlagSet.Arg(0)

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

	if unmountFlag {
		if err := unmount(logger, mountPoint); err != nil {
			logger.Error(err.Error())
			return err
		}

		return nil
	}

	conn, err := mount(logger, mountPoint)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	defer conn.Close()

	fuseServer, err := newFUSEServer(logger, metadataServerFlag, cryptServerFlag)
	if err != nil {
		return err
	}

	if err := fs.Serve(conn, fuseServer); err != nil {
		return err
	}

	return nil
}

func unmount(logger *log.Logger, mountPoint string) error {
	if err := fuse.Unmount(mountPoint); err != nil {
		return err
	}
	logger.Infof("unmounted point: %s", mountPoint)
	return nil
}

func mount(logger *log.Logger, mountPoint string) (*fuse.Conn, error) {
	conn, err := fuse.Mount(
		mountPoint,
		fuse.FSName("kurafs"),
		fuse.VolumeName("Kura"),
		fuse.NoAppleDouble(),
		fuse.NoAppleXattr(),
		fuse.WritebackCache(),
	)
	if err != nil {
		return nil, err
	}

	logger.Infof("mounted point: %s", mountPoint)
	return conn, nil
}
