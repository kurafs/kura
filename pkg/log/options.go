// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in licenses/BSD-golang.txt.

// Portions of this file are additionally subject to the following
// license and copyright.
//
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

// Portions of this code originated in the standard library 'log' package.

package log

import (
	"io"
	"path/filepath"
	"runtime"
)

type option func(*Logger)

type Flag int

// TODO(irfansharif): Include goroutine ID. Take a look at kubernetes logger
// implementation.

// These flags define which text to prefix to each log entry generated by the Logger.
const (
	// Bits or'ed together to control what's printed.
	// There is no control over the order they appear (the order listed
	// here) or the format they present (as described in the comments).
	//
	// For example, flags Lmode | Ldate | Ltime | Lshortfile, produces:
	//   I180419 06:33:04 fname.go:42] message
	//
	// Flags Ldate | Ltime | Lmicroseconds | Llongfile, produces:
	//   180419 06:33:04.606396 /src/repo/fname.go:42] message

	Ldate         Flag = 1 << iota // The date in the local time zone: 180419 (yymmdd)
	Ltime                          // The time in the local time zone: 01:23:23
	Lmicroseconds                  // Microsecond resolution: 01:23:23.123123, assumes Ltime
	Llongfile                      // Fully qualified file path and line number: /a/b/c/d.go:23
	Lshortfile                     // File name and line number: d.go:23. overrides Llongfile
	LUTC                           // If Ldate or Ltime is set, use UTC instead of local time zone
	Lmode                          // If Lmode is set, each line is prefixed by statement log mode

	// Default values for the logger, produces:
	//   I180419 06:33:04.606396 fname.go:42 message
	LstdFlags = Lmode | Ldate | Ltime | Lmicroseconds | LUTC | Lshortfile
)

// Writer configures a Logger instance with the specified io.Writer.
func Writer(w io.Writer) option {
	return func(l *Logger) {
		l.w = w
	}
}

// Flags configures the header format for all logs emitted by a Logger instance.
func Flags(flags Flag) option {
	return func(l *Logger) {
		l.flag = flags
	}
}

// SkipBasePath allows for log.Llongfile to only write out filepaths
// relative to project root. For example:
//
//  I180419 06:33:04.606396 main.go:89] from main!
//  I180419 06:33:04.606420 pkg/logger.go:6] from pkg!
//  I180419 06:33:04.606426 pkg/subpkg/logger.go:6] from pkg/subpkg!
//
// Instead of:
//
//  I180419 06:36:26.554520 [...]/log/cmd/logger/main.go:89] from main!
//  I180419 06:36:26.554555 [...]/log/cmd/logger/pkg/logger.go:6] from pkg!
//  I180419 06:36:26.554566 [...]/log/cmd/logger/pkg/subpkg/logger.go:6] from pkg/subpkg!
//
// Use SkipBasePath() with no arguments if calling from the root of the
// project/repository (think top level main.go). Barring that, passing in the
// fully qualified path of the project base strips out the corresponding prefix
// from subsequent log statements.
func SkipBasePath(path ...string) option {
	var basePath string
	if len(path) > 1 {
		panic("expecting single root dir")
	}

	if len(path) == 0 {
		// Determine root directory from caller.
		_, file, _, ok := runtime.Caller(1)
		if !ok {
			panic("unable to retrieve caller")
		}
		basePath = filepath.Dir(file)
	} else {
		// len(path) == 1, i.e. base path is provided.
		basePath = path[0]
	}

	return func(l *Logger) {
		l.basePath = basePath
	}
}
