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

package main

import (
	"os"

	"github.com/kurafs/kura/doc"
	"github.com/kurafs/kura/pkg/cli"

	blobserver "github.com/kurafs/kura/cmd/blob-server"
	directoryserver "github.com/kurafs/kura/cmd/directory-server"
	identityserver "github.com/kurafs/kura/cmd/identity-server"
	storageserver "github.com/kurafs/kura/cmd/storage-server"
)

func main() {
	// We aggregate all the top-level commands, accessible via 'kura <command> ...', as needed.
	var commands cli.Commands

	// We include top level commands for {blob,storage,directory,identity} servers.
	commands = append(commands, blobserver.BlobServerCmd)
	commands = append(commands, directoryserver.DirectoryServerCmd)
	commands = append(commands, identityserver.IdentityServerCmd)
	commands = append(commands, storageserver.StorageServerCmd)

	// We also include a documentation pseudo-command for Kura's security model and architecture.
	commands = append(commands, doc.SecurityModelCmd)
	commands = append(commands, doc.ArchitectureCmd)

	// We define the top level CLI blurb here.
	abstract := "Kura is an end-to-end encrypted, synchronized, global file system in Go."
	if err := cli.Process(abstract, commands); err != nil {
		os.Exit(1)
	}
}