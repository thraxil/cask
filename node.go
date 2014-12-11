package main

import (
	"crypto/sha1"
	"fmt"
	"io"
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
