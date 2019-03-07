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

package proquint_test

import (
	"bytes"
	"testing"

	"github.com/kurafs/kura/pkg/proquint"
)

func TestFromUint16(t *testing.T) {
	var tests = []struct {
		input    uint16
		expected []byte
	}{
		{0, []byte("babab")},
		{1, []byte("babad")},
		{2, []byte("babaf")},
		{34, []byte("babof")},
		{129, []byte("bafad")},
		{2510, []byte("bolav")},
		{16241, []byte("gutud")},
		{64298, []byte("zosop")},
	}

	for _, test := range tests {
		result := proquint.FromUint16(test.input)
		if !bytes.Equal(test.expected, result) {
			t.Errorf("expected %s, got %s", test.expected, result)
		}
	}
}

func TestToUint16(t *testing.T) {
	var tests = []struct {
		input    []byte
		expected uint16
	}{
		{[]byte("babab"), 0},
		{[]byte("babad"), 1},
		{[]byte("babaf"), 2},
		{[]byte("babof"), 34},
		{[]byte("bafad"), 129},
		{[]byte("bolav"), 2510},
		{[]byte("gutud"), 16241},
		{[]byte("zosop"), 64298},
	}

	for _, test := range tests {
		result := proquint.ToUint16(test.input)
		if result != test.expected {
			t.Errorf("expected %d, got %d", test.expected, result)
		}
	}
}

func TestUint32(t *testing.T) {
	var tests = []struct {
		i     uint32
		quint []byte
	}{
		{0, []byte("babab-babab")},
		{241418941, []byte("bunog-saput")},
		{31231151, []byte("balis-mufoz")},
		{543123113, []byte("fadiz-kipon")},
	}

	for _, test := range tests {
		{
			result := proquint.FromUint32(test.i)
			if !bytes.Equal(test.quint, result) {
				t.Errorf("expected %s, got %s", test.quint, result)
			}
		}
		{
			result := proquint.ToUint32(test.quint)
			if result != test.i {
				t.Errorf("expected %d, got %d", test.i, result)
			}
		}
	}
}

func TestUint64(t *testing.T) {
	var tests = []struct {
		i     uint64
		quint []byte
	}{
		{0, []byte("babab-babab-babab-babab")},
		{41418391241418941, []byte("bafig-filav-rahis-vofut")},
		{9489151893131231151, []byte("mavub-gujiz-balaz-zuvoz")},
		{81518945431213, []byte("babab-homoh-dozam-vupot")},
	}

	for _, test := range tests {
		{
			result := proquint.FromUint64(test.i)
			if !bytes.Equal(test.quint, result) {
				t.Errorf("expected %s, got %s", test.quint, result)
			}
		}
		{
			result := proquint.ToUint64(test.quint)
			if result != test.i {
				t.Errorf("expected %d, got %d", test.i, result)
			}
		}
	}
}
