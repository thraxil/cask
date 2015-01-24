package main

import (
	"fmt"
	"io"
)

type Backend interface {
	fmt.Stringer
	Write(key Key, r io.ReadCloser) error
	Read(key Key) ([]byte, error)
	Exists(key Key) bool
	Delete(key Key) error
	ActiveAntiEntropy(cluster *Cluster, site Site, interval int)
	NewVerifier(cluster *Cluster) Verifier
}
