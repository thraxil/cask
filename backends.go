package main

import "io"

type Backend interface {
	Write(key Key, r io.Reader)
	Read(key Key) ([]byte, error)
	Exists(key Key) bool
	Delete(key Key) error
	ActiveAntiEntropy(cluster *Cluster, site Site, interval int)
}
