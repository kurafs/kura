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

package proquint

import (
	"bytes"
)

var consonants = []byte("bdfghjklmnprstvz")
var vowels = []byte("aiou")

// FromUint16 converts an uint16 to a proquint of alternating consonants and
// vowels as follows.
//
// Four-bits as a consonant:
//      0 1 2 3 4 5 6 7 8 9 A B C D E F
//      b d f g h j k l m n p r s t v z
//
// Two-bits as a vowel:
//      0 1 2 3
//      a i o u
//
// Whole 16-bit word, where "con" = consonant, "vo" = vowel:
//      0 1 2 3 4 5 6 7 8 9 A B C D E F
//      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//      |con    |vo |con    |vo |con    |
//      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
func FromUint16(i uint16) []byte {
	var quint []byte
	for j := 0; j < 5; j++ {
		if j%2 == 0 {
			quint = append(quint, consonants[i&0xF000>>12])
			i <<= 4
		} else {
			quint = append(quint, vowels[i&0xC000>>14])
			i <<= 2
		}
	}
	return quint
}

// FromUint32 converts an uint32 to a proquint of alternating consonants and
// vowels as follows: FromUint16(first 16 bits)-FromUint16(last 16 bits)
func FromUint32(i uint32) []byte {
	var quint []byte
	quint = append(quint, FromUint16(uint16(i&0xFFFF0000>>16))...)
	quint = append(quint, byte('-'))
	quint = append(quint, FromUint16(uint16(i&0x0000FFFF))...)
	return quint
}

// FromUint64 converts an uint64 to a proquint of alternating consonants and
// vowels as follows: FromUint32(first 32 bits)-FromUint32(last 32 bits)
func FromUint64(i uint64) []byte {
	var quint []byte
	quint = append(quint, FromUint32(uint32(i&0xFFFFFFFF00000000>>32))...)
	quint = append(quint, byte('-'))
	quint = append(quint, FromUint32(uint32(i&0x00000000FFFFFFFF))...)
	return quint
}

func ToUint16(quint []byte) uint16 {
	var i uint16
	if len(quint) != 5 {
		panic("invalid len(quint), expected 5")
	}

	for _, c := range quint {
		isVowel := bytes.IndexByte(vowels, c) != -1
		if isVowel {
			i <<= 2
			i += uint16(bytes.IndexByte(vowels, c))
		} else {
			i <<= 4
			i += uint16(bytes.IndexByte(consonants, c))
		}
	}
	return i
}

func ToUint32(quint []byte) uint32 {
	if len(quint) != 5*2+1 { // Count the separator.
		panic("invalid len(quint), expected 11")
	}

	return uint32(ToUint16(quint[0:5]))<<16 + uint32(ToUint16(quint[6:]))
}

func ToUint64(quint []byte) uint64 {
	if len(quint) != 5*4+3 { // Count the separators.
		panic("invalid len(quint), expected 23")
	}
	return uint64(ToUint32(quint[0:11]))<<32 + uint64(ToUint32(quint[12:]))
}
