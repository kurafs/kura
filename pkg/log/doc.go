// Copyright 2018 Irfan Sharif.
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

// Package log implements leveled execution logs for go. The library provides
// hooks such that the following top-level usage is made possible:
//
//     $ <binary-name> -help
//     Usage of <binary-name>:
//       -log-dir string
//             Write log files in this directory.
//       -log-to-stderr
//             Log to standard error.
//       -log-level (info|debug|warn|error)
//             Log level for logs emitted (global, can be overrode using -log-filter).
//       -log-filter value
//             Comma-separated list of pattern:level settings for file-filtered logging.
//       -log-backtrace-at value
//             Comma-separated list of filename:N settings, when any logging statement at
//             the specified locations are executed, a stack trace will be emitted.
//
//     $ <binary-name> -log-level info \
//                     -log-dir /path/to/dir \
//                     -log-to-stderr \
//                     -log-filter f.go:warn,g/h/*.go:debug \
//                     -log-backtrace-at y.go:42
//
// These hooks can be invoked at runtime, what this means is that if needed, a
// running service could opt-in to provide open endpoints to accept logger
// reconfigurations (via RPCs or otherwise).
//
// Basic example:
//
//      import "github.com/irfansharif/log"
//
//      ...
//
//      logger := log.New()
//      logger.Info("hello, world")
//
// The logger can be be configured to be safe for concurrent use, output to
// rotating logs, log with specific formatted headers, etc. using variadic
// options during initialization. An example of the above:
//
//      writer := os.Stderr
//      writer = log.SynchronizedWriter(writer)
//      writer = log.MultiWriter(writer,
//                      log.LogRotationWriter("/logs", 50 << 20 /* 50 MiB */))
//
//      logf := log.Lmode | log.Ldate | log.Ltime | log.Llongfile
//
//      logger.New(log.Writer(writer), log.Flags(logf))
package log
