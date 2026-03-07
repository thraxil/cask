package main

import (
	"errors"
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
			_, _ = w.Write([]byte("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709"))
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
			_, _ = w.Write([]byte("sha1:otherkey"))
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
	
	func Test_Retrieve(t *testing.T) {
		// Success
		{
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("Expected GET, got %s", r.Method)
				}
				if r.URL.Path != "/local/sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709/" {
					t.Errorf("Expected path /local/sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709/, got %s", r.URL.Path)
				}
				// Check secret
				if r.Header.Get("X-Cask-Cluster-Secret") != "secret" {
					t.Errorf("Expected secret header, got %s", r.Header.Get("X-Cask-Cluster-Secret"))
				}
	
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("test content"))
			}))
			defer server.Close()
	
			n := newNode("testuuid", server.URL, true)
			k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
			b, err := n.Retrieve(*k, "secret")
			if err != nil {
				t.Fatalf("Retrieve failed: %v", err)
			}
			if string(b) != "test content" {
				t.Errorf("Expected 'test content', got '%s'", string(b))
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
			_, err := n.Retrieve(*k, "secret")
			if err == nil {
				t.Error("Retrieve should have failed on server error")
			}
		}
	
		// 404
		{
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()
	
			n := newNode("testuuid", server.URL, true)
			k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
			_, err := n.Retrieve(*k, "secret")
			if err == nil {
				t.Error("Retrieve should have failed on 404")
			}
					if err.Error() != "404, probably" {
						t.Errorf("Expected '404, probably', got '%v'", err)
					}
				}
			}
			
			func Test_RetrieveInfo(t *testing.T) {
				// Success
				{
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Method != "HEAD" {
							t.Errorf("Expected HEAD, got %s", r.Method)
						}
						w.WriteHeader(http.StatusOK)
					}))
					defer server.Close()
			
					n := newNode("testuuid", server.URL, true)
					k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
					ok, err := n.RetrieveInfo(*k, "secret")
					if err != nil {
						t.Fatalf("RetrieveInfo failed: %v", err)
					}
					if !ok {
						t.Error("RetrieveInfo returned false on success")
					}
				}
			
				// 404
				{
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusNotFound)
					}))
					defer server.Close()
			
					n := newNode("testuuid", server.URL, true)
					k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
					ok, err := n.RetrieveInfo(*k, "secret")
					if ok {
						t.Error("RetrieveInfo returned true on 404")
					}
					if err == nil || err.Error() != "404, probably" {
						t.Errorf("Expected '404, probably', got '%v'", err)
					}
				}
			
				// Timeout (using timedHeadRequest directly to test with shorter timeout)
				{
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						time.Sleep(200 * time.Millisecond)
						w.WriteHeader(http.StatusOK)
					}))
					defer server.Close()
			
					n := newNode("testuuid", server.URL, true)
					k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
					
					// We can't easily change the timeout in RetrieveInfo without changing code, 
					// but we can test timedHeadRequest directly.
					_, err := timedHeadRequest(n.retrieveInfoURL(*k), 10*time.Millisecond, "secret")
					if err == nil {
						t.Error("timedHeadRequest should have timed out")
							} else if err.Error() != "HEAD request timed out" {
								t.Errorf("Expected 'HEAD request timed out', got '%v'", err)
							}
						}
					}
					
					func Test_CheckFile(t *testing.T) {
						// Exists and Valid
						{
							server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
								w.WriteHeader(http.StatusOK)
								_, _ = w.Write([]byte("test content"))
							}))
							defer server.Close()
					
							n := newNode("testuuid", server.URL, true)
							// sha1("test content") = 1eebdf4fdc9fc7bf283031b93f9aef3338de9052
							k, _ := keyFromString("sha1:1eebdf4fdc9fc7bf283031b93f9aef3338de9052")
							
							found, content, err := n.CheckFile(*k, "secret")
							if !found {
								t.Error("CheckFile returned false when file exists")
							}
							if err != nil {
								t.Errorf("CheckFile returned error: %v", err)
							}
							if string(content) != "test content" {
								t.Errorf("Expected 'test content', got '%s'", string(content))
							}
						}
					
						// Exists but Corrupt
						{
							server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
								w.WriteHeader(http.StatusOK)
								_, _ = w.Write([]byte("corrupt content"))
							}))
							defer server.Close()
					
							n := newNode("testuuid", server.URL, true)
							k, _ := keyFromString("sha1:1eebdf4fdc9fc7bf283031b93f9aef3338de9052")
							
							found, _, err := n.CheckFile(*k, "secret")
							if !found {
								t.Error("CheckFile returned false when file exists (even if corrupt)")
							}
							if err == nil || err.Error() != "corrupt" {
								t.Errorf("Expected 'corrupt' error, got '%v'", err)
							}
						}
					
						// Not Found
						{
							server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
								w.WriteHeader(http.StatusNotFound)
							}))
							defer server.Close()
					
							n := newNode("testuuid", server.URL, true)
							k, _ := keyFromString("sha1:1eebdf4fdc9fc7bf283031b93f9aef3338de9052")
							
							found, _, err := n.CheckFile(*k, "secret")
							if found {
								t.Error("CheckFile returned true when file missing")
							}
							if err != nil {
								t.Errorf("CheckFile returned error when file missing: %v", err)
							}
						}
					}
					
					func Test_doublecheckReplica(t *testing.T) {
						k, _ := keyFromString("sha1:1eebdf4fdc9fc7bf283031b93f9aef3338de9052")
						if !doublecheckReplica([]byte("test content"), *k) {
							t.Error("doublecheckReplica failed for valid content")
						}
						if doublecheckReplica([]byte("wrong content"), *k) {
							t.Error("doublecheckReplica succeeded for invalid content")
						}
					}
					
					func Test_processRetrieveInfoResponse_Nil(t *testing.T) {
						n := newNode("testuuid", "http://localhost:1000", true)
						_, err := n.processRetrieveInfoResponse(nil)
							if err == nil {
								t.Error("processRetrieveInfoResponse should fail with nil response")
							}
						}
						
						type MockBackend struct {
							freeSpace uint64
						}
						
						func (m MockBackend) String() string                        { return "mock" }
						func (m MockBackend) Write(key key, r io.ReadCloser) error  { return nil }
						func (m MockBackend) Read(key key) ([]byte, error)          { return nil, nil }
						func (m MockBackend) Exists(key key) bool                   { return false }
						func (m MockBackend) Delete(key key) error                  { return nil }
						func (m MockBackend) ActiveAntiEntropy(c *cluster, s site, i int) {}
						func (m MockBackend) NewVerifier(c *cluster) verifier       { return nil }
						func (m MockBackend) FreeSpace() uint64                     { return m.freeSpace }
						
						func Test_updateFreeSpaceStatus(t *testing.T) {
							n := newNode("testuuid", "http://localhost:1000", true)
							
							// Case 1: Writeable, FreeSpace > Min => Stay Writeable
							mb := MockBackend{freeSpace: 2000}
							n.updateFreeSpaceStatus(1000, mb)
							if !n.Writeable {
								t.Error("Should be writeable")
							}
						
							// Case 2: Writeable, FreeSpace < Min => Become Unwriteable
							mb.freeSpace = 500
							n.updateFreeSpaceStatus(1000, mb)
							if n.Writeable {
								t.Error("Should be unwriteable")
							}
						
							// Case 3: Unwriteable, FreeSpace < Min => Stay Unwriteable
							n.Writeable = false
							mb.freeSpace = 500
							n.updateFreeSpaceStatus(1000, mb)
							if n.Writeable {
								t.Error("Should be unwriteable")
							}
						
								// Case 4: Unwriteable, FreeSpace > Min => Become Writeable
								mb.freeSpace = 2000
								n.updateFreeSpaceStatus(1000, mb)
								if !n.Writeable {
									t.Error("Should be writeable")
								}
							}
							
							func Test_Retrieve_Errors(t *testing.T) {
								// NewRequest Error
								{
									// Control character in URL to trigger NewRequest error
									n := newNode("testuuid", "http://loc\nalhost:1000", true)
									k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
									_, err := n.Retrieve(*k, "secret")
									if err == nil {
										t.Error("Retrieve should have failed on invalid URL")
									}
								}
							
								// Do Error
								{
									n := newNode("testuuid", "http://invalid-host-does-not-exist:12345", true)
									k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
									_, err := n.Retrieve(*k, "secret")
									if err == nil {
										t.Error("Retrieve should have failed on unreachable host")
									}
								}
							}
							
							func Test_timedHeadRequest_Errors(t *testing.T) {
								// NewRequest Error
								{
									_, err := timedHeadRequest("http://loc\nalhost:1000", 1*time.Second, "secret")
									if err == nil {
										t.Error("timedHeadRequest should have failed on invalid URL")
									}
								}
							
								// Do Error
								{
									_, err := timedHeadRequest("http://invalid-host-does-not-exist:12345", 1*time.Second, "secret")
									if err == nil {
										t.Error("timedHeadRequest should have failed on unreachable host")
									}
								}
							}
							
							func Test_postFile_Errors(t *testing.T) {
									// Do Error
									{
										r := strings.NewReader("test content")
										_, err := postFile(r, "http://invalid-host-does-not-exist:12345", "secret")
										if err == nil {
											t.Error("postFile should have failed on unreachable host")
										}
									}
								}
								
								type FailReader struct{}
								
								func (f FailReader) Read(p []byte) (n int, err error) { return 0, errors.New("read failed") }
								
								func Test_postFile_ReadError(t *testing.T) {
									_, err := postFile(FailReader{}, "http://localhost:1000", "secret")
									if err == nil {
										t.Error("postFile should fail on read error")
									}
								}