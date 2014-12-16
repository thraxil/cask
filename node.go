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

type Node struct {
	UUID       string    `json:uuid`
	BaseUrl    string    `json:base_url`
	Writeable  bool      `json:writeable`
	LastSeen   time.Time `json:"last_seen"`
	LastFailed time.Time `json:"last_failed"`
}

func NewNode(uuid, base_url string, writeable bool) *Node {
	return &Node{
		UUID:      uuid,
		BaseUrl:   base_url,
		Writeable: writeable,
	}
}

func (n Node) AddFileUrl() string {
	return n.BaseUrl + "/local/"
}

func (n *Node) AddFile(key Key, f io.Reader, secret string) bool {
	resp, err := postFile(f, n.AddFileUrl(), secret)
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

func postFile(f io.Reader, target_url, secret string) (*http.Response, error) {
	body_buf := bytes.NewBufferString("")
	body_writer := multipart.NewWriter(body_buf)
	file_writer, err := body_writer.CreateFormFile("file", "file.dat")
	if err != nil {
		panic(err.Error())
	}
	io.Copy(file_writer, f)
	// .Close() finishes setting it up
	// do not defer this or it will make and empty POST request
	body_writer.Close()
	content_type := body_writer.FormDataContentType()
	c := http.Client{}
	req, err := http.NewRequest("POST", target_url, body_buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", content_type)
	req.Header.Set("X-Cask-Cluster-Secret", secret)

	return c.Do(req)
}

func (n Node) HashKeys() []string {
	keys := make([]string, REPLICAS)
	h := sha1.New()
	for i := range keys {
		h.Reset()
		io.WriteString(h, fmt.Sprintf("%s%d", n.UUID, i))
		keys[i] = fmt.Sprintf("%x", h.Sum(nil))
	}
	return keys
}

func (n Node) retrieveUrl(key Key) string {
	return n.BaseUrl + "/local/" + key.String() + "/"
}

func (n *Node) Retrieve(key Key, secret string) ([]byte, error) {
	c := http.Client{}
	req, err := http.NewRequest("GET", n.retrieveUrl(key), nil)
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

func (n Node) retrieveInfoUrl(key Key) string {
	return n.BaseUrl + "/local/" + key.String() + "/"
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

func (n *Node) RetrieveInfo(key Key, secret string) (bool, error) {
	url := n.retrieveInfoUrl(key)
	resp, err := timedHeadRequest(url, 1*time.Second, secret)
	if err != nil {
		// TODO: n.LastFailed = time.Now()
		return false, err
	}

	// otherwise, we got the info
	// TODO: n.LastSeen = time.Now()
	return n.processRetrieveInfoResponse(resp)
}

func (n *Node) processRetrieveInfoResponse(resp *http.Response) (bool, error) {
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

type node_heartbeat struct {
	UUID      string `json:"uuid"`
	BaseUrl   string `json:"base_url"`
	Writeable bool   `json:"writeable"`
}

func (n Node) NodeHeartbeat() node_heartbeat {
	return node_heartbeat{UUID: n.UUID, BaseUrl: n.BaseUrl, Writeable: n.Writeable}
}

func (n Node) HeartbeatUrl() string {
	return n.BaseUrl + "/heartbeat/"
}

func (n Node) SendHeartbeat(hb heartbeat) {
	j, err := json.Marshal(hb)
	if err != nil {
		log.Println(err)
		return
	}
	req, err := http.NewRequest("POST", n.HeartbeatUrl(), bytes.NewBuffer(j))
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
func (n Node) CheckFile(key Key, secret string) (bool, []byte, error) {
	f, err := n.Retrieve(key, secret)
	if err != nil {
		// node doesn't have it
		return false, nil, nil
	}
	if !doublecheck_replica(f, key) {
		// that node had a bad copy as well
		return true, nil, errors.New("corrupt")
	}
	return true, f, nil
}

func doublecheck_replica(f []byte, key Key) bool {
	hn := sha1.New()
	io.WriteString(hn, string(f))
	nhash := fmt.Sprintf("%x", hn.Sum(nil))
	return "sha1:"+nhash == key.String()
}
