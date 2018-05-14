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

// Portions of this code originated in the Go source code, under cmd/go/internal/help.

package cli

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"os"
	"strings"
	"unicode"
)

var usageTemplate = `{{abstract}}

Usage:

    {{program}} command [arguments]

The commands are:
{{range .}}{{if .Runnable}}
	{{.Name | printf "%-20s"}}   {{.Short}}{{end}}{{end}}

Use '{{program}} help [command]' for more information about a command.

Additional help topics:
{{range .}}{{if not .Runnable}}
	{{.Name | printf "%-20s"}}   {{.Short}}{{end}}{{end}}

Use "{{program}} help [topic]" for more information about that topic.
`

var helpTemplate = `{{if .Runnable}}Usage: {{program}} {{.UsageLine}}

{{else}}Topic: {{.Short}}

{{end}}{{.Long | trim}}
`

var cmdErrorHelpTemplate = `Usage: 

  {{program}} {{.UsageLine}}

`

// tmpl executes the given template text on data, writing the result to w.
func tmpl(w io.Writer, templateText, program, abstract string, data interface{}) {
	t := template.New("")
	t.Funcs(template.FuncMap{
		"trim":     strings.TrimSpace,
		"abstract": func() string { return abstract },
		"program":  func() string { return program },
	})
	template.Must(t.Parse(templateText))
	if err := t.Execute(w, data); err != nil {
		panic(err)
	}
}

// printFullUsage prints out to os.Stdout the entire top level usage, including the program abstract,
// all commands and help topics.
func printFullUsage(program, abstract string, commands Commands) {
	tmpl(os.Stdout, usageTemplate, program, abstract, commands)
}

// printCommandUsage prints out to os.Stdout the help output for the given command. This is what's
// visible when '<program> help command' is invoked.
func printCommandUsage(program, command string, commands Commands) error {
	for _, cmd := range commands {
		if cmd.Name() == command {
			tmpl(os.Stdout, helpTemplate, program, "", cmd)
			return nil
		}
	}

	return errors.New("command not found")
}

// printCommandParsingError prints out to os.Stderr the command flags parsing error along with brief
// usage information.
func printCommandParsingError(program string, cmd *Command, err error) {
	if !strings.Contains(err.Error(), "help requested") {
		fmt.Fprintln(os.Stderr, upcaseInitial(err.Error()))
	}
	tmpl(os.Stderr, cmdErrorHelpTemplate, program, "", cmd)
	cmd.FlagSet.SetOutput(os.Stderr)
	cmd.FlagSet.PrintDefaults()
}

// printCommandHelp prints out to os.Stdout the default flags for the given command. This is what's
// visible when '<program> command -h' is invoked.
func printCommandHelp(program string, cmd *Command) {
	tmpl(os.Stdout, cmdErrorHelpTemplate, program, "", cmd)
	cmd.FlagSet.SetOutput(os.Stderr)
	cmd.FlagSet.PrintDefaults()
}

// HACK: Unicode points (runes) can consist of multiple bytes. This implementation exits on encountering
// the first rune without any consideration for its width, but is sufficient for when the first
// character is known to be single byte width (as is the case for stdlib flag errors).
func upcaseInitial(str string) string {
	for i, v := range str {
		return string(unicode.ToUpper(v)) + str[i+1:]

	}
	return ""
}
