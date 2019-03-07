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

// Package cli allows the construction of structured command-line interfaces with sub-commands and
// help topics. This is very similar to the interface in git where the top-level program name (git)
// is preceded by a qualifier that determines what sub-command to execute
// (git {reflog,commit,cherry-pick}).
//
// Package cli explicitly avoid init time global hooks and has a minimal binary size footprint.
//
// Example (from kurafs/kura):
//
//      // We aggregate all the top-level commands, accessible via 'kura <command> ...', as needed.
//	    var commands cli.Commands
//
//	    // We include top level commands for {blob,storage,directory,identity} servers.
//	    commands = append(commands, blobserver.BlobServerCmd)
//	    commands = append(commands, directoryserver.DirectoryServerCmd)
//	    commands = append(commands, identityserver.IdentityServerCmd)
//      commands = append(commands, storageserver.StorageServerCmd)
//
//      // We also include a documentation pseudo-command for Kura's security model and architecture.
//      commands = append(commands, doc.SecurityModelCmd)
//      commands = append(commands, doc.ArchitectureCmd)
//
//      // We define the top level CLI blurb here.
//      abstract := "Kura is an end-to-end encrypted, synchronized, global file system in Go."
//      if err := cli.Process(abstract, commands); err != nil {
//      	os.Exit(1)
//      }
//
// This generates the following top-level behaviour:
//
//      $ kura {,-h,help}
//      Kura is an end-to-end encrypted, synchronized, global file system in Go.
//
//      Usage:
//
//          kura command [arguments]
//
//      The commands are:
//
//              blob-server            blob-server command overview
//              directory-server       directory-server command overview
//              identity-server        identity-server command overview
//              storage-server         storage-server command overview
//
//      Use 'kura help [command]' for more information about a command.
//
//      Additional help topics:
//
//              security-model         security model overview
//              architecture           kura system architecture overview
//
//      Use "kura help [topic]" for more information about that topic.
//
// Using help for a listed command displays to following:
//
//      $ kura help storage-server
//      Usage: kura storage-server [-f] [-a arg]
//
//      Storage server detailed overview.
//
// Doing the same for an additional help topic, we get the following:
//
//      $ kura help architecture
//      Topic: kura system architecture overview
//
//      Detailed description about the system architecture for Kura.
//
// Individual commands also have their own '-h' switches for additional command details.
//
//      $ kura storage-server -h
//      Usage:
//
//          kura storage-server [-f] [-a arg]
//
//          -a string
//              Argument parameter usage
//          -f    Flag usage
//
package cli

// TODO(irfansharif): What about top level root command flags? Applicable across sub-commands?
