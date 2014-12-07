package main

type Node struct {
	UUID      string `json:uuid`
	BaseUrl   string `json:base_url`
	Writeable bool   `json:writeable`
}

func NewNode(uuid, base_url string, writeable bool) *Node {
	return &Node{
		UUID:      uuid,
		BaseUrl:   base_url,
		Writeable: writeable,
	}
}
