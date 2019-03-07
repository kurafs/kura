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

// Copyright (c) 2013 Kyle Isom <kyle@tyrfingr.is>
// Copyright (c) 2012 The Go Authors. All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//    * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//    * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package cryptserver

import (
	"bytes"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
)

// Ensure the KDF generates appropriately sized keys.
func TestKDF(t *testing.T) {
	msg := []byte("Hello, world")
	k, err := kdf(msg, nil, 64)
	if err != nil {
		t.Fatal(err)
	}
	if len(k) != 64 {
		t.Fatal("KDF: generated key is the wrong size")
	}
}

// Validate the ECDH component.
func TestSharedKey(t *testing.T) {
	prv1, err := generateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err.Error())
	}

	prv2, err := generateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	skLen := aesKeyLen
	sk1, err := generateShared(&prv2.PublicKey, prv1, skLen, skLen)
	if err != nil {
		t.Fatal(err)
	}

	sk2, err := generateShared(&prv1.PublicKey, prv2, skLen, skLen)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(sk1, sk2) {
		t.Fatal("expected equal shared keys")
	}
}

// Verify that an encrypted message can be successfully decrypted.
func TestEncryptDecrypt(t *testing.T) {
	prv1, err := generateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	prv2, err := generateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("Hello, world.")
	ct, err := encrypt(&prv2.PublicKey, message)
	if err != nil {
		t.Fatal(err)
	}

	pt, err := decrypt(prv2, ct)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(pt, message) {
		t.Fatal("plaintext doesn't match message")
	}

	_, err = decrypt(prv1, ct)
	if err == nil {
		t.Fatal("encryption should not have succeeded")
	}
}
