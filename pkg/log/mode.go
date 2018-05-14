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

// TODO(irfansharif): Comment, explain modal logging concept.
type Mode int

const (
	InfoMode Mode = 1 << iota
	WarnMode
	ErrorMode
	FatalMode
	DebugMode

	// The zero-value of DisableMode can also be used to check if modes
	// intersect, i.e.  (lmode&gmode) != DisabledMode checks if the local
	// logger mode is filtered through by the global mode.
	DisabledMode = 0
	DefaultMode  = InfoMode | WarnMode | ErrorMode
)

func (m Mode) string() string {
	switch m {
	case InfoMode:
		return "I"
	case WarnMode:
		return "W"
	case ErrorMode:
		return "E"
	case FatalMode:
		return "F"
	case DebugMode:
		return "D"
	default:
		return "?"
	}
}

func (m Mode) byte() byte {
	switch m {
	case InfoMode:
		return 'I'
	case WarnMode:
		return 'W'
	case ErrorMode:
		return 'E'
	case FatalMode:
		return 'F'
	case DebugMode:
		return 'D'
	default:
		return '?'
	}
}
