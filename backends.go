package main

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

type Backend interface {
	Write(key Key, r io.Reader)
	Read(key Key) ([]byte, error)
	Exists(key Key) bool
	ActiveAntiEntropy(cluster *Cluster)
}

type DiskBackend struct {
	Root string
}

func NewDiskBackend(root string) *DiskBackend {
	return &DiskBackend{Root: root}
}

func (d *DiskBackend) Write(key Key, r io.Reader) {
	path := d.Root + key.AsPath()
	log.Println(fmt.Sprintf("writing to %s\n", path))
	os.MkdirAll(path, 0755)
	fullpath := path + "/data"
	f, _ := os.OpenFile(fullpath, os.O_CREATE|os.O_RDWR, 0644)
	defer f.Close()
	io.Copy(f, r)
}

func (d DiskBackend) Read(key Key) ([]byte, error) {
	path := d.Root + key.AsPath() + "/data"
	return ioutil.ReadFile(path)
}

func (d DiskBackend) Exists(key Key) bool {
	path := d.Root + key.AsPath() + "/data"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

// the only File methods that we care about
// makes it easier to mock
type FileIsh interface {
	IsDir() bool
	Name() string
}

// part of the path that's not a directory or extension
func basename(path string) string {
	filename := filepath.Base(path)
	ext := filepath.Ext(filename)
	return filename[:len(filename)-len(ext)]
}

func visitPreChecks(path string, f FileIsh, err error, c *Cluster) (bool, error) {
	if err != nil {
		log.Printf("visit was handed an error: %s", err.Error())
		return true, err
	}
	if c == nil {
		log.Println("verifier.visit was given a nil cluster")
		return true, errors.New("nil cluster")
	}
	// all we care about is the "full" version of each
	if f.IsDir() {
		return true, nil
	}
	if basename(path) != "data" {
		return true, nil
	}
	return false, nil
}

func verify(path string, key Key, h string, c *Cluster) error {
	if key.String() == "sha1:"+h {
		return nil
	}
	log.Printf("corrupted file %s\n", path)
	return nil
}

func visit(path string, f os.FileInfo, err error, c *Cluster) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Error in active anti-entropy.visit() [%s] %s\n", c.Myself.UUID, path)
			log.Println(r)
		}
	}()

	done, err := visitPreChecks(path, f, err, c)
	if done {
		return err
	}

	log.Printf("active anti-entropy visiting %s\n", path)

	key, err := KeyFromPath(path)
	if err != nil {
		log.Println("couldn't get key from path")
		return nil
	}
	h := sha1.New()
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		log.Printf("error opening %s\n", path)
		return err
	}
	_, err = io.Copy(h, file)
	if err != nil {
		log.Printf("error copying %s\n", path)
		return err
	}
	hash := fmt.Sprintf("%x", h.Sum(nil))
	err = verify(path, *key, hash, c)
	if err != nil {
		return err
	}

	// rebalance

	// slow things down a little to keep server load down
	var base_time = 10
	jitter := rand.Intn(5)
	time.Sleep(time.Duration(base_time+jitter) * time.Second)
	return nil
}

func makeVisitor(fn func(string, os.FileInfo, error, *Cluster) error,
	c *Cluster) func(path string, f os.FileInfo, err error) error {
	return func(path string, f os.FileInfo, err error) error {
		return fn(path, f, err, c)
	}
}

func (d DiskBackend) ActiveAntiEntropy(cluster *Cluster) {
	for {
		log.Println("active anti-entropy starting at the top")
		err := filepath.Walk(d.Root, makeVisitor(visit, cluster))
		if err != nil {
			log.Printf("filepath.Walk() returned %v\n", err)
		}
	}
}
