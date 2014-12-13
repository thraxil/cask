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

func (n *Node) AddFile(key Key, f multipart.File) bool {
	resp, err := postFile(f, n.AddFileUrl())
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

func postFile(f io.Reader, target_url string) (*http.Response, error) {
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
	return http.Post(target_url, content_type, body_buf)
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

func (n *Node) Retrieve(key Key) ([]byte, error) {
	resp, err := http.Get(n.retrieveUrl(key))
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
		panic(err)
	}
	defer resp.Body.Close()
	ioutil.ReadAll(resp.Body)
}
