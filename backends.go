package main

import (
	"fmt"
	"io"
	"os"
)

type Backend interface {
	Write(key Key, r io.Reader)
	Read(key Key) ([]byte, error)
	Exists(key Key) bool
}

type DiskBackend struct {
	Root string
}

func NewDiskBackend(root string) *DiskBackend {
	return &DiskBackend{Root: root}
}

func (d *DiskBackend) Write(key Key, r io.Reader) {
	path := d.Root + key.AsPath()
	fmt.Printf("writing to %s\n", path)
	os.MkdirAll(path, 0755)
	fullpath := path + "/data"
	f, _ := os.OpenFile(fullpath, os.O_CREATE|os.O_RDWR, 0644)
	defer f.Close()
	io.Copy(f, r)
}

func (d DiskBackend) Read(key Key) ([]byte, error) {
	return []byte(""), nil
}

func (d DiskBackend) Exists(key Key) bool {
	return true
}
