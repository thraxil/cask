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
