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

package storageserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"

	"github.com/kurafs/kura/pkg/log"
	spb "github.com/kurafs/kura/pkg/pb/storage"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
)

type gdriveServer struct {
	mu     sync.RWMutex
	client *drive.Service
	store  Store
	logger *log.Logger
}

var _ spb.StorageServiceServer = &storageServer{}

func newGoogleDriveServer(client *drive.Service) *gdriveServer {
	return &gdriveServer{client: client}
}

func (g *gdriveServer) Has(name string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	id, err := g.getFileId(name)
	if err != nil {
		return false
	}

	_, err = g.client.Files.Get(id).Download()
	if err != nil {
		return false
	}

	return true
}

func (g *gdriveServer) Read(name string) ([]byte, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	id, err := g.getFileId(name)
	if err != nil {
		return nil, err
	}

	file, err := g.client.Files.Get(id).Download()
	if err != nil {
		return nil, errors.New("Unable to download file")
	}

	content, err := ioutil.ReadAll(file.Body)
	if err != nil {
		return nil, errors.New("Unable to serve file")
	}

	return content, nil
}

func (g *gdriveServer) Write(name string, contents []byte) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	query := "name = '" + name + "'"
	res, err := g.client.Files.List().Fields("files(name, id)").Q(query).Do()

	if err != nil {
		return err
	}

	for _, f := range res.Files {
		err = g.client.Files.Delete(f.Id).Do()
		if err != nil {
			return err
		}
	}

	r := bytes.NewReader(contents)
	_, err = g.client.Files.Create(&drive.File{Name: name}).Media(r).Do()
	if err != nil {
		return errors.New("Unable to write file to Google Drive")
	}

	return nil
}

func (g *gdriveServer) Rename(oldKey string, newKey string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	id, err := g.getFileId(oldKey)
	if err != nil {
		return err
	}

	file := &drive.File{Name: newKey, OriginalFilename: oldKey}
	_, err = g.client.Files.Update(id, file).Do()

	if err != nil {
		return errors.New("Unable to rename file")
	}

	return nil
}

func (g *gdriveServer) Erase(name string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	id, err := g.getFileId(name)
	if err != nil {
		return err
	}

	err = g.client.Files.Delete(id).Do()
	if err != nil {
		return err
	}

	return nil
}

func (g *gdriveServer) Keys() []string {
	g.mu.Lock()
	defer g.mu.Unlock()

	res := make([]string, 0)
	list, _ := g.client.Files.List().Fields("files(name)").Do()

	for _, file := range list.Files {
		res = append(res, file.Name)
	}

	return res
}

func (g *gdriveServer) getFileId(name string) (string, error) {
	query := "name = '" + name + "'"
	res, err := g.client.Files.List().Fields("files(name, id)").Q(query).Do()

	if err != nil {
		return "", err
	}

	if len(res.Files) == 0 {
		return "", errors.New("Read error: no such file or directory")
	}

	return res.Files[0].Id, nil
}

func (g *gdriveServer) Setup(logger *log.Logger) error {
	client, err := getClient(logger)
	if err != nil {
		return errors.New("Unable to locate Google Drive authentication file")
	}
	driveClient, err := drive.New(client)
	if err != nil {
		return errors.New("Unable to get Google Drive client")
	}

	g.client = driveClient
	g.logger = logger
	return nil
}

func getClient(logger *log.Logger) (*http.Client, error) {
	b, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		logger.Errorf("Unable to read client secret file: %v", err)
		return nil, err
	}

	config, err := google.ConfigFromJSON(b, drive.DriveScope)
	if err != nil {
		logger.Errorf("Unable to parse client secret file to config: %v", err)
		return nil, err
	}

	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(logger, config)
		saveToken(logger, tokFile, tok)
	}

	return config.Client(context.Background(), tok), nil
}

func getTokenFromWeb(logger *log.Logger, config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		logger.Errorf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		logger.Errorf("Unable to retrieve token from web %v", err)
	}
	return tok
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(logger *log.Logger, path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		logger.Errorf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
