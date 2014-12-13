package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
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

func (n *Node) Stash(key Key) bool {
	return true
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

func (n *Node) Retrieve(key Key) ([]byte, error) {
	b := make([]byte, 0)
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
