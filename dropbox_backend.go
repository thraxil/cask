package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/stacktic/dropbox"
)

type dropboxBackend struct {
	AccessKey string
	SecretKey string
	Token     string
	db        *dropbox.Dropbox
}

func newDropboxBackend(accessKey, secretKey, token string) *dropboxBackend {
	db := dropbox.NewDropbox()
	db.SetAppInfo(accessKey, secretKey)

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

	return &dropboxBackend{accessKey, secretKey, token, db}
}

func (d dropboxBackend) String() string {
	return "DropBox"
}

func (d *dropboxBackend) Write(key key, r io.ReadCloser) error {
	path := key.Algorithm + "/" + key.AsPath()
	// ignore error on this since it means that the path
	// already exists
	d.db.CreateFolder(path)

	_, err := d.db.UploadByChunk(r, 1024*1024, path+"/data", true, "")
	if err != nil {
		log.Println("uh oh. couldn't write to dropbox")
		log.Println(err)
		return err
	}
	return nil
}

func (d dropboxBackend) Read(key key) ([]byte, error) {
	path := key.Algorithm + "/" + key.AsPath()
	r, _, err := d.db.Download(path+"/data", "", 0)
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

func (d dropboxBackend) Exists(key key) bool {
	path := key.Algorithm + "/" + key.AsPath()
	ent, err := d.db.Metadata(path+"/data", false, false, "", "", 1)

	if err != nil || ent == nil {
		return false
	}
	return true
}

func (d *dropboxBackend) Delete(key key) error {
	_, err := d.db.Delete(key.Algorithm + "/" + key.AsPath() + "/data")
	return err
}

func (d *dropboxBackend) AAEEntry(e dropbox.Entry, site site, interval int) {
	if e.IsDir {
		ent, err := d.db.Metadata(e.Path, true, false, "", "", -1)
		if err != nil {
			log.Println(err)
			return
		}
		n := len(ent.Contents)
		idxes := rand.Perm(n)
		for _, i := range idxes {
			d.AAEEntry(ent.Contents[i], site, interval)
		}
	} else {
		log.Println(e.Path)
		k, err := keyFromPath(e.Path)
		if err != nil {
			log.Println("couldn't make key from path")
			return
		}
		err = site.Rebalance(*k)
		if err != nil {
			log.Println(err)
		}
		jitter := rand.Intn(5)
		time.Sleep(time.Duration(interval+jitter) * time.Second)
	}
}

func (d *dropboxBackend) ActiveAntiEntropy(cluster *cluster, site site, interval int) {
	log.Println("DropBox AAE starting")
	// DropBox backend doesn't need verification, just rebalancing
	rand.Seed(time.Now().UnixNano())
	for {
		log.Println("AAE starting at the top")

		ent, err := d.db.Metadata("", true, false, "", "", -1)
		if err != nil {
			log.Println(err)
			return
		}
		n := len(ent.Contents)
		idxes := rand.Perm(n)
		for _, i := range idxes {
			d.AAEEntry(ent.Contents[i], site, interval)
		}
	}
}

type dropBoxVerifier struct{}

func (v *dropBoxVerifier) Verify(path string, key key, h string) error {
	// DropBox doesn't need verification
	return nil
}

func (v *dropBoxVerifier) VerifyKey(key key) error {
	// DropBox doesn't need verification
	return nil
}

func (d dropboxBackend) NewVerifier(c *cluster) verifier {
	return &dropBoxVerifier{}
}

func (d dropboxBackend) FreeSpace() uint64 {
	account, _ := d.db.GetAccountInfo()
	// TODO: this is just returning the quota for now
	// not the amount remaining
	return uint64(account.QuotaInfo.Quota)
}
