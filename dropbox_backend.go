package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/stacktic/dropbox"
)

type DropBoxBackend struct {
	AccessKey string
	SecretKey string
	Token     string
	db        *dropbox.Dropbox
}

func NewDropBoxBackend(access_key, secret_key, token string) *DropBoxBackend {
	db := dropbox.NewDropbox()
	db.SetAppInfo(access_key, secret_key)

	if token == "" {
		fmt.Println("DropBox access needs to be authorized")
		if err := db.Auth(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		token = db.AccessToken()
		fmt.Println("set your CASK_DROPBOX_TOKEN environment variable to:")
		fmt.Println(token)
		os.Exit(1)
	}

	db.SetAccessToken(token)

	return &DropBoxBackend{access_key, secret_key, token, db}
}

func (d DropBoxBackend) String() string {
	return "DropBox"
}

func (s *DropBoxBackend) Write(key Key, r io.ReadCloser) error {
	path := key.Algorithm + "/" + key.AsPath()
	// ignore error on this since it means that the path
	// already exists
	s.db.CreateFolder(path)

	_, err := s.db.UploadByChunk(r, 1024*1024, path+"/data", true, "")
	if err != nil {
		log.Println("uh oh. couldn't write to dropbox")
		log.Println(err)
		return err
	}
	return nil
}

func (s DropBoxBackend) Read(key Key) ([]byte, error) {
	path := key.Algorithm + "/" + key.AsPath()
	r, _, err := s.db.Download(path+"/data", "", 0)
	if err != nil {
		log.Println("error downloading from dropbox")
		log.Println(err)
		return nil, err
	}
	b, err := ioutil.ReadAll(r)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return b, nil
}

func (s DropBoxBackend) Exists(key Key) bool {
	path := key.Algorithm + "/" + key.AsPath()
	ent, err := s.db.Metadata(path+"/data", false, false, "", "", 1)

	if err != nil || ent == nil {
		return false
	}
	return true
}

func (s *DropBoxBackend) Delete(key Key) error {
	_, err := s.db.Delete(key.Algorithm + "/" + key.AsPath() + "/data")
	return err
}

func (s *DropBoxBackend) ActiveAntiEntropy(cluster *Cluster, site Site, interval int) {
	log.Println("DropBox AAE starting")
	// DropBox backend doesn't need verification, just rebalancing

	// don't know how to get a list of files/dirs on dropbox yet...
}

type DropBoxVerifier struct{}

func (v *DropBoxVerifier) Verify(path string, key Key, h string) error {
	// DropBox doesn't need verification
	return nil
}

func (v *DropBoxVerifier) VerifyKey(key Key) error {
	// DropBox doesn't need verification
	return nil
}

func (b DropBoxBackend) NewVerifier(c *Cluster) Verifier {
	return &DropBoxVerifier{}
}
