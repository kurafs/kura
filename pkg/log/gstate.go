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

package log

import (
	"sync"
	"sync/atomic"
)

// Map from program counter fname.go:linenumber to mode.
type tracePointMap map[string]struct{}
type fileModeMap map[string]Mode
type gstateT struct {
	gmode        atomic.Value
	tracePointMu struct {
		sync.Mutex
		m atomic.Value // type: tracePointMap
	}
	fileModeMu struct {
		sync.Mutex
		m atomic.Value // type: fileModeMap
	}
}

var gstate gstateT

// Need to initialize the atomics; to be used once during init time.
func init() {
	gstate.gmode.Store(DefaultMode)
	gstate.tracePointMu.m.Store(make(tracePointMap))
	gstate.fileModeMu.m.Store(make(fileModeMap))
}

// SetGlobalLogMode sets the global log mode to the one specified. Logging
// outside what's included in the mode is thereby suppressed.
func SetGlobalLogMode(m Mode) {
	gstate.gmode.Store(m)
}

// GetGlobalLogMode gets the currently set global log mode.
func GetGlobalLogMode() Mode {
	return gstate.gmode.Load().(Mode)
}

// SetTracePoint enables the provided tracepoint. A tracepoint is of the form
// filename.go:line-number (compiles to [\w]+.go:[\d]+) corresponding to the
// position of a logging statement that once enabled, emits a backtrace when
// the logging statement is executed. The specified tracepoint is agnostic to
// the mode, i.e. Logger.{Info|Warn|Error|Fatal|Debug}{,f}, used at the line.
func SetTracePoint(tp string) {
	gstate.tracePointMu.Lock()                         // Synchronize with other potential writers.
	ma := gstate.tracePointMu.m.Load().(tracePointMap) // Load current value of the map.
	mb := make(tracePointMap)                          // Create a new map.
	for tp := range ma {
		mb[tp] = struct{}{} // Copy all data from the current object to the new one.
	}
	mb[tp] = struct{}{}             // Do the update that we need.
	gstate.tracePointMu.m.Store(mb) // Atomically replace the current object with the new one.
	// At this point all new readers start working with the new version.
	// The old version will be garbage collected once the existing readers
	// (if any) are done with it.
	gstate.tracePointMu.Unlock()
}

// ResetTracePoint resets the provided tracepoint so that a backtraces are no
// longer emitted when the specified logging statement is executed. See comment
// for SetTracePoint for what a tracepoint is.
func ResetTracePoint(tp string) {
	gstate.tracePointMu.Lock()                         // Synchronize with other potential writers.
	ma := gstate.tracePointMu.m.Load().(tracePointMap) // Load current value of the map.
	mb := make(tracePointMap)                          // Create a new map.
	for tp := range ma {
		mb[tp] = struct{}{} // Copy all data from the current object to the new one.
	}
	delete(mb, tp)                  // Do the update that we need.
	gstate.tracePointMu.m.Store(mb) // Atomically replace the current object with the new one.
	// At this point all new readers start working with the new version.
	// The old version will be garbage collected once the existing readers
	// (if any) are done with it.
	gstate.tracePointMu.Unlock()
}

// GetTracePoint checks if the corresponding tracepoint is enabled.
func GetTracePoint(tp string) (tpenabled bool) {
	tpmap := gstate.tracePointMu.m.Load().(tracePointMap)
	_, ok := tpmap[tp]
	return ok
}

// SetFileLogMode sets the log mode for the provided filename. Subsequent
// logging statements within the file get filtered accordingly.
func SetFileLogMode(fname string, m Mode) {
	gstate.fileModeMu.Lock()                       // Synchronize with other potential writers.
	ma := gstate.fileModeMu.m.Load().(fileModeMap) // Load current value of the map.
	mb := make(fileModeMap)                        // Create a new map.
	for fname, m := range ma {
		mb[fname] = m // Copy all data from the current object to the new one.
	}
	mb[fname] = m                 // Do the update that we need.
	gstate.fileModeMu.m.Store(mb) // Atomically replace the current object with the new one.
	// At this point all new readers start working with the new version.
	// The old version will be garbage collected once the existing readers
	// (if any) are done with it.
	gstate.fileModeMu.Unlock()
}

// GetFileLogMode gets the log mode for the specified file.
func GetFileLogMode(fname string) (m Mode, ok bool) {
	fmmap := gstate.fileModeMu.m.Load().(fileModeMap)
	m, ok = fmmap[fname]
	return m, ok
}

// ResetFileLogMode resets the log mode for the provided filename. Subsequent
// logging statements within the file get filtered as per the global log mode.
func ResetFileLogMode(fname string) {
	gstate.fileModeMu.Lock()                       // Synchronize with other potential writers.
	ma := gstate.fileModeMu.m.Load().(fileModeMap) // Load current value of the map.
	mb := make(fileModeMap)                        // Create a new map.
	for fname, m := range ma {
		mb[fname] = m // Copy all data from the current object to the new one.
	}
	delete(mb, fname)             // Do the update that we need.
	gstate.fileModeMu.m.Store(mb) // Atomically replace the current object with the new one.
	// At this point all new readers start working with the new version.
	// The old version will be garbage collected once the existing readers
	// (if any) are done with it.
	gstate.fileModeMu.Unlock()
}
