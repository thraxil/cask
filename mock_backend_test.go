package main

import (
	"io"
)

type MockBackend struct {
	freeSpace uint64
}

func (m MockBackend) String() string                        { return "mock" }
func (m MockBackend) Write(key key, r io.ReadCloser) error  { return nil }
func (m MockBackend) Read(key key) ([]byte, error)          { return nil, nil }
func (m MockBackend) Exists(key key) bool                   { return false }
func (m MockBackend) Delete(key key) error                  { return nil }
func (m MockBackend) ActiveAntiEntropy(c *cluster, s site, i int) {}
func (m MockBackend) NewVerifier(c *cluster) verifier       { return nil }
func (m MockBackend) FreeSpace() uint64                     { return m.freeSpace }
