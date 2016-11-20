package main

import "testing"

func Test_KeyFromPath(t *testing.T) {
	k, err := keyFromPath("sha1/ae/28/60/5f/0f/fc/34/fe/53/14/34/2f/78/ef/aa/13/ee/45/f6/99/data")
	if err != nil {
		t.Error("bad key")
	}
	if k.String() != "sha1:ae28605f0ffc34fe5314342f78efaa13ee45f699" {
		t.Error("didn't get the right key")
	}

	k, err = keyFromPath("/tmp/cask0/sha1/ae/28/60/5f/0f/fc/34/fe/53/14/34/2f/78/ef/aa/13/ee/45/f6/99/data")
	if err != nil {
		t.Error("error on full key")
	}
	if k != nil && k.String() != "sha1:ae28605f0ffc34fe5314342f78efaa13ee45f699" {
		t.Error("failed on full path")
	}

}
