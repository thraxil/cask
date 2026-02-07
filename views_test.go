package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Mocks for dependencies
type mockCluster struct {
	secret string
}

func (m *mockCluster) CheckSecret(s string) bool {
	return m.secret == s
}

func (m *mockCluster) AddFile(k key, r io.Reader, replication int, minReplication int) bool {
	return true
}

// mockBackend implements the Backend interface
type mockBackend struct {
	readData []byte
	exists   bool
	err      error
}

func (m *mockBackend) String() string                        { return "mock" }
func (m *mockBackend) Write(key key, r io.ReadCloser) error  { return nil }
func (m *mockBackend) Read(key key) ([]byte, error)          { return m.readData, m.err }
func (m *mockBackend) Exists(key key) bool                   { return m.exists }
func (m *mockBackend) Delete(key key) error                  { return nil }
func (m *mockBackend) NewVerifier(c *cluster) verifier       { return nil }
func (m *mockBackend) ActiveAntiEntropy(c *cluster, s site, interval int) {}
func (m *mockBackend) FreeSpace() uint64                     { return 0 }

// mockVerifier implements the verifier interface
type mockVerifier struct {
	err error
}

func (m *mockVerifier) Verify(path string, key key, h string) error { return m.err }
func (m *mockVerifier) VerifyKey(key key) error                     { return m.err }

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
	part.Write(largeContent)
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
	part.Write(largeContent)
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