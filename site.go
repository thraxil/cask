package main

type Site struct {
	Node    *Node
	Backend Backend
}

func NewSite(n *Node, b Backend) *Site {
	return &Site{
		Node:    n,
		Backend: b,
	}
}
