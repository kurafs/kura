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
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"math/big"

	"golang.org/x/crypto/hkdf"
)

var aesKeyLen int = 16

// encrypt encrypts a message using ECIES as specified in SEC 1, 5.1.3.
func encrypt(pub *ecdsa.PublicKey, plaintext []byte) ([]byte, error) {
	R, err := generateKey(pub.Curve, rand.Reader)
	if err != nil {
		return nil, err
	}
	Rb := elliptic.Marshal(R.PublicKey.Curve, R.PublicKey.X, R.PublicKey.Y)

	z, err := generateShared(pub, R, aesKeyLen, aesKeyLen)
	if err != nil {
		return nil, err
	}

	K, err := kdf(z, nil, aesKeyLen+aesKeyLen)
	if err != nil {
		return nil, err
	}

	EK := K[:aesKeyLen]
	MK := K[aesKeyLen:]

	EM, err := encryptAES(EK, plaintext)
	if err != nil {
		return nil, err
	}

	D := mac(MK, EM, nil)

	ciphertext := make([]byte, len(Rb)+len(EM)+len(D))
	copy(ciphertext, Rb)
	copy(ciphertext[len(Rb):], EM)
	copy(ciphertext[len(Rb)+len(EM):], D)
	return ciphertext, nil
}

// decrypt decrypts an ECIES ciphertext as specified in SEC 1, 5.1.4.
func decrypt(priv *ecdsa.PrivateKey, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, errors.New("empty ciphertext")
	}

	hash := sha256.New()
	var (
		rblen int
		mlen  int = hash.Size()
	)

	switch ciphertext[0] {
	case 2, 3:
		rblen = (priv.PublicKey.Curve.Params().BitSize+7)>>3 + 1
	case 4:
		rblen = 2*((priv.PublicKey.Curve.Params().BitSize+7)>>3) + 1
	default:
		return nil, errors.New("invalid public key")
	}

	Rb := ciphertext[:rblen]
	EM := ciphertext[rblen : len(ciphertext)-mlen]
	D := ciphertext[len(ciphertext)-mlen:]

	R := new(ecdsa.PublicKey)
	R.Curve = elliptic.P256()
	R.X, R.Y = elliptic.Unmarshal(R.Curve, Rb)
	if R.X == nil {
		return nil, errors.New("invalid public key")
	}

	z, err := generateShared(R, priv, aesKeyLen, aesKeyLen)
	if err != nil {
		return nil, err
	}

	K, err := kdf(z, nil, aesKeyLen+aesKeyLen)
	if err != nil {
		return nil, err
	}

	EK := K[:aesKeyLen]
	MK := K[aesKeyLen:]

	if !hmac.Equal(mac(MK, EM, nil), D) {
		return nil, errors.New("invalid message")
	}

	plaintext, err := decryptAES(EK, ciphertext[rblen:len(ciphertext)-mlen])
	return plaintext, err
}

// encryptAES carries out AES encryption using the provided key.
func encryptAES(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)
	return ciphertext, nil
}

// decryptAES carries out AES decryption using the provided key.
func decryptAES(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	iv := ciphertext[:aes.BlockSize]
	plaintext := ciphertext[aes.BlockSize:]

	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(plaintext, plaintext)
	return plaintext, nil
}

// Generate an elliptic curve public/private keypair.
func generateKey(curve elliptic.Curve, rand io.Reader) (*ecdsa.PrivateKey, error) {
	pb, x, y, err := elliptic.GenerateKey(curve, rand)
	if err != nil {
		return nil, err
	}
	priv := new(ecdsa.PrivateKey)
	priv.PublicKey.X = x
	priv.PublicKey.Y = y
	priv.PublicKey.Curve = curve
	priv.D = new(big.Int).SetBytes(pb)

	return priv, nil
}

// ECDH key agreement method used to establish secret keys for encryption.
func generateShared(pub *ecdsa.PublicKey, priv *ecdsa.PrivateKey, skLen, macLen int) ([]byte, error) {
	x, _ := pub.Curve.ScalarMult(pub.X, pub.Y, priv.D.Bytes())
	if x == nil {
		return nil, errors.New("shared key is point at infinity")
	}

	sharedKey := make([]byte, skLen+macLen)
	copy(sharedKey[len(sharedKey)-len(x.Bytes()):], x.Bytes())
	return sharedKey, nil
}

func kdf(secret, shared []byte, kdflen int) ([]byte, error) {
	hkdf := hkdf.New(sha256.New, secret, shared, nil)
	key := make([]byte, kdflen)
	if _, err := io.ReadFull(hkdf, key); err != nil {
		return nil, err
	}
	return key, nil
}

func mac(key, message, shared []byte) []byte {
	tag := hmac.New(sha256.New, key)
	tag.Write(message)
	tag.Write(shared)
	return tag.Sum(nil)
}
