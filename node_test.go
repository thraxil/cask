package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func Test_Unhealthy(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	n.LastSeen = time.Now()
	// LastFailed before LastSeen => Healthy
	n.LastFailed = n.LastSeen.Add(-1 * time.Minute)
	if n.Unhealthy() {
		t.Error("node should be healthy")
	}

	// LastFailed after LastSeen => Unhealthy
	n.LastFailed = n.LastSeen.Add(1 * time.Minute)
	if !n.Unhealthy() {
		t.Error("node should be unhealthy")
	}
}

func Test_URLGenerators(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	if n.AddFileURL() != "http://localhost:1000/local/" {
		t.Errorf("AddFileURL mismatch: %s", n.AddFileURL())
	}

	k, err := keyFromString("sha1:0000000000000000000000000000000000000000")
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	expected := "http://localhost:1000/local/sha1:0000000000000000000000000000000000000000/"
	if n.retrieveURL(*k) != expected {
		t.Errorf("retrieveURL mismatch: %s", n.retrieveURL(*k))
	}
	if n.retrieveInfoURL(*k) != expected {
		t.Errorf("retrieveInfoURL mismatch: %s", n.retrieveInfoURL(*k))
	}
}

func Test_AddFile(t *testing.T) {
	// Success
	{
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("Expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/local/" {
				t.Errorf("Expected path /local/, got %s", r.URL.Path)
			}
			// Check secret
			if r.Header.Get("X-Cask-Cluster-Secret") != "secret" {
				t.Errorf("Expected secret header, got %s", r.Header.Get("X-Cask-Cluster-Secret"))
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709"))
		}))
		defer server.Close()

		n := newNode("testuuid", server.URL, true)
		k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
		// sha1 of empty string is da39a3ee5e6b4b0d3255bfef95601890afd80709
		r := strings.NewReader("")
		if !n.AddFile(*k, r, "secret") {
			t.Error("AddFile returned false on success")
		}
	}

	// Server Error
	{
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		n := newNode("testuuid", server.URL, true)
		k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
		r := strings.NewReader("")
		if n.AddFile(*k, r, "secret") {
			t.Error("AddFile returned true on server error")
		}
	}

	// Wrong key returned
	{
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("sha1:otherkey"))
		}))
		defer server.Close()

		n := newNode("testuuid", server.URL, true)
		k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
		r := strings.NewReader("")
		if n.AddFile(*k, r, "secret") {
			t.Error("AddFile returned true when wrong key returned")
		}
	}

	// Invalid URL
	{
		n := newNode("testuuid", ":::invalid-url:::", true)
		k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
		r := strings.NewReader("")
		if n.AddFile(*k, r, "secret") {
			t.Error("AddFile returned true on invalid URL")
		}
	}
}

func Test_postFile(t *testing.T) {
	// Success
	{
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("Expected POST, got %s", r.Method)
			}
			// Check multipart
			err := r.ParseMultipartForm(1024)
			if err != nil {
				t.Errorf("failed to parse multipart form: %v", err)
			}
			file, _, err := r.FormFile("file")
			if err != nil {
				t.Errorf("failed to get file from form: %v", err)
			}
			defer file.Close()
			content, _ := io.ReadAll(file)
			if string(content) != "test content" {
				t.Errorf("Expected 'test content', got '%s'", string(content))
			}

			if r.Header.Get("X-Cask-Cluster-Secret") != "secret" {
				t.Errorf("Expected secret header, got %s", r.Header.Get("X-Cask-Cluster-Secret"))
			}

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		r := strings.NewReader("test content")
		resp, err := postFile(r, server.URL, "secret")
		if err != nil {
			t.Fatalf("postFile failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
	}

	// Invalid URL
	{
		r := strings.NewReader("test content")
		_, err := postFile(r, ":::invalid-url:::", "secret")
		if err == nil {
			t.Error("postFile should have failed with invalid URL")
		}
	}
}