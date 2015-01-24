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

func (d DiskBackend) String() string {
	return "Disk"
}

func (d *DiskBackend) Write(key Key, r io.Reader) error {
	path := d.Root + key.Algorithm + "/" + key.AsPath()
	log.Println(fmt.Sprintf("writing to %s\n", path))
	err := os.MkdirAll(path, 0755)
	if err != nil {
		log.Println("couldn't make directory path")
		log.Println(err)
		return err
	}
	fullpath := path + "/data"
	f, err := os.OpenFile(fullpath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Println("couldn't write file")
		log.Println(err)
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	if err != nil {
		log.Println("error copying data into file")
		log.Println(err)
		return err
	}
	return nil
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

type DiskVerifier struct {
	b   DiskBackend
	c   *Cluster
	chF chan func()
}

func (b DiskBackend) NewVerifier(c *Cluster) Verifier {
	v := &DiskVerifier{
		b:   b,
		c:   c,
		chF: make(chan func()),
	}
	go v.run()
	return v
}

func (v *DiskVerifier) run() {
	for f := range v.chF {
		f()
	}
}

func (v *DiskVerifier) Verify(path string, key Key, h string) error {
	r := make(chan error)
	go func() {
		v.chF <- func() {
			r <- v.doVerify(path, key, h)
		}
	}()
	return <-r
}

// does the same thing as Verify(), but given only the key
// so it is expected to get the path, compute the hash of the
// file and then do the verify.
func (v *DiskVerifier) VerifyKey(key Key) error {
	r := make(chan error)
	go func() {
		v.chF <- func() {
			path := v.b.Root + key.Algorithm + "/" + key.AsPath() + "/data"
			h := sha1.New()
			file, err := os.Open(path)
			defer file.Close()
			if err != nil {
				log.Printf("error opening %s\n", path)
				return
			}
			_, err = io.Copy(h, file)
			if err != nil {
				log.Printf("error copying %s\n", path)
				return
			}
			hash := fmt.Sprintf("%x", h.Sum(nil))
			r <- v.doVerify(path, key, hash)
		}
	}()
	return <-r
}

func (v *DiskVerifier) doVerify(path string, key Key, h string) error {
	if key.String() == "sha1:"+h {
		return nil
	}
	log.Printf("corrupted file %s\n", path)
	repaired, err := v.repair_file(path, key)
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

func (v *DiskVerifier) repair_file(path string, key Key) (bool, error) {
	nodes_to_check := v.c.ReadOrder(key.String())
	for _, n := range nodes_to_check {
		if n.UUID == v.c.Myself.UUID {
			continue
		}
		found, f, err := n.CheckFile(key, v.c.secret)
		if found && err == nil {
			err := v.replaceFile(path, f)
			if err != nil {
				log.Println("error replacing the file")
				continue
			}
			return true, nil
		}

	}

	return false, errors.New("no good copies found")
}

func (v *DiskVerifier) replaceFile(path string, file []byte) error {
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
	if AAE_SKIP < AAE_OFFSET {
		AAE_SKIP++
		return nil
	}

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
	err = s.Verify(path, *key, hash)
	if err != nil {
		return err
	}

	err = s.Rebalance(*key)
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

var AAE_OFFSET = 0
var AAE_SKIP = 0

func (d DiskBackend) ActiveAntiEntropy(cluster *Cluster, site Site, interval int) {
	AAE_OFFSET = rand.Intn(10000)
	var jitter = 1
	for {
		_, err := ioutil.ReadDir(d.Root)
		if err != nil {
			fmt.Printf("Can't get a directory listing for %s. Let's fail fast.\n", d.Root)
			os.Exit(1)
		}
		if AAE_SKIP >= AAE_OFFSET {
			jitter = rand.Intn(5)
			time.Sleep(time.Duration(interval+jitter) * time.Second)
			log.Println("AAE starting at the top")
		}
		err = filepath.Walk(d.Root, makeVisitor(visit, cluster, site))
		if err != nil {
			log.Printf("filepath.Walk() returned %v\n", err)
		}
	}
}
