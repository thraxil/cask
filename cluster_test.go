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
