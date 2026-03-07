package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// MockBackendFull extends MockBackend to support reading and deleting
type MockBackendFull struct {
	MockBackend
	data       map[string][]byte
	deletedKey string
}

func (m *MockBackendFull) Read(k key) ([]byte, error) {
	if d, ok := m.data[k.String()]; ok {
		return d, nil
	}
	return nil, io.EOF
}

func (m *MockBackendFull) Delete(k key) error {
	m.deletedKey = k.String()
	return nil
}

func (m *MockBackendFull) FreeSpace() uint64 { return 1000 }

func Test_newRebalancer(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "secret", 60)
	s := site{Node: n, Cluster: c}
	r := newRebalancer(c, s)
	if r == nil {
		t.Error("failed to create rebalancer")
	}
}

func Test_Rebalance_NilCluster(t *testing.T) {
	r := rebalancer{c: nil}
	k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
	err := r.doRebalance(*k)
	if err == nil {
		t.Error("expected error on nil cluster")
	}
}

func Test_Rebalance_Simple(t *testing.T) {
	// Mock backend
	mb := &MockBackendFull{}
	
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "secret", 60)
	s := site{
		Node:           n,
		Cluster:        c,
		Backend:        mb,
		Replication:    1,
		MaxReplication: 2,
	}
	r := newRebalancer(c, s)
	
	k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
	
	// Rebalance when already satisfied (replication 1, and we are in the list)
	err := r.Rebalance(*k)
	if err != nil {
		t.Errorf("Rebalance failed: %v", err)
	}
}

func Test_Rebalance_ReplicateToNeighbor(t *testing.T) {
	// Mock neighbor
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709"))
			return
		}
	}))
	defer ts.Close()

	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "secret", 60)
	
	// Add neighbor
	n2 := newNode("neighbor", ts.URL, true)
	c.AddNeighbor(*n2)
	time.Sleep(50 * time.Millisecond)

	mb := &MockBackendFull{
		data: map[string][]byte{
			"sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709": []byte("some data"),
		},
	}
	
	s := site{
		Node:           n,
		Cluster:        c,
		Backend:        mb,
		Replication:    2,
		MaxReplication: 3,
	}
	r := newRebalancer(c, s)
	
	k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
	
	err := r.Rebalance(*k)
	if err != nil {
		t.Errorf("Rebalance failed: %v", err)
	}
}

func Test_Rebalance_DeleteLocal(t *testing.T) {
	mb := &MockBackendFull{
		data: map[string][]byte{
			"sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709": []byte(""),
		},
	}
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "secret", 60)
	s := site{Backend: mb}
	r := rebalancer{c: c, s: s}
	
	k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
	r.cleanUpExcessReplica(*k)
	
	if mb.deletedKey != "sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709" {
		t.Error("Delete was not called on backend")
	}
}

func Test_Rebalance_ReplicateToNeighbor_Fail(t *testing.T) {
	// Mock neighbor that fails
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "secret", 60)
	n2 := newNode("neighbor", ts.URL, true)
	c.AddNeighbor(*n2)
	time.Sleep(50 * time.Millisecond)

	mb := &MockBackendFull{
		data: map[string][]byte{
			"sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709": []byte("some data"),
		},
	}
	s := site{Node: n, Cluster: c, Backend: mb, Replication: 2}
	r := rebalancer{c: c, s: s}
	k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
	
	// Should fail to satisfy replication but still complete
	err := r.doRebalance(*k)
	if err != nil {
		t.Errorf("doRebalance failed: %v", err)
	}
}

func Test_Rebalance_BackendReadError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "secret", 60)
	n2 := newNode("neighbor", ts.URL, true)
	c.AddNeighbor(*n2)
	time.Sleep(50 * time.Millisecond)

	mb := &MockBackendFull{
		data: map[string][]byte{}, // Empty data will cause Read error
	}
	s := site{Node: n, Cluster: c, Backend: mb, Replication: 2}
	r := rebalancer{c: c, s: s}
	k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
	
	err := r.doRebalance(*k)
	if err != nil {
		t.Errorf("doRebalance failed: %v", err)
	}
}

func Test_cleanUpExcessReplica_Error(t *testing.T) {
	mb := &MockBackendError{}
	s := site{Backend: mb}
	r := rebalancer{s: s}
	k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
	r.cleanUpExcessReplica(*k)
}

type MockBackendError struct {
	MockBackend
}

func (m MockBackendError) Delete(k key) error {
	return io.EOF
}

func Test_retrieveReplica_UnwriteableNode(t *testing.T) {
	n := newNode("neighbor", "http://localhost:1001", false)
	c := &cluster{secret: "secret"}
	r := rebalancer{c: c}
	k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
	if r.retrieveReplica(*k, *n, false) != 0 {
		t.Error("should return 0 for unwriteable node")
	}
}