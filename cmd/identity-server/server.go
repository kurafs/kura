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

package identityserver

import (
	"context"
	"errors"

	"github.com/boltdb/bolt"
	"github.com/kurafs/kura/pkg/log"
	ipb "github.com/kurafs/kura/pkg/pb/identity"
)

type identityServer struct {
	logger *log.Logger
	db     *bolt.DB
}

var _ ipb.IdentityServiceServer = &identityServer{}

func newIdentityServer(logger *log.Logger, db *bolt.DB) *identityServer {
	return &identityServer{
		logger: logger,
		db:     db,
	}
}

func (i *identityServer) GetPublicKey(ctx context.Context, req *ipb.GetKeyRequest) (*ipb.GetKeyResponse, error) {
	var pkey []byte
	err := i.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("user-keys"))
		pkey = b.Get([]byte(req.Email))
		return nil
	})

	if err != nil {
		return nil, err
	}

	if pkey == nil {
		return nil, errors.New("user not found")
	}

	return &ipb.GetKeyResponse{PublicKey: pkey}, nil
}

func (i *identityServer) PutPublicKey(ctx context.Context, req *ipb.PutKeyRequest) (*ipb.PutKeyResponse, error) {
	err := i.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("user-keys"))
		pkey := b.Get([]byte(req.Email))
		if pkey != nil {
			return errors.New("public key for email already exists")
		}
		err := b.Put([]byte(req.Email), req.PublicKey)
		return err
	})

	if err != nil {
		return nil, err
	}
	return &ipb.PutKeyResponse{}, nil
}
