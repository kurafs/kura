// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in licenses/BSD-golang.txt.

// Portions of this file are additionally subject to the following
// license and copyright.
//
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

// Portions of this code originated in the Go source code, under cmd/go/internal/base.

package cli

import (
	"flag"
	"strings"
)

// A Command is an implementation of a CLI command like '<program> key-server' or '<program>
// directory-server ...'.
// If Command.Run is nil, it's a documentation pseudo-command accessible only via '<program> help
// [topic]'.
type Command struct {
	// Run runs the command. The args are the arguments after the command name, to be parsed as
	// needed using cmd.FlagSet. In case of flag parsing error (due to incorrect flags being
	// provided, return CmdParseError(error) for output composability.
	Run func(cmd *Command, args []string) error

	// UsageLine is the one-line usage message.
	//
	// NB: The first word in the line is taken to be the command name.
	UsageLine string

	// Short is the short description shown in the '<program> help' output.
	Short string

	// Long is the long description shown in the '<program> help <command>' output.
	Long string

	// FlagSet is the set of flags specific to the command. This can be provided when constructing
	// the Command struct or used the provided Run implementation to parse the command-line args.
	//
	// NB: FlagSet output is discarded for composability with the rest of this package.
	FlagSet flag.FlagSet
}

type Commands []*Command

// Name returns the command's name: the first word in the usage line.
func (c *Command) Name() string {
	name := c.UsageLine
	i := strings.Index(name, " ")
	if i >= 0 {
		name = name[:i]
	}
	return name
}

// Runnable reports whether the command can be run; otherwise it is a documentation pseudo-command
// such as 'security-model'.
func (c *Command) Runnable() bool {
	return c.Run != nil
}

type cmdParseError error

func CmdParseError(err error) error {
	return err.(cmdParseError)
}
