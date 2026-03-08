package main

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"
)

func TestVisitSleeps(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "disk_backend_aae_test")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	backend := newDiskBackend(tmpdir + "/")
	key, _ := keyFromString("sha1:f48dd853820860816c75d54d0f58d47663456009")
	data := []byte("test data")
	err = backend.Write(*key, io.NopCloser(bytes.NewReader(data)))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	path := backend.Root + key.Algorithm + "/" + key.AsPath() + "/data"
	f, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	n := newNode("local", "http://localhost:8080", true)
	c := newCluster(n, "test_secret", 10)
	s := site{
		Node:        n,
		Cluster:     c,
		Backend:     backend,
		AAEInterval: 1,
	}
	s.verifier = backend.NewVerifier(c)
	s.rebalancer = newRebalancer(c, s)

	start := time.Now()
	_ = visit(path, f, nil, c, s)
	elapsed := time.Since(start)

	if elapsed < 1*time.Second {
		t.Errorf("visit took %v, expected at least 1s", elapsed)
	}
}
