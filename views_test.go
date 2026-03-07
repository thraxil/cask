package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalPostFormHandler(t *testing.T) {
	// Use anonymous struct for site to satisfy interface while only providing necessary fields
	s := &site{
		Cluster: &cluster{secret: "test_secret"}, // Real cluster instance
	}

	// Test cases
	tests := []struct {
		name           string
		secretHeader   string
		method         string
		expectStatus   int
		expectBody     string
	}{
		{
			name:         "Missing secret",
			secretHeader: "",
			method:       "POST",
			expectStatus: http.StatusForbidden,
			expectBody:   "sorry, need the secret knock\n",
		},
		{
			name:         "Incorrect secret",
			secretHeader: "wrong_secret",
			method:       "POST",
			expectStatus: http.StatusForbidden,
			expectBody:   "sorry, need the secret knock\n",
		},
		{
			name:         "Correct secret",
			secretHeader: "test_secret",
			method:       "POST",
			expectStatus: http.StatusOK,
			expectBody:   "show form/handle post\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/local/form", nil)
			req.Header.Set("X-Cask-Cluster-Secret", tt.secretHeader)

			rr := httptest.NewRecorder()
			localPostFormHandler(rr, req, s)

			if status := rr.Code; status != tt.expectStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.expectStatus)
			}

			if rr.Body.String() != tt.expectBody {
				t.Errorf("handler returned unexpected body: got %q want %q",
					rr.Body.String(), tt.expectBody)
			}
		})
	}
}

func TestFileUploadSizeLimit(t *testing.T) {
	s := &site{
		MaxUploadSize: 1024,
		Node:          &node{Writeable: true},
		Cluster:       &cluster{secret: "test_secret"},
	}

	// Create a large file

	largeContent := make([]byte, 2048)
	for i := range largeContent {
		largeContent[i] = 'a'
	}

	// Create a multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "largefile.txt")
	_, _ = part.Write(largeContent)
	writer.Close()

	req := httptest.NewRequest("POST", "/local/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Cask-Cluster-Secret", "test_secret")
	req.ContentLength = int64(body.Len())

	rr := httptest.NewRecorder()
	handleLocalPost(rr, req, s)

	if status := rr.Code; status != http.StatusRequestEntityTooLarge {
		t.Errorf("handleLocalPost returned wrong status code: got %v want %v",
			status, http.StatusRequestEntityTooLarge)
	}

	// Reset for the next request
	body.Reset()
	writer = multipart.NewWriter(body)
	part, _ = writer.CreateFormFile("file", "largefile.txt")
	_, _ = part.Write(largeContent)
	writer.Close()

	req = httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.ContentLength = int64(body.Len())

	rr = httptest.NewRecorder()
	postFileHandler(rr, req, s)

	if status := rr.Code; status != http.StatusRequestEntityTooLarge {
		t.Errorf("postFileHandler returned wrong status code: got %v want %v",
			status, http.StatusRequestEntityTooLarge)
	}
}

func Test_localHandler(t *testing.T) {
	mb := &MockBackendFull{
		data: map[string][]byte{
			"sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709": []byte("content"),
		},
	}
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "test_secret", 60)
	s := &site{
		Cluster: c,
		Node:    n,
		Backend: mb,
	}
	s.verifier = &MockVerifier{}
	s.rebalancer = newRebalancer(c, *s)

	tests := []struct {
		name         string
		secretHeader string
		key          string
		ifNoneMatch  string
		expectStatus int
	}{
		{
			name:         "Unauthorized",
			secretHeader: "wrong",
			key:          "sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709",
			expectStatus: http.StatusForbidden,
		},
		{
			name:         "Invalid Key",
			secretHeader: "test_secret",
			key:          "invalid",
			expectStatus: http.StatusBadRequest,
		},
		{
			name:         "Not Found",
			secretHeader: "test_secret",
			key:          "sha1:0000000000000000000000000000000000000000",
			expectStatus: http.StatusNotFound,
		},
		{
			name:         "Success",
			secretHeader: "test_secret",
			key:          "sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709",
			expectStatus: http.StatusOK,
		},
		{
			name:         "Not Modified",
			secretHeader: "test_secret",
			key:          "sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709",
			ifNoneMatch:  "\"sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709\"",
			expectStatus: http.StatusNotModified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/local/"+tt.key+"/", nil)
			req.Header.Set("X-Cask-Cluster-Secret", tt.secretHeader)
			if tt.ifNoneMatch != "" {
				req.Header.Set("If-None-Match", tt.ifNoneMatch)
			}
			req.SetPathValue("key", tt.key)

			rr := httptest.NewRecorder()
			localHandler(rr, req, s)

			if rr.Code != tt.expectStatus {
				t.Errorf("got status %d, want %d", rr.Code, tt.expectStatus)
			}
		})
	}
}

func Test_handleLocalPost(t *testing.T) {
	mb := &MockBackendFull{}
	n := &node{Writeable: true, UUID: "test"}
	c := newCluster(n, "test_secret", 60)
	s := &site{
		Cluster:       c,
		Backend:       mb,
		Node:          n,
		MaxUploadSize: 1024,
	}

	// Success case
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	_, _ = part.Write([]byte("content"))
	writer.Close()

	req := httptest.NewRequest("POST", "/local/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Cask-Cluster-Secret", "test_secret")

	rr := httptest.NewRecorder()
	handleLocalPost(rr, req, s)

	if rr.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
	}

	// Read-only node
	n.Writeable = false
	body = &bytes.Buffer{}
	writer = multipart.NewWriter(body)
	part, _ = writer.CreateFormFile("file", "test.txt")
	_, _ = part.Write([]byte("content"))
	writer.Close()
	req = httptest.NewRequest("POST", "/local/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Cask-Cluster-Secret", "test_secret")
	rr = httptest.NewRecorder()
	handleLocalPost(rr, req, s)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

func Test_fileHandler(t *testing.T) {
	mb := &MockBackendFull{
		data: map[string][]byte{
			"sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709": []byte("content"),
		},
	}
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "test_secret", 60)
	s := &site{
		Cluster: c,
		Node:    n,
		Backend: mb,
	}
	s.verifier = &MockVerifier{}
	s.rebalancer = newRebalancer(c, *s)

	tests := []struct {
		name         string
		key          string
		ifNoneMatch  string
		expectStatus int
	}{
		{
			name:         "Invalid Key",
			key:          "invalid",
			expectStatus: http.StatusBadRequest,
		},
		{
			name:         "Found Local",
			key:          "sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709",
			expectStatus: http.StatusOK,
		},
		{
			name:         "Not Modified",
			key:          "sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709",
			ifNoneMatch:  "\"sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709\"",
			expectStatus: http.StatusNotModified,
		},
		// "Not Found Cluster" case needs more mocking of Cluster.Retrieve
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/file/"+tt.key+"/", nil)
			if tt.ifNoneMatch != "" {
				req.Header.Set("If-None-Match", tt.ifNoneMatch)
			}
			req.SetPathValue("key", tt.key)

			rr := httptest.NewRecorder()
			fileHandler(rr, req, s)

			if rr.Code != tt.expectStatus {
				t.Errorf("got status %d, want %d", rr.Code, tt.expectStatus)
			}
		})
	}
}

func Test_clusterInfoHandler(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "secret", 60)
	mb := &MockBackendFull{}
	s := &site{
		Node:    n,
		Cluster: c,
		Backend: mb,
	}

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	clusterInfoHandler(rr, req, s)

	if rr.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
	}
}

func Test_joinFormHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/join/", nil)
	rr := httptest.NewRecorder()
	joinFormHandler(rr, req, nil)

	if rr.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
	}
}

func Test_joinHandler(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "secret", 60)
	s := &site{Cluster: c}

	// Missing URL
	req := httptest.NewRequest("POST", "/join/", nil)
	rr := httptest.NewRecorder()
	joinHandler(rr, req, s)
	if rr.Body.String() != "no url specified" {
		t.Errorf("got body %q, want 'no url specified'", rr.Body.String())
	}

	// Incorrect Secret
	req = httptest.NewRequest("POST", "/join/?url=localhost:1234&secret=wrong", nil)
	rr = httptest.NewRecorder()
	joinHandler(rr, req, s)
	if rr.Code != http.StatusForbidden {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusForbidden)
	}
	
	// Correct Secret but join fails (because we haven't started memberlist properly in test)
	// We already tested join logic in cluster_test.go, so we can stop here or mock mlist.
}

func Test_configHandler(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	s := &site{Node: n}

	req := httptest.NewRequest("GET", "/config/", nil)
	rr := httptest.NewRecorder()
	configHandler(rr, req, s)

	if rr.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("got content type %q, want application/json", rr.Header().Get("Content-Type"))
	}
}

func Test_postFileHandler_Success(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "secret", 60)
	mb := &MockBackendFull{}
	s := &site{
		Node:          n,
		Cluster:       c,
		Backend:       mb,
		MaxUploadSize: 1024,
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_, _ = writer.CreateFormFile("file", "test.txt")
	// empty file
	writer.Close()

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	
	postFileHandler(rr, req, s)

	if rr.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
	}
	
	var pr postResponse
	err := json.Unmarshal(rr.Body.Bytes(), &pr)
	if err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	// sha1("") = da39a3ee5e6b4b0d3255bfef95601890afd80709
	expectedKey := "sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709"
	if pr.Key != expectedKey {
		t.Errorf("got key %q, want %q", pr.Key, expectedKey)
	}
}

func Test_faviconHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/favicon.ico", nil)
	rr := httptest.NewRecorder()
	faviconHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
	}
}