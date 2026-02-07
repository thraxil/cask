package main

import (
	"testing"
)

func Test_newCluster(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "clustersecret", 60)

	if c.Myself != n {
		t.Error("cluster wasn't created properly")
	}
	if !c.CheckSecret("clustersecret") {
		t.Error("doesn't accept correct secret")
	}
	if c.CheckSecret("wrongsecret") {
		t.Error("accept's a wrong secret")
	}
}

func Test_jsonSerialize(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "clustersecret", 60)

	d := "{\"uuid\":\"testuuid\",\"base_url\":\"http://localhost:1000\",\"writeable\":true,\"secret\":\"clustersecret\",\"neighbors\":null}"
	if string(c.jsonSerialize()) != d {
		t.Error("incorrect json")
	}
	// NodeMeta outputs the same
	if string(c.NodeMeta(10)) != d {
		t.Error("bad output from NodeMeta")
	}
	// LocalState outputs the same
	if string(c.LocalState(true)) != d {
		t.Error("bad output from NodeMeta")
	}
}

func Test_addAndFind(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "clustersecret", 60)

	// add a neighbor
	n2 := newNode("testuuid2", "http://localhost:1001", true)
	c.AddNeighbor(*n2)

	// we should be able to retrieve them
	n3, ok := c.FindNeighborByUUID("testuuid2")
	if !ok {
		t.Error("failed to find neighbor")
	}
	if n2.UUID != n3.UUID {
		t.Error("UUID mismatch", n2.UUID, n3.UUID)
	}

	// remove the neighbor
	c.RemoveNeighbor(*n2)
	// we should not be able to retrieve them anymore
	n3, ok = c.FindNeighborByUUID("testuuid2")
	if ok {
		t.Error("neighbor was not removed")
	}
}

func TestWriteableNeighbors(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "clustersecret", 60)

	// add a writeable neighbor
	n2 := newNode("testuuid2", "http://localhost:1001", true)
	c.AddNeighbor(*n2)

	// add a non-writeable neighbor
	n3 := newNode("testuuid3", "http://localhost:1002", false)
	c.AddNeighbor(*n3)

	w := c.WriteableNeighbors()
	if len(w) != 2 {
		t.Errorf("Expected 2 writeable neighbors, got %d", len(w))
	}

	for _, wn := range w {
		if !wn.Writeable {
			t.Errorf("Got a non-writeable node in the list: %v", wn)
		}
	}
}

func TestWriteRing(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "clustersecret", 60)

	// add a writeable neighbor
	n2 := newNode("testuuid2", "http://localhost:1001", true)
	c.AddNeighbor(*n2)

	// add another writeable neighbor
	n3 := newNode("testuuid3", "http://localhost:1002", true)
	c.AddNeighbor(*n3)

	r := c.WriteRing()

	if len(r) != 3*replicas {
		t.Errorf("Expected ring length of %d, but got %d", 3*replicas, len(r))
	}
}

func TestWriteOrder(t *testing.T) {
	n1 := newNode("a", "http://localhost:1000", true)
	n2 := newNode("b", "http://localhost:1001", true)
	n3 := newNode("c", "http://localhost:1002", true)
	c := newCluster(n1, "clustersecret", 60)
	c.AddNeighbor(*n2)
	c.AddNeighbor(*n3)

	// This hash should put a, b, c in order
	hash := "sha1:a94a8fe5ccb19ba61c4c0873d391e987982fbbd3"
	w := c.WriteOrder(hash)

	if len(w) != 3 {
		t.Fatalf("Expected 3 nodes, got %d", len(w))
	}

	if w[0].UUID != "a" {
		t.Errorf("Expected first node to be a, but got %s", w[0].UUID)
	}
	if w[1].UUID != "b" {
		t.Errorf("Expected second node to be b, but got %s", w[1].UUID)
	}
	if w[2].UUID != "c" {
		t.Errorf("Expected third node to be c, but got %s", w[2].UUID)
	}
}
