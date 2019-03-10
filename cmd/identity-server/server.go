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

func (i *identityServer) GetIdentity(ctx context.Context, req *ipb.GetIdentityRequest) (*ipb.GetIdentityResponse, error) {
	var pkey []byte
	var server string
	err := i.db.View(func(tx *bolt.Tx) error {
		pkey = tx.Bucket([]byte("user-keys")).Get([]byte(req.Email))
		server = string(tx.Bucket([]byte("user-servers")).Get([]byte(req.Email)))
		return nil
	})

	if err != nil {
		return nil, err
	}

	if pkey == nil {
		return nil, errors.New("user not found")
	}

	return &ipb.GetIdentityResponse{
		PublicKey:      pkey,
		MetadataServer: server,
	}, nil
}

func (i *identityServer) PutIdentity(ctx context.Context, req *ipb.PutIdentityRequest) (*ipb.PutIdentityResponse, error) {
	err := i.db.Update(func(tx *bolt.Tx) error {
		{
			b := tx.Bucket([]byte("user-keys"))
			pkey := b.Get([]byte(req.Email))
			if pkey != nil {
				return errors.New("public key for email already exists")
			}
			err := b.Put([]byte(req.Email), req.PublicKey)
			return err
		}

		{
			b := tx.Bucket([]byte("user-servers"))
			server := b.Get([]byte(req.Email))
			if server != nil {
				return errors.New("server for email already exists")
			}
			err := b.Put([]byte(req.Email), []byte(req.MetadataServer))
			return err
		}
	})

	if err != nil {
		return nil, err
	}
	return &ipb.PutIdentityResponse{}, nil
}
