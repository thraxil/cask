package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"time"
)

type node struct {
	UUID       string    `json:"uuid"`
	BaseURL    string    `json:"base_url"`
	Writeable  bool      `json:"writeable"`
	LastSeen   time.Time `json:"last_seen"`
	LastFailed time.Time `json:"last_failed"`
}

func newNode(uuid, baseURL string, writeable bool) *node {
	return &node{
		UUID:      uuid,
		BaseURL:   baseURL,
		Writeable: writeable,
	}
}

func (n node) LastSeenFormatted() string {
	return n.LastSeen.Format("2006-01-02 15:04:05")
}

func (n node) LastFailedFormatted() string {
	return n.LastFailed.Format("2006-01-02 15:04:05")
}

func (n node) AddFileURL() string {
	return n.BaseURL + "/local/"
}

func (n *node) AddFile(key key, f io.Reader, secret string) bool {
	resp, err := postFile(f, n.AddFileURL(), secret)
	if err != nil {
		log.Println("postFile returned false")
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Println("didn't get a 200")
		return false
	}

	b, _ := ioutil.ReadAll(resp.Body)
	// make sure it saved it as the same key
	return string(b) == key.String()
}

func (n node) Unhealthy() bool {
	return n.LastFailed.After(n.LastSeen)
}

func postFile(f io.Reader, targetURL, secret string) (*http.Response, error) {
	bodyBuf := bytes.NewBufferString("")
	bodyWriter := multipart.NewWriter(bodyBuf)
	fileWriter, err := bodyWriter.CreateFormFile("file", "file.dat")
	if err != nil {
		panic(err.Error())
	}
	io.Copy(fileWriter, f)
	// .Close() finishes setting it up
	// do not defer this or it will make and empty POST request
	bodyWriter.Close()
	contentType := bodyWriter.FormDataContentType()
	c := http.Client{}
	req, err := http.NewRequest("POST", targetURL, bodyBuf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Cask-Cluster-Secret", secret)

	return c.Do(req)
}

func (n node) HashKeys() []string {
	keys := make([]string, replicas)
	h := sha1.New()
	for i := range keys {
		h.Reset()
		io.WriteString(h, fmt.Sprintf("%s%d", n.UUID, i))
		keys[i] = fmt.Sprintf("%x", h.Sum(nil))
	}
	return keys
}

func (n node) retrieveURL(key key) string {
	return n.BaseURL + "/local/" + key.String() + "/"
}

func (n *node) Retrieve(key key, secret string) ([]byte, error) {
	c := http.Client{}
	req, err := http.NewRequest("GET", n.retrieveURL(key), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Cask-Cluster-Secret", secret)
	resp, err := c.Do(req)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.Status != "200 OK" {
		return nil, errors.New("404, probably")
	}
	b, _ := ioutil.ReadAll(resp.Body)
	return b, nil
}

func (n node) retrieveInfoURL(key key) string {
	return n.BaseURL + "/local/" + key.String() + "/"
}

type pingResponse struct {
	Resp *http.Response
	Err  error
}

func timedHeadRequest(url string, duration time.Duration, secret string) (resp *http.Response, err error) {
	rc := make(chan pingResponse, 1)
	go func() {
		c := http.Client{}
		req, err := http.NewRequest("HEAD", url, nil)
		if err != nil {
			rc <- pingResponse{nil, err}
			return
		}
		req.Header.Set("X-Cask-Cluster-Secret", secret)
		resp, err := c.Do(req)
		rc <- pingResponse{resp, err}
	}()
	select {
	case pr := <-rc:
		resp = pr.Resp
		err = pr.Err
	case <-time.After(duration):
		err = errors.New("HEAD request timed out")
	}
	return
}

func (n *node) RetrieveInfo(key key, secret string) (bool, error) {
	url := n.retrieveInfoURL(key)
	resp, err := timedHeadRequest(url, 1*time.Second, secret)
	if err != nil {
		// TODO: n.LastFailed = time.Now()
		return false, err
	}

	// otherwise, we got the info
	// TODO: n.LastSeen = time.Now()
	return n.processRetrieveInfoResponse(resp)
}

func (n *node) processRetrieveInfoResponse(resp *http.Response) (bool, error) {
	if resp == nil {
		return false, errors.New("nil response")
	}
	defer resp.Body.Close()
	if resp.Status != "200 OK" {
		return false, errors.New("404, probably")
	}
	ioutil.ReadAll(resp.Body)
	return true, nil
}

type nodeHeartbeat struct {
	UUID      string `json:"uuid"`
	BaseURL   string `json:"base_url"`
	Writeable bool   `json:"writeable"`
}

func (n node) NodeHeartbeat() nodeHeartbeat {
	return nodeHeartbeat{UUID: n.UUID, BaseURL: n.BaseURL, Writeable: n.Writeable}
}

func (n node) heartbeatURL() string {
	return n.BaseURL + "/heartbeat/"
}

func (n node) SendHeartbeat(hb heartbeat) {
	j, err := json.Marshal(hb)
	if err != nil {
		log.Println(err)
		return
	}
	req, err := http.NewRequest("POST", n.heartbeatURL(), bytes.NewBuffer(j))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		// can't send a heartbeat to that node
		// TODO: mark it as failed
		// for now, just keep going...
		return
	}
	defer resp.Body.Close()
	ioutil.ReadAll(resp.Body)
}

// get file with specified key from the node
// return (found, file content, error)
func (n node) CheckFile(key key, secret string) (bool, []byte, error) {
	f, err := n.Retrieve(key, secret)
	if err != nil {
		// node doesn't have it
		return false, nil, nil
	}
	if !doublecheckReplica(f, key) {
		// that node had a bad copy as well
		return true, nil, errors.New("corrupt")
	}
	return true, f, nil
}

func doublecheckReplica(f []byte, key key) bool {
	hn := sha1.New()
	io.WriteString(hn, string(f))
	nhash := fmt.Sprintf("%x", hn.Sum(nil))
	return "sha1:"+nhash == key.String()
}
