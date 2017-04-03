package main

import (
	"fmt"
	"io"
)

type backend interface {
	fmt.Stringer
	Write(key, io.ReadCloser) error
	Read(key) ([]byte, error)
	Exists(key) bool
	Delete(key) error
	ActiveAntiEntropy(*cluster, site, int)
	NewVerifier(*cluster) verifier
	FreeSpace() uint64
}
