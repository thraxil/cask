package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
)

func TestDiskBackendWriteAndRead(t *testing.T) {
	// Create a temporary directory for testing
	tmpdir, err := ioutil.TempDir("", "disk_backend_test")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	// Create a new diskBackend instance
	backend := newDiskBackend(tmpdir + "/")

	// Create a test key and data
	key, err := keyFromString("sha1:a94a8fe5ccb19ba61c4c0873d391e987982fbbd3")
	if err != nil {
		t.Fatalf("keyFromString failed: %v", err)
	}
	data := []byte("test data")

	// Write the data to the backend
	err = backend.Write(*key, ioutil.NopCloser(bytes.NewReader(data)))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read the data back from the backend
	readData, err := backend.Read(*key)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Check if the read data is the same as the original data
	if !bytes.Equal(data, readData) {
		t.Errorf("Read data does not match written data. Got %q, want %q", readData, data)
	}
}

func TestDiskBackendExists(t *testing.T) {
	// Create a temporary directory for testing
	tmpdir, err := ioutil.TempDir("", "disk_backend_test")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	// Create a new diskBackend instance
	backend := newDiskBackend(tmpdir + "/")

	// Create a test key
	key, err := keyFromString("sha1:a94a8fe5ccb19ba61c4c0873d391e987982fbbd3")
	if err != nil {
		t.Fatalf("keyFromString failed: %v", err)
	}

	// Check that the key doesn't exist
	if backend.Exists(*key) {
		t.Errorf("Key should not exist yet")
	}

	// Write the key to the backend
	data := []byte("test data")
	err = backend.Write(*key, ioutil.NopCloser(bytes.NewReader(data)))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Check that the key exists
	if !backend.Exists(*key) {
		t.Errorf("Key should exist")
	}
}

func TestDiskBackendDelete(t *testing.T) {
	// Create a temporary directory for testing
	tmpdir, err := ioutil.TempDir("", "disk_backend_test")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	// Create a new diskBackend instance
	backend := newDiskBackend(tmpdir + "/")

	// Create a test key and data
	key, err := keyFromString("sha1:a94a8fe5ccb19ba61c4c0873d391e987982fbbd3")
	if err != nil {
		t.Fatalf("keyFromString failed: %v", err)
	}
	data := []byte("test data")

	// Write the data to the backend
	err = backend.Write(*key, ioutil.NopCloser(bytes.NewReader(data)))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Check that the key exists
	if !backend.Exists(*key) {
		t.Fatalf("Key should exist before delete")
	}

	// Delete the key
	err = backend.Delete(*key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Check that the key no longer exists
	if backend.Exists(*key) {
		t.Errorf("Key should not exist after delete")
	}
}

func TestDiskBackendWriteError(t *testing.T) {
	// Create a temporary directory for testing
	tmpdir, err := ioutil.TempDir("", "disk_backend_test")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	// Create a new diskBackend instance
	backend := newDiskBackend(tmpdir + "/")

	// Create a test key and data
	key, err := keyFromString("sha1:a94a8fe5ccb19ba61c4c0873d391e987982fbbd3")
	if err != nil {
		t.Fatalf("keyFromString failed: %v", err)
	}
	data := []byte("test data")

	// Create a file that will conflict with the directory creation
	conflictingDirPath := tmpdir + "/sha1"
	err = os.MkdirAll(conflictingDirPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create conflicting directory: %v", err)
	}
	conflictingFilePath := conflictingDirPath + "/a9"
	f, err := os.Create(conflictingFilePath)
	if err != nil {
		t.Fatalf("Failed to create conflicting file: %v", err)
	}
	f.Close()

	// Write the data to the backend, expecting an error
	err = backend.Write(*key, ioutil.NopCloser(bytes.NewReader(data)))
	if err == nil {
		t.Errorf("Write should have failed but it didn't")
	}
}

func TestDiskVerifierVerify(t *testing.T) {
	// Create a temporary directory for testing
	tmpdir, err := ioutil.TempDir("", "disk_verifier_test")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	// Create a new diskBackend instance
	backend := newDiskBackend(tmpdir + "/")

	// Create a test key and data
	key, err := keyFromString("sha1:a94a8fe5ccb19ba61c4c0873d391e987982fbbd3")
	if err != nil {
		t.Fatalf("keyFromString failed: %v", err)
	}
	data := []byte("test data")

	// Write the data to the backend
	err = backend.Write(*key, ioutil.NopCloser(bytes.NewReader(data)))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Create a verifier
	verifier := backend.NewVerifier(nil) // Pass nil for cluster for this test

	// Verify the file with the correct hash
	path := backend.Root + key.Algorithm + "/" + key.AsPath() + "/data"
	hash := "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3"
	err = verifier.Verify(path, *key, hash)
	if err != nil {
		t.Errorf("Verify failed with correct hash: %v", err)
	}
}

func TestDiskVerifierVerifyBadHash(t *testing.T) {
	// Create a temporary directory for testing
	tmpdir, err := ioutil.TempDir("", "disk_verifier_test")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	// Create a new diskBackend instance
	backend := newDiskBackend(tmpdir + "/")

	// Create a test key and data
	key, err := keyFromString("sha1:a94a8fe5ccb19ba61c4c0873d391e987982fbbd3")
	if err != nil {
		t.Fatalf("keyFromString failed: %v", err)
	}
	data := []byte("test data")

	// Write the data to the backend
	err = backend.Write(*key, ioutil.NopCloser(bytes.NewReader(data)))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Create a mock cluster
	n := newNode("local", "http://localhost:8080", true)
	c := newCluster(n, "test_secret", 10)


	// Create a verifier
	verifier := backend.NewVerifier(c)

	// Verify the file with an incorrect hash
	path := backend.Root + key.Algorithm + "/" + key.AsPath() + "/data"
	hash := "deadbeef"
	err = verifier.Verify(path, *key, hash)
	if err == nil {
		t.Errorf("Verify should have failed with bad hash")
	}
}
