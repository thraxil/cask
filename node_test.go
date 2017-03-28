package main

import (
	"testing"
	"time"
)

func Test_newNode(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	if n.UUID != "testuuid" {
		t.Error("node wasn't created properly")
	}
}

func Test_DateFormatting(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	n.LastSeen, _ = time.Parse("Mon Jan 2 15:04:05 -0700 MST 2006", "Mon Jan 2 15:04:05 -0700 MST 2006")
	if n.LastSeenFormatted() != "2006-01-02 15:04:05" {
		t.Error("wrong formatted date for LastSeen")
	}
	n.LastFailed, _ = time.Parse("Mon Jan 2 15:04:05 -0700 MST 2006", "Mon Jan 2 15:04:05 -0700 MST 2006")
	if n.LastFailedFormatted() != "2006-01-02 15:04:05" {
		t.Error("wrong formatted date for LastFailed")
	}
}

func Test_HashKeys(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	keys := n.HashKeys()
	if len(keys) != replicas {
		t.Error("wrong number of keys")
	}
}
