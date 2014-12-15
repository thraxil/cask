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

type DiskBackend struct {
	Root string
}

func NewDiskBackend(root string) *DiskBackend {
	return &DiskBackend{Root: root}
}

func (d *DiskBackend) Write(key Key, r io.Reader) {
	path := d.Root + key.Algorithm + "/" + key.AsPath()
	log.Println(fmt.Sprintf("writing to %s\n", path))
	os.MkdirAll(path, 0755)
	fullpath := path + "/data"
	f, _ := os.OpenFile(fullpath, os.O_CREATE|os.O_RDWR, 0644)
	defer f.Close()
	io.Copy(f, r)
}

func (d DiskBackend) Read(key Key) ([]byte, error) {
	path := d.Root + key.Algorithm + "/" + key.AsPath() + "/data"
	return ioutil.ReadFile(path)
}

func (d DiskBackend) Exists(key Key) bool {
	path := d.Root + key.Algorithm + "/" + key.AsPath() + "/data"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func (d DiskBackend) Delete(key Key) error {
	path := d.Root + key.Algorithm + "/" + key.AsPath()
	return os.RemoveAll(path)
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
	repaired, err := repair_file(path, key, c)
	if err != nil {
		log.Printf("error trying to repair file")
		return err
	}
	if repaired {
		log.Printf("successfully repaired file")
		return nil
	}
	return errors.New("unrepairable file")
}

func repair_file(path string, key Key, c *Cluster) (bool, error) {
	nodes_to_check := c.ReadOrder(key.String())
	for _, n := range nodes_to_check {
		if n.UUID == c.Myself.UUID {

			continue
		}
		found, f, err := n.CheckFile(key)
		if found && err == nil {
			err := replaceFile(path, f)
			if err != nil {
				log.Println("error replacing the file")
				continue
			}
			return true, nil
		}

	}

	return false, errors.New("no good copies found")
}

func replaceFile(path string, file []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Println("couldn't open for writing")
		f.Close()
		return err
	}
	_, err = f.Write(file)
	f.Close()
	if err != nil {
		log.Println("couldn't write file")
		return err
	}
	return nil
}

func visit(path string, f os.FileInfo, err error, c *Cluster, s Site) error {
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

	log.Printf("AAE visiting %s\n", path)

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

	r := NewRebalancer(path, *key, c, s)
	err = r.Rebalance()
	if err != nil {
		return err
	}

	// slow things down a little to keep server load down
	var base_time = 10
	jitter := rand.Intn(5)
	time.Sleep(time.Duration(base_time+jitter) * time.Second)
	return nil
}

func makeVisitor(fn func(string, os.FileInfo, error, *Cluster, Site) error,
	c *Cluster, s Site) func(path string, f os.FileInfo, err error) error {
	return func(path string, f os.FileInfo, err error) error {
		return fn(path, f, err, c, s)
	}
}

func (d DiskBackend) ActiveAntiEntropy(cluster *Cluster, site Site) {
	var base_time = 10
	var jitter = 1
	for {
		jitter = rand.Intn(5)
		time.Sleep(time.Duration(base_time+jitter) * time.Second)
		log.Println("AAE starting at the top")
		err := filepath.Walk(d.Root, makeVisitor(visit, cluster, site))
		if err != nil {
			log.Printf("filepath.Walk() returned %v\n", err)
		}
	}
}
