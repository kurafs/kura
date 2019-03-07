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
	"context"
	"crypto/ecdsa"
	"sync"

	"github.com/kurafs/kura/pkg/log"
	cpb "github.com/kurafs/kura/pkg/pb/crypt"
)

type cryptServer struct {
	mu     sync.RWMutex
	priv   *ecdsa.PrivateKey
	logger *log.Logger
}

var _ cpb.CryptServiceServer = &cryptServer{}

func newCryptServer(logger *log.Logger, priv *ecdsa.PrivateKey) *cryptServer {
	return &cryptServer{
		priv:   priv,
		logger: logger,
	}
}

func (c *cryptServer) Encrypt(ctx context.Context, req *cpb.EncryptionRequest) (
	*cpb.EncryptionResponse, error,
) {
	var ciphertext []byte
	var err error

	if req.AesKey != nil {
		ciphertext, err = encryptAES(req.AesKey, req.Plaintext)
		if err != nil {
			return nil, err
		}
	} else {
		ciphertext, err = encrypt(&c.priv.PublicKey, req.Plaintext)
		if err != nil {
			return nil, err
		}
	}

	resp := &cpb.EncryptionResponse{Ciphertext: ciphertext}
	return resp, nil
}

func (c *cryptServer) Decrypt(ctx context.Context, req *cpb.DecryptionRequest) (*cpb.DecryptionResponse, error) {
	var plaintext []byte
	var err error

	if req.AesKey != nil {
		plaintext, err = decryptAES(req.AesKey, req.Ciphertext)
		if err != nil {
			return nil, err
		}

	} else {
		plaintext, err = decrypt(c.priv, req.Ciphertext)
		if err != nil {
			return nil, err
		}
	}

	resp := &cpb.DecryptionResponse{Plaintext: plaintext}
	return resp, nil
}

func (c *cryptServer) PublicKey(context.Context, *cpb.PublicKeyRequest) (*cpb.PublicKeyResponse, error) {
	X, err := c.priv.PublicKey.X.MarshalText()
	if err != nil {
		return nil, err
	}
	Y, err := c.priv.PublicKey.Y.MarshalText()
	if err != nil {
		return nil, err
	}
	resp := &cpb.PublicKeyResponse{
		X: X,
		Y: Y,
	}
	return resp, nil
}
