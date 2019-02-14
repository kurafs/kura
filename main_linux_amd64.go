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

	cacheserver "github.com/kurafs/kura/cmd/cache-server"
	identityserver "github.com/kurafs/kura/cmd/identity-server"
	metadataserver "github.com/kurafs/kura/cmd/metadata-server"
	storageserver "github.com/kurafs/kura/cmd/storage-server"
)

func main() {
	// We aggregate all the top-level commands (i.e. 'kura <command> ...') as
	// needed.
	var commands cli.Commands

	// We include top level commands for
	// {cache,storage,metadata,identity}-servers.
	commands = append(commands, cacheserver.CacheServerCmd)
	commands = append(commands, metadataserver.MetadataServerCmd)
	commands = append(commands, identityserver.IdentityServerCmd)
	commands = append(commands, storageserver.StorageServerCmd)

	// We also include a documentation pseudo-command for Kura's security model
	// and architecture.
	commands = append(commands, doc.SecurityModelCmd)
	commands = append(commands, doc.ArchitectureCmd)

	// We define the top level CLI abstract here.
	abstract := "Kura is an end-to-end encrypted, synchronized, global file system in Go."
	if err := cli.Process(abstract, commands); err != nil {
		os.Exit(1)
	}
}
