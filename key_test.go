package main

import (
	"fmt"
	"testing"
)

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

func Test_KeyFromPathExceptions(t *testing.T) {
	// too few parts
	_, err := keyFromPath("sha1/ae/28/60/5f/0f/fc/34/fe/53/")
	if err == nil {
		t.Error("not enough parts for a valid key")
	}
	// too many parts
	_, err = keyFromPath("sha1/ae/28/60/5f/0f/fc/34/fe/53/28/60/5f/0f/fc/34/fe/53/14/34/2f/78/ef/aa/13/ee/45/f6/99/data")
	if err == nil {
		t.Error("too many parts for a valid key")
	}

	// right number of parts, but not enough chars
	_, err = keyFromPath("sha1/a/28/60/5f/0f/fc/34/fe/53/14/34/2f/78/ef/aa/13/ee/45/f6/99/data")
	if err == nil {
		t.Error("must be 40 chars in hash")
	}
}

func Test_AsPath(t *testing.T) {
	p := "ae/28/60/5f/0f/fc/34/fe/53/14/34/2f/78/ef/aa/13/ee/45/f6/99"
	k, _ := keyFromPath("sha1/" + p + "/data")
	if p != k.AsPath() {
		t.Error("path came back changed")
	}
}

func Test_Valid(t *testing.T) {
	p := "ae/28/60/5f/0f/fc/34/fe/53/14/34/2f/78/ef/aa/13/ee/45/f6/99"
	k, _ := keyFromPath("sha1/" + p + "/data")

	if !k.Valid() {
		fmt.Println(k.Algorithm, k.String())
		t.Error("should be valid")
	}
}

func Test_keyFromString(t *testing.T) {
	// invalid algorithm
	_, err := keyFromString("foo:not valid")
	if err == nil {
		t.Error("'foo' is not a valid algorithm")
	}

	// valid algorithm, invalid hash length
	_, err = keyFromString("sha1:not enough chars")
	if err == nil {
		t.Error("sha1 hash must be 40 chars")
	}

	// valid
	k, err := keyFromString("sha1:ae28605f0ffc34fe5314342f78efaa13ee45f699")
	if err != nil {
		t.Error("valid keyFromString failed")
	}
	if k.String() != "sha1:ae28605f0ffc34fe5314342f78efaa13ee45f699" {
		t.Error("keyFromString returned wrong key")
	}
}

func Test_String(t *testing.T) {
	k := key{
		Algorithm: "sha1",
		Value:     []byte("ae28605f0ffc34fe5314342f78efaa13ee45f699"),
	}
	if k.String() != "sha1:ae28605f0ffc34fe5314342f78efaa13ee45f699" {
		t.Error("String() returned wrong value")
	}
}

func Test_Invalid(t *testing.T) {
	k := key{
		Algorithm: "sha256",
		Value:     []byte("ae28605f0ffc34fe5314342f78efaa13ee45f699"),
	}
	if k.Valid() {
		t.Error("should be invalid with wrong algorithm")
	}

	k = key{
		Algorithm: "sha1",
		Value:     []byte("short"),
	}
	if k.Valid() {
		t.Error("should be invalid with wrong length")
	}
}
