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

package cryptserver

import (
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"

	"github.com/kurafs/kura/pkg/log"
	"github.com/kurafs/kura/pkg/proquint"
)

var seedp string = "^([a-z]{5}-){7}([a-z]{5})$"

// drsrc is a deterministic random io.Reader.
type drsrc struct {
	entropy []byte
}

func (d *drsrc) Read(p []byte) (n int, err error) {
	b := sha256.Sum256(d.entropy)
	d.entropy = b[:]
	n = copy(p, d.entropy)
	return n, nil
}

func seedToEntropy(seed string, entropy []byte) error {
	matched, err := regexp.Match(`^([a-z]{5}-){7}([a-z]{5})$`, []byte(seed))
	if err != nil {
		return err
	}

	if !matched {
		return errors.New(
			fmt.Sprintf("expected seed %s to match regex %s", seed, seedp))
	}
	pattern := regexp.MustCompile(`[a-z]{5}`)
	for i, match := range pattern.FindAll([]byte(seed), -1) {
		binary.BigEndian.PutUint16(
			entropy[2*i:2*i+2], proquint.ToUint16(match))
	}

	return nil
}

func entropyToSeed(entropy [16]byte) string {
	var seed []byte
	for i := 0; i < 8; i++ {
		seed = append(seed,
			proquint.FromUint16(binary.BigEndian.Uint16(entropy[2*i:2*i+2]))...)
		seed = append(seed, byte('-'))
	}

	return string(seed[:len(seed)-1])
}

func GenerateAndWriteKeys(logger *log.Logger, seed string, keysDir string) error {
	var entropy [16]byte
	if seed == "" {
		_, err := rand.Read(entropy[:])
		if err != nil {
			return err
		}

		seed = entropyToSeed(entropy)
		logger.Infof("SEED = %s", seed)
	} else {
		if err := seedToEntropy(seed, entropy[:]); err != nil {
			return err
		}
	}

	d := &drsrc{entropy[:]}
	key, err := generateKey(elliptic.P256(), d)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return err
	}

	X, err := key.PublicKey.X.MarshalText()
	if err != nil {
		return err
	}
	Y, err := key.PublicKey.Y.MarshalText()
	if err != nil {
		return err
	}
	D, err := key.D.MarshalText()
	if err != nil {
		return err
	}

	pubkbuffer := make([]byte, 0)
	pubkbuffer = append(pubkbuffer, X...)
	pubkbuffer = append(pubkbuffer, byte('\n'))
	pubkbuffer = append(pubkbuffer, Y...)
	if err := ioutil.WriteFile(
		path.Join(keysDir, "public-key.kura"), pubkbuffer, 0400); err != nil {
		return err
	}

	if err := ioutil.WriteFile(
		path.Join(keysDir, "private-key.kura"), D, 0400); err != nil {
		return err
	}
	return nil
}
