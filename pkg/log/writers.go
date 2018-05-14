// Copyright 2013 Google Inc. All Rights Reserved.
// Copyright 2018 Irfan Sharif.
// Copyright 2018 The Kura Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Portions of this code originated in the github.com/golang/glog package.

package log

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"time"
)

var (
	program  = "?"
	hostname = "?"
	username = "?"
	pid      = -1
)

func init() {
	program = filepath.Base(os.Args[0])

	host, err := os.Hostname()
	if err == nil {
		hostname = host
	}

	currentUser, err := user.Current()
	if err == nil {
		username = currentUser.Username
	}

	pid = os.Getpid()
}

// DefaultWriter returns a default os.Stderr writer that is safe for concurrent use.
func DefaultWriter() io.Writer {
	return SynchronizedWriter(os.Stderr)
}

// LogRotationWriter returns an io.Writer that internally operates off the
// specified directory where writes are written out to rotating files,
// thresholded at the specified size in bytes. Within the directory a symlink
// is generated pointing to the most recently created log file.
//
// For the case where the size of the write exceeds the provided threshold
// size (which is probably indicative of an improperly configured threshold),
// we write it out to a single file. This is the only instance where the log
// file size may exceed the specified size limit.
func LogRotationWriter(dirname string, sizeThreshold int) io.Writer {
	os.MkdirAll(dirname, os.ModePerm)
	return &logRotationWriter{
		dirname:         dirname,
		symlink:         fmt.Sprintf("%s.log", program),
		currentFileSize: 0,
		sizeThreshold:   sizeThreshold,
	}
}

// SynchronizedWriter wraps an io.Writer with a mutex for concurrent access.
func SynchronizedWriter(w io.Writer) io.Writer {
	return &synchronizedWriter{
		w: w,
	}
}

// MultiWriter multiplexes writes to multiple io.Writers.
func MultiWriter(w io.Writer, ws ...io.Writer) io.Writer {
	mw := &multiWriter{}
	mw.ws = append(mw.ws, w)
	mw.ws = append(mw.ws, ws...)
	return mw
}

// generateLogFilename generates a name for a log file of the form
// <program>.<host>.<user>.<year>-<month>-<day>.<hour>:<minute>:<second>.<millisecond>.<pid>.log
// An example: logger.irfansharif-macbook.irfansharif.2018-04-10.22:43:54.717.7989.log
func generateLogFilename(t time.Time) (fname string) {
	return fmt.Sprintf("%s.%s.%s.%s.%d.log",
		program, hostname, username,
		t.Format("2006-01-02.15:04:05.999"), pid,
	)
}

type logRotationWriter struct {
	dirname, symlink               string
	currentFileSize, sizeThreshold int

	currentFile *os.File
}

// We create a new file within the given directory, if one is not present
// already or if we've written more bytes out than the provided threshold in
// our previous log file.
func (r *logRotationWriter) Write(b []byte) (n int, err error) {
	if r.currentFile == nil || (r.currentFileSize+len(b) > r.sizeThreshold) {
		fname := generateLogFilename(time.Now())
		f, err := os.Create(filepath.Join(r.dirname, fname))
		if err != nil {
			return 0, err
		}

		r.currentFile = f
		r.currentFileSize = 0
		os.Remove(filepath.Join(r.dirname, r.symlink))         // Remove symlink, if any, ignore error.
		os.Symlink(fname, filepath.Join(r.dirname, r.symlink)) // Best effort symlinking, ignore error.
	}

	n, err = r.currentFile.Write(b)
	r.currentFileSize += n
	return n, err
}

type synchronizedWriter struct {
	sync.Mutex
	w io.Writer
}

func (s *synchronizedWriter) Write(b []byte) (n int, err error) {
	s.Lock()
	n, err = s.w.Write(b)
	s.Unlock()
	return n, err
}

type multiWriter struct {
	ws []io.Writer
}

// We do a best effort write on all the writers, but return (n, err)
// conservatively. i.e. we return the smallest n across all the writers, and
// the last non-nil error, if any.
func (m *multiWriter) Write(b []byte) (n int, err error) {
	n = len(b) // Optimistic estimation.
	for _, w := range m.ws {
		nbytes, er := w.Write(b)
		if nbytes < n {
			n = nbytes
		}
		if er != nil {
			err = er
		}
	}
	return n, err
}
