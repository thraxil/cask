package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/memberlist"
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
	_, ok = c.FindNeighborByUUID("testuuid2")
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

func TestRing(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "clustersecret", 60)

	// add a writeable neighbor
	n2 := newNode("testuuid2", "http://localhost:1001", true)
	c.AddNeighbor(*n2)

	// add a non-writeable neighbor - should still be in Ring (unlike WriteRing)
	n3 := newNode("testuuid3", "http://localhost:1002", false)
	c.AddNeighbor(*n3)

	r := c.Ring()

	// 3 nodes (myself + n2 + n3) * replicas
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

func Test_FailedNeighbor(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "clustersecret", 60)

	// add a neighbor
	n2 := newNode("testuuid2", "http://localhost:1001", true)
	c.AddNeighbor(*n2)

	// fail the neighbor
	c.FailedNeighbor(*n2)

	// we should be able to retrieve them
	n3, ok := c.FindNeighborByUUID("testuuid2")
	if !ok {
		t.Error("failed to find neighbor")
	}
	if n3.Writeable {
		t.Error("neighbor should not be writeable")
	}
	// LastFailed should be recent (within last second)
	if time.Since(n3.LastFailed) > time.Second {
		t.Error("LastFailed time was not updated recently")
	}
}

func Test_UpdateNeighbor(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "clustersecret", 60)

	// add a neighbor
	n2 := newNode("testuuid2", "http://localhost:1001", true)
	c.AddNeighbor(*n2)

	// update the neighbor
	n2.BaseURL = "http://localhost:1002"
	n2.Writeable = false
	n2.LastSeen = time.Now()
	c.UpdateNeighbor(*n2)

	// we should be able to retrieve them
	n3, ok := c.FindNeighborByUUID("testuuid2")
	if !ok {
		t.Error("failed to find neighbor")
	}
	if n3.BaseURL != "http://localhost:1002" {
		t.Error("BaseURL was not updated")
	}
	if n3.Writeable {
		t.Error("Writeable was not updated")
	}
	if !n3.LastSeen.Equal(n2.LastSeen) {
		t.Error("LastSeen was not updated")
	}
}

func Test_Cluster_Retrieve(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "clustersecret", 60)

	// we should not be able to retrieve anything yet
	k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
	_, err := c.Retrieve(*k)
	if err == nil {
		t.Error("Retrieve should have failed")
	}

	// add a neighbor (mocked via httptest in a real scenario, but here we can just verify the logic)
	// Since Retrieve calls n.Retrieve which makes network calls, we need to mock the node or the network.
	// However, given the current structure, we can't easily inject a mock HTTP client into the node.
	// But we can test that it iterates through neighbors.
	// For this test, we'll just verify the "not found" case which is what we can test without mocking network.
}

func Test_Cluster_AddFile(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	_ = newCluster(n, "clustersecret", 60)
	
	// Create a dummy file
	f := multipart.NewReader(nil, "")
	_ = f // avoid unused variable error if we don't use it yet
	
	// Since AddFile also makes network calls, we can't fully test it without mocking.
	// But we can test the behavior when there are no neighbors.
	
	// k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
	// if c.AddFile(*k, f, 1, 1) {
	// 	t.Error("AddFile should have failed (no writeable neighbors)")
	// }
	// Wait, AddFile takes multipart.File which is an interface. We can mock that if needed.
	// But the real issue is n.AddFile making network calls.
	
	// Given the constraints and the request to add coverage, we should focus on what we can test.
	// We can test GetBroadcasts, LocalState, etc.
}

func Test_GetBroadcasts(t *testing.T) {
	// Need to initialize broadcasts queue which is a global
	_ = startMemberList(newCluster(newNode("testuuid", "http://localhost:1000", true), "clustersecret", 60), config{GossipPort: 12345})
	// defer cleanup?
	
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "clustersecret", 60)
	
	b := c.GetBroadcasts(10, 10)
	if len(b) != 0 {
		t.Errorf("Expected 0 broadcasts, got %d", len(b))
	}
}

func Test_LocalState(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "clustersecret", 60)
	
	s := c.LocalState(true)
	if len(s) == 0 {
		t.Error("LocalState returned empty")
	}
	
	var hb heartbeat
	if err := json.Unmarshal(s, &hb); err != nil {
		t.Error("LocalState returned invalid json")
	}
	if hb.UUID != "testuuid" {
		t.Error("LocalState returned wrong UUID")
	}
}

func Test_MergeRemoteState(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "clustersecret", 60)
	
	// Valid state
	hb := heartbeat{
		UUID:      "testuuid2",
		BaseURL:   "http://localhost:1001",
		Writeable: true,
		Secret:    "clustersecret",
	}
	b, _ := json.Marshal(hb)
	c.MergeRemoteState(b, true)
	
	// Should be added as neighbor (actually UpdateNeighbor logic)
	// Wait, MergeRemoteState calls UpdateNeighbor, not AddNeighbor?
	// Looking at code: Yes, UpdateNeighbor.
	// But UpdateNeighbor only updates if it exists in neighbors map?
	// Let's check UpdateNeighbor:
	// func (c *cluster) UpdateNeighbor(neighbor node) {
	// 	c.chF <- func() {
	// 		if n, ok := c.neighbors[neighbor.UUID]; ok {
	// ...
	// So MergeRemoteState only updates EXISTING neighbors. It doesn't add new ones?
	// That seems odd for a merge function, but let's test that behavior.
	
	// Add it first
	n2 := newNode("testuuid2", "http://localhost:1001", true)
	c.AddNeighbor(*n2)
	
	// Now update via MergeRemoteState
	hb.BaseURL = "http://localhost:1002"
	b, _ = json.Marshal(hb)
	c.MergeRemoteState(b, true)
	
	// Allow for goroutine execution
	time.Sleep(10 * time.Millisecond)
	
	n3, ok := c.FindNeighborByUUID("testuuid2")
	if !ok {
		t.Error("neighbor lost")
	}
	if n3.BaseURL != "http://localhost:1002" {
		t.Errorf("MergeRemoteState didn't update neighbor. Expected http://localhost:1002, got %s", n3.BaseURL)
	}
	
	// Invalid JSON
	c.MergeRemoteState([]byte("invalid"), true)
	// Should not panic
}

func Test_NotifyEvents(t *testing.T) {
	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "clustersecret", 60)
	
	hb := heartbeat{
		UUID:      "testuuid2",
		BaseURL:   "http://localhost:1001",
		Writeable: true,
		Secret:    "clustersecret",
	}
	b, _ := json.Marshal(hb)
	
	// Join
	mn := &memberlist.Node{
		Meta: b,
	}
	c.NotifyJoin(mn)
	time.Sleep(10 * time.Millisecond)
	
	_, ok := c.FindNeighborByUUID("testuuid2")
	if !ok {
		t.Error("NotifyJoin failed to add neighbor")
	}
	
	// Update
	hb.BaseURL = "http://localhost:1002"
	b, _ = json.Marshal(hb)
	mn.Meta = b
	c.NotifyUpdate(mn)
	time.Sleep(10 * time.Millisecond)
	
	n3, ok := c.FindNeighborByUUID("testuuid2")
	if !ok {
		t.Error("neighbor lost after update")
	}
	if n3.BaseURL != "http://localhost:1002" {
		t.Errorf("NotifyUpdate failed. Expected http://localhost:1002, got %s", n3.BaseURL)
	}
	
	// Leave
	c.NotifyLeave(mn)
	time.Sleep(10 * time.Millisecond)
	
	_, ok = c.FindNeighborByUUID("testuuid2")
	if ok {
		t.Error("NotifyLeave failed to remove neighbor")
	}
	
	// NotifyMsg (should do nothing, but cover it)
	c.NotifyMsg([]byte("msg"))
}

func Test_Cluster_Retrieve_With_Neighbor(t *testing.T) {
	// Setup a mock neighbor
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/local/sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709/" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("content"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "clustersecret", 60)
	
	// Use the mock server URL for the neighbor
	n2 := newNode("neighbor", ts.URL, true)
	c.AddNeighbor(*n2)
	
	// Wait for neighbor to be added to ensure it's in the list
	time.Sleep(50 * time.Millisecond)
	
	// We need to ensure the neighbor is selected by ReadOrder.
	// ReadOrder depends on hashing.
	// Since we have ourselves and one neighbor.
	// Retrieve checks ReadOrder.
	
	k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
	
	// Try to retrieve. It should query the neighbor.
	b, err := c.Retrieve(*k)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}
	if string(b) != "content" {
		t.Errorf("Expected 'content', got '%s'", string(b))
	}
}

func Test_Cluster_AddFile_With_Neighbor(t *testing.T) {
	// Setup a mock neighbor that accepts the file
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/local/" {
			// verify content if needed, but for now just return success
			w.WriteHeader(http.StatusOK)
			// Return the key as expected by AddFile
			_, _ = w.Write([]byte("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709"))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	n := newNode("testuuid", "http://localhost:1000", true)
	c := newCluster(n, "clustersecret", 60)
	
	n2 := newNode("neighbor", ts.URL, true)
	c.AddNeighbor(*n2)
	
	time.Sleep(50 * time.Millisecond)

	k, _ := keyFromString("sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709")
	
	// Create a dummy multipart file in memory
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	_, _ = part.Write([]byte("")) // Empty file gives the hash
	writer.Close()
	
	// We need a multipart.File to pass to AddFile.
	// We can get one by parsing a request.
	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	err := req.ParseMultipartForm(1024)
	if err != nil {
		t.Fatal(err)
	}
	
	file, _, err := req.FormFile("file")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	
	// We need to ensure WriteOrder selects the neighbor.
	// With replication=1, minReplication=1, it should try until it succeeds.
	
	if !c.AddFile(*k, file, 1, 1) {
		t.Error("AddFile failed")
	}
}
