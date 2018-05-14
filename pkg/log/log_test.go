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
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"testing"
)

func TestSetGetGlobalPCMode(t *testing.T) {
	tp := fmt.Sprintf("%s:%d", "t.go", 42)
	SetTracePoint(tp)
	defer ResetTracePoint(tp)
	enabled := GetTracePoint(tp)
	if !enabled {
		t.Errorf("Expected tracepoint %s to be enabled", tp)
	}
}

func TestGetGlobalPCMode(t *testing.T) {
	tp := fmt.Sprintf("%s:%d", "t.go", 42)
	enabled := GetTracePoint(tp)
	if enabled {
		t.Errorf("Didn't expected tracepoint mode for %s to be enabled", tp)
	}
}

func TestInfoLog(t *testing.T) {
	SetGlobalLogMode(InfoMode)
	defer SetGlobalLogMode(DefaultMode)

	buffer := new(bytes.Buffer)
	logger := New(Writer(buffer))
	{
		logger.Info("info")
		regex := "^I.*: info"
		match, err := regexp.Match(regex, buffer.Bytes())
		if err != nil {
			t.Error(err)
		}
		if !match {
			t.Errorf("expected pattern: \"%s\", got: %s", regex, buffer.String())
		}
		buffer.Reset()
	}
	{
		logger.Infof("infof")
		regex := "^I.*: infof"
		match, err := regexp.Match(regex, buffer.Bytes())
		if err != nil {
			t.Error(err)
		}
		if !match {
			t.Errorf("expected pattern: \"%s\", got: %s", regex, buffer.String())
		}
		buffer.Reset()
	}
	{
		logger.Infof("%t %d %s", true, 1, "infof")
		regex := "^I.*: true 1 infof"
		match, err := regexp.Match(regex, buffer.Bytes())
		if err != nil {
			t.Error(err)
		}
		if !match {
			t.Errorf("expected pattern: \"%s\", got: %s", regex, buffer.String())
		}
		buffer.Reset()
	}
}

func TestDebugModeEnableDisable(t *testing.T) {
	SetGlobalLogMode(InfoMode)
	defer SetGlobalLogMode(DefaultMode)

	buffer := new(bytes.Buffer)
	logger := New(Writer(buffer))
	{
		logger.Debug("debug")
		logger.Debugf("%t %d %s", true, 1, "debugf")
		logger.Debugf("debugf")

		regex := "^$"
		match, err := regexp.Match(regex, buffer.Bytes())
		if err != nil {
			t.Error(err)
		}
		if !match {
			t.Errorf("expected pattern: \"%s\", got: %s", regex, buffer.String())
		}
		buffer.Reset()
	}
	SetGlobalLogMode(DebugMode)
	{
		logger.Debug("debug")
		regex := "^D.*: debug"
		match, err := regexp.Match(regex, buffer.Bytes())
		if err != nil {
			t.Error(err)
		}
		if !match {
			t.Errorf("expected pattern: \"%s\", got: %s", regex, buffer.String())
		}
		buffer.Reset()
	}
}

func TestEnableTracePoint(t *testing.T) {
	SetGlobalLogMode(DisabledMode)
	defer SetGlobalLogMode(DefaultMode)

	// XXX(irfansharif): This test depends on the exact difference in line
	// numbers between the call to callers and the logger.Info execution below.
	// The tracepoint is set to be the line exactly ten lines below it.

	file, line := caller(0)
	tp := fmt.Sprintf("%s:%d", filepath.Base(file), line+10)
	SetTracePoint(tp)
	if tpenabled := GetTracePoint(tp); !tpenabled {
		t.Errorf("Expected tracepoint %s to be enabled; found disabled", tp)
	}

	buffer := new(bytes.Buffer)
	logger := New(Writer(buffer))
	{
		logger.Info()
		if buffer.Len() == 0 {
			t.Error("Expected stack trace to be populated, found empty buffer instead")
		}

		line, err := buffer.ReadString(byte('\n'))
		if err != nil {
			t.Error(err)
		}

		goroutineRegex := "^goroutine [\\d]+ \\[running\\]:"
		match, err := regexp.Match(goroutineRegex, []byte(line))
		if err != nil {
			t.Error(err)
		}
		if !match {
			t.Errorf("expected pattern (first line): \"%s\", got: %s", goroutineRegex, line)
		}

		line, err = buffer.ReadString(byte('\n'))
		if err != nil {
			t.Error(err)
		}

		functionSignatureRegex := "^github.com/irfansharif/log.TestEnableTracePoint"
		match, err = regexp.Match(functionSignatureRegex, []byte(line))
		if err != nil {
			t.Error(err)
		}
		if !match {
			t.Errorf("expected pattern (second line): \"%s\", got: %s", functionSignatureRegex, line)
		}
	}
}
