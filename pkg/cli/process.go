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

package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

// Process is the entry point for CLI commands. User provided arguments are captured and processed
// through the defined commands, and the appropriate one (if any), is executed.
// As is structured at the time of writing, there's no root level command or flags. Given this when
// <program> is invoked without any arguments, the full usage is printed out instead.
//
// All CLI errors are printed out to os.Stderr and follow with os.Exit(2). Command execution errors
// are propagated to the caller. All remaining printed output is directed at os.Stdout.
//
// The abstract is used in generating structured help messages. Example:
//
//      $ <program> -h
//      <abstract>
//
//      Usage of <program>:
//          ...
//
func Process(abstract string, commands Commands) error {
	// We capture the program name and remaining arguments. The prints out the relative path of the
	// binary if that's how the command is invoke, but we don't account for this as the alternative
	// would mean asking the caller to pass in the program name.
	program, args := os.Args[0], os.Args[1:]

	// 	FlagSet outputs are discarded for composability with the rest of this package.
	for _, cmd := range commands {
		cmd.FlagSet.SetOutput(ioutil.Discard)
	}

	// We fall back to printing out default usage when no commands are provided.
	if len(args) == 0 {
		printFullUsage(program, abstract, commands)
		return nil
	}

	command := args[0]
	// We also provide an out of the box '<program> help' command that simply prints out default
	// usage.  '<program> -h' is a special allowance accounted for.
	if (command == "help" || command == "-h") && len(args) == 1 {
		printFullUsage(program, abstract, commands)
		return nil
	}

	// If '<program> help cmd' is used, we ensure there's only one command provided.
	if command == "help" && len(args) > 2 {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("Usage: %s help [command]", program))
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Too many arguments given.")
		os.Exit(2) // failed at 'go help'
	}

	// '<program> help' should also work with every other command (i.e. '<program> help cmd').
	if command == "help" && len(args) == 2 {
		cmd := args[1]
		err := printCommandUsage(program, cmd, commands)
		if err != nil {
			fmt.Fprintln(os.Stderr, fmt.Sprintf("Unknown help topic '%s'", cmd))
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, fmt.Sprintf("Run '%s help' for available topics.", program))
			os.Exit(2) // Failed at '<program> help cmd'.
		}
		return nil
	}

	// A non-help command is executed, we look to find the one provided and if runnable, we run it.
	for _, cmd := range commands {
		if !(cmd.Name() == command) {
			continue
		}
		if !cmd.Runnable() {
			// TODO(irfansharif): Better error message for help topics?
			continue
		}

		// If a custom CLI parse error is returned we can selectively handle that here, and
		// propagate everything else up above.
		err := cmd.Run(cmd, args[1:])
		if _, ok := err.(cmdParseError); !ok {
			return err
		}

		// We handle the flag error where help is requested as a special case as this is a valid
		// state, despite the flag.Parse error response. We also do this after cmd.Run as the flags
		// may have been defined there.
		if strings.Contains(err.Error(), "help requested") {
			printCommandHelp(program, cmd)
			return nil
		}

		// We specifically first print out the flag package/parsing errors.
		printCommandParsingError(program, cmd, err)
		os.Exit(2)
	}

	fmt.Fprintln(os.Stderr, fmt.Sprintf("Unknown command '%s'", command))
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, fmt.Sprintf("Run '%s help' for available commands.\n", program))
	os.Exit(2)
	return nil
}
