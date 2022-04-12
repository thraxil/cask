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
	"syscall"
	"time"

	"github.com/thraxil/randwalk"
)

type diskBackend struct {
	Root string
}

func newDiskBackend(root string) *diskBackend {
	return &diskBackend{Root: root}
}

func (d diskBackend) String() string {
	return "Disk"
}

func (d *diskBackend) Write(key key, r io.ReadCloser) error {
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

func (d diskBackend) Read(key key) ([]byte, error) {
	path := d.Root + key.Algorithm + "/" + key.AsPath() + "/data"
	return ioutil.ReadFile(path)
}

func (d diskBackend) Exists(key key) bool {
	path := d.Root + key.Algorithm + "/" + key.AsPath() + "/data"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func (d diskBackend) Delete(key key) error {
	path := d.Root + key.Algorithm + "/" + key.AsPath()
	return os.RemoveAll(path)
}

// the only File methods that we care about
// makes it easier to mock
type fileish interface {
	IsDir() bool
	Name() string
}

// part of the path that's not a directory or extension
func basename(path string) string {
	filename := filepath.Base(path)
	ext := filepath.Ext(filename)
	return filename[:len(filename)-len(ext)]
}

func visitPreChecks(path string, f fileish, err error, c *cluster) (bool, error) {
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

type diskVerifier struct {
	b   diskBackend
	c   *cluster
	chF chan func()
}

func (d diskBackend) NewVerifier(c *cluster) verifier {
	v := &diskVerifier{
		b:   d,
		c:   c,
		chF: make(chan func()),
	}
	go v.run()
	return v
}

func (v *diskVerifier) run() {
	for f := range v.chF {
		f()
	}
}

func (v *diskVerifier) Verify(path string, key key, h string) error {
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
func (v *diskVerifier) VerifyKey(key key) error {
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

func (v *diskVerifier) doVerify(path string, key key, h string) error {
	if key.String() == "sha1:"+h {
		return nil
	}
	log.Printf("corrupted file %s\n", path)
	repaired, err := v.repairFile(path, key)
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

func (v *diskVerifier) repairFile(path string, key key) (bool, error) {
	nodesToCheck := v.c.ReadOrder(key.String())
	for _, n := range nodesToCheck {
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

func (v *diskVerifier) replaceFile(path string, file []byte) error {
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

func visit(path string, f os.FileInfo, err error, c *cluster, s site) error {
	if aaeSkip < aaeOffset {
		aaeSkip++
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

	key, err := keyFromPath(path)
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
	var baseTime = 10
	jitter := rand.Intn(5)
	time.Sleep(time.Duration(baseTime+jitter) * time.Second)
	return nil
}

func makeVisitor(fn func(string, os.FileInfo, error, *cluster, site) error,
	c *cluster, s site) func(path string, f os.FileInfo, err error) error {
	return func(path string, f os.FileInfo, err error) error {
		return fn(path, f, err, c, s)
	}
}

var aaeOffset = 0
var aaeSkip = 0

func (d diskBackend) ActiveAntiEntropy(cluster *cluster, site site, interval int) {
	aaeOffset = rand.Intn(10000)
	var jitter = 1
	for {
		_, err := ioutil.ReadDir(d.Root)
		if err != nil {
			fmt.Printf("Can't get a directory listing for %s. Let's fail fast.\n", d.Root)
			os.Exit(1)
		}
		if aaeSkip >= aaeOffset {
			jitter = rand.Intn(5)
			time.Sleep(time.Duration(interval+jitter) * time.Second)
			log.Println("AAE starting at the top")
		}
		err = randwalk.Walk(d.Root, makeVisitor(visit, cluster, site))
		if err != nil {
			log.Printf("randwalk.Walk() returned %v\n", err)
		}
	}
}

func (d diskBackend) FreeSpace() uint64 {
	var stat syscall.Statfs_t
	syscall.Statfs(d.Root, &stat)
	return stat.Bavail * uint64(stat.Bsize)
}
