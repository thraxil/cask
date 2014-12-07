package main

type Site struct {
	Node *Node
}

func NewSite(n *Node) *Site {
	return &Site{
		Node: n,
	}
}
