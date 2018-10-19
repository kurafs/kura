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
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"
)

// TODO(irfansharif): Implement tagging API?
// TODO(irfansharif): Implement custom leveling/verbosity with filtering?

// Logger is the concrete logger type. It writes out logs to the specified
// io.Writer, with the header format determined by the flags set.
type Logger struct {
	w        io.Writer // Where logs are written to
	flag     Flag      // Flag set determining log headers. See options.go
	basePath string    // Base path of the consumer's repository, optional
}

const newline string = "\n"

// configure sets up the default options for the Logger, these include a
// synchronized os.Stderr writer, an empty basepath (Llongfile will
// consequently print out the fully specified path) and Lstdflags, which
// produces logs with the following header format:
//
//   Myymmdd hh:mm:ss:millis filename:ln message
// 	 I180419 06:33:04.606396 fname.go:42 message
func configure(l *Logger) {
	l.w = DefaultWriter()
	l.flag = LstdFlags
	l.basePath = ""
}

// New returns a new Logger, configured with the provided options, if any.
func New(options ...option) *Logger {
	l := &Logger{}
	configure(l)

	// Overrides.
	for _, option := range options {
		option(l)
	}
	return l
}

// Discarder returns a Logger configured to discard all writes.
func Discarder() *Logger {
	return New(Writer(ioutil.Discard))
}

// Info logs to the INFO log. Arguments are handled in the manner of
// fmt.Println; a newline is appended at the end.
func (l *Logger) Info(v ...interface{}) {
	l.log(InfoMode, fmt.Sprintln(v...))
}

// Infof logs to the INFO log. Arguments are handled in the manner of
// fmt.Printf; a newline is appended at the end.
func (l *Logger) Infof(format string, v ...interface{}) {
	l.log(InfoMode, fmt.Sprintf(format+newline, v...))
}

// Warn logs to the WARN log. Arguments are handled in the manner of
// fmt.Println; a newline is appended at the end.
func (l *Logger) Warn(v ...interface{}) {
	l.log(WarnMode, fmt.Sprintln(v...))
}

// Warnf logs to the WARN log. Arguments are handled in the manner of
// fmt.Printf; a newline is appended at the end.
func (l *Logger) Warnf(format string, v ...interface{}) {
	l.log(WarnMode, fmt.Sprintf(format+newline, v...))
}

// Error logs to the ERROR log. Arguments are handled in the manner of
// fmt.Println; a newline is appended at the end.
func (l *Logger) Error(v ...interface{}) {
	l.log(ErrorMode, fmt.Sprintln(v...))
}

// Errorf logs to the ERROR log. Arguments are handled in the manner of
// fmt.Printf; a newline is appended at the end.
func (l *Logger) Errorf(format string, v ...interface{}) {
	l.log(ErrorMode, fmt.Sprintf(format+newline, v...))
}

// Fatal logs to the FATAL log. Arguments are handled in the manner of
// fmt.Println; a newline is appended at the end.
//
// TODO(irfansharif): Including a stack trace of all running goroutines, then
// calls os.Exit(255).
func (l *Logger) Fatal(v ...interface{}) {
	l.log(FatalMode, fmt.Sprintln(v...))
	os.Exit(255)
}

// Fatalf logs to the FATAL log. Arguments are handled in the manner of
// fmt.Printf; a newline is appended at the end.
//
// TODO(irfansharif): Including a stack trace of all running goroutines, then
// calls os.Exit(255).
func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.log(FatalMode, fmt.Sprintf(format+newline, v...))
	os.Exit(255)
}

// Debug logs to the DEBUG log. Arguments are handled in the manner of
// fmt.Println; a newline is appended at the end.
func (l *Logger) Debug(v ...interface{}) {
	l.log(DebugMode, fmt.Sprintln(v...))
}

// Debugf logs to the DEBUG log. Arguments are handled in the manner of
// fmt.Printf; a newline is appended at the end.
func (l *Logger) Debugf(format string, v ...interface{}) {
	l.log(DebugMode, fmt.Sprintf(format+newline, v...))
}

// Logger.log is only to be called from
// Logger.{Info,Warn,Error,Fatal,Debug}{,f}. We use a depth of two to retrieve
// the caller immediately preceding it.
func (l *Logger) log(lmode Mode, data string) {
	// TODO(irfansharif): Right now this isn't robust to shared filenames across
	// varied sub packages (for tracepoints and file log modes both). This is a
	// stand-in to allow for direct file name specification without
	// fully-specified paths (in the host machine or relative to project root).
	// We could implement for project root relative paths if project root was
	// provided.
	file, line := caller(2)
	bfile := filepath.Base(file)
	tp := fmt.Sprintf("%s:%d", bfile, line)

	tpenabled := GetTracePoint(tp)
	if tpenabled {
		// Skip logger.log, and the invoking public wrapper
		// Logger.{Info,Warn,Error,Fatal,Debug}{,f}
		l.w.Write(stacktrace(2))
	}

	var shouldLog bool
	if fmode, ok := GetFileLogMode(bfile); ok && (fmode&lmode) != DisabledMode {
		// Log mode satisfies the specific file mode. Since file mode filtering
		// is only used for overrides, we check for this first.
		// Log mode satisfies specific file mode, and crucially, not the global
		// mode. File mode filtering is only to be used for overrides, if global
		// log mode is satisfied, we already capture it.
		shouldLog = true
	} else if gmode := GetGlobalLogMode(); !ok && (gmode&lmode) != DisabledMode {
		// Log mode satisfies global mode, and crucially, there isn't
		// a file specific override.
		shouldLog = true
	} else if (lmode & FatalMode) != DisabledMode {
		// Logger.Fatal{,f} statements aren't filtered out.
		shouldLog = true
	}

	if !shouldLog {
		return
	}

	var buf bytes.Buffer
	buf.Write(l.header(lmode, time.Now(), file, line))
	buf.WriteString(data)
	l.w.Write(buf.Bytes())
}

// header, given the local log mode, time stamp, file name (fully qualified)
// and line number, formats the log header as per Logger.flag and returns the
// corresponding byte array. It also factors in the configured base path, if
// any, so that of Llongfile is specified, the base path prefix is truncated.
func (l *Logger) header(lmode Mode, t time.Time, file string, line int) []byte {
	var b []byte
	var buf *[]byte = &b
	if l.flag&(Lmode) != 0 {
		*buf = append(*buf, lmode.byte())
	}
	if l.flag&LUTC != 0 {
		t = t.UTC()
	}
	if l.flag&(Ldate|Ltime|Lmicroseconds) != 0 {
		datef := l.flag&Ldate != 0
		timef := l.flag&(Ltime|Lmicroseconds) != 0
		if datef {
			year, month, day := t.Date()
			if year < 2000 {
				year = 2000
			}
			itoa(buf, year-2000, 2)
			itoa(buf, int(month), 2)
			itoa(buf, day, 2)
		}

		if datef && timef {
			*buf = append(*buf, ' ')
		}

		if timef {
			hour, min, sec := t.Clock()
			itoa(buf, hour, 2)
			*buf = append(*buf, ':')
			itoa(buf, min, 2)
			*buf = append(*buf, ':')
			itoa(buf, sec, 2)
			if l.flag&Lmicroseconds != 0 {
				*buf = append(*buf, '.')
				itoa(buf, t.Nanosecond()/1e3, 6)
			}
		}
	}

	*buf = append(*buf, ' ')

	if l.flag&(Lshortfile|Llongfile) != 0 {
		// This will panic with index out of range if the project path is
		// improperly configured. Consider project path is defined to be
		// [...]/app/pkg/subpkg (read: not the project root), and is used at the
		// top level, [...]/app/main.go, it will panic is it's trying to drop
		// the prefix [...]/app.
		file = file[len(l.basePath):]
		if len(l.basePath) != 0 {
			// [1:] is for leading '/', if basePath is non-empty.
			file = file[1:]
		}

		if l.flag&Lshortfile != 0 {
			short := file
			for i := len(file) - 1; i > 0; i-- {
				if file[i] == '/' {
					short = file[i+1:]
					break
				}
			}
			file = short
		}
		*buf = append(*buf, file...)
		*buf = append(*buf, ':')
		itoa(buf, line, -1)
		*buf = append(*buf, "] "...)
	}
	return b
}

// Cheap integer to fixed-width decimal ASCII. Give a negative width to avoid
// zero-padding.
func itoa(buf *[]byte, i int, wid int) {
	// Assemble decimal in reverse order.
	var b [20]byte
	bp := len(b) - 1
	for i >= 10 || wid > 1 {
		wid--
		q := i / 10
		b[bp] = byte('0' + i - q*10)
		bp--
		i = q
	}
	// i < 10
	b[bp] = byte('0' + i)
	*buf = append(*buf, b[bp:]...)
}

// stacktrace returns the stack trace for the current goroutine, skipping n
// immediately preceding function traces (last being the caller, inclusive of
// the caller).
//
// e.go: 32 func e() {
// e.go: 33     f()
// e.go: 34 }
//
// f.go: 11 func f() {
// f.go: 12      g()
// f.go: 13 }
//
// g.go: 25 func g() {
// g.go: 26 	{
// g.go: 27         // Request stack trace, skipping no preceding levels.
// g.go: 28 		strace := stacktrace(0)
// g.go: 29
// g.go: 30         // goroutine 1 [running]:
// g.go: 31         // g.g()
// g.go: 32         //     /path/g.go:28 +0x2e
// g.go: 33         // f.f()
// g.go: 34         //     /path/f.go:12 +0x20
// g.go: 35         // e.e()
// g.go: 36         //     /path/e.go:33 +0x20
// g.go: 37         // ...
// g.go: 38 		fmt.Println(string(strace))
// g.go: 39 	}
// g.go: 40 	{
// g.go: 41         // Request stack trace, skipping one preceding level.
// g.go: 42 		strace := stacktrace(1)
// g.go: 43
// g.go: 44         // goroutine 1 [running]:
// g.go: 45         // f.f()
// g.go: 46         //     /path/f.go:12 +0x20
// g.go: 47         // e.e()
// g.go: 48         //     /path/e.go:33 +0x20
// g.go: 49         // ...
// g.go: 50 		fmt.Println(string(strace))
// g.go: 51 	}
// g.go: 52 }
//
func stacktrace(skip int) []byte {
	skip *= 2 // Each function depth corresponds to to lines of stack trace output.
	skip += 2 // For debug.Stack()
	skip += 2 // For this function, log.stacktrace()

	b := debug.Stack()
	bs := bytes.Split(b, []byte("\n"))

	// TODO(irfansharif): Are these pointer copies? It could/should be.
	copy(bs[1:], bs[1+skip:])
	bs = bs[:len(bs)-skip]
	return bytes.Join(bs, []byte("\n"))
}

// caller returns the file and line number of where the caller's caller's
// call site.
//
// e.go: 32 func e() {
// e.go: 33     f()
// e.go: 34 }
//
// f.go: 11 func f() {
// f.go: 12      g()
// f.go: 13 }
//
// g.go: 25 func g() {
// g.go: 26 	{
// g.go: 27         // Request caller one level above.
// g.go: 28 		file, line := caller(1)
// g.go: 29 		fmt.Println(fmt.Sprintf("%s: %d", file, line)) // f.go: 12
// g.go: 30 	}
// g.go: 31 	{
// g.go: 32         // Request caller two levels above.
// g.go: 33 		file, line := caller(2)
// g.go: 34 		fmt.Println(fmt.Sprintf("%s: %d", file, line)) // e.go: 33
// g.go: 35 	}
// g.go: 36 }
//
func caller(depth int) (file string, line int) {
	// +1 to account for call to caller itself.
	_, file, line, ok := runtime.Caller(depth + 1)
	if !ok {
		file = "[???]"
		line = -1
	}
	return file, line
}
