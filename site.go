package main

type Site struct {
	Node           *Node
	Cluster        *Cluster
	Backend        Backend
	Replication    int
	MaxReplication int
}

func NewSite(n *Node, c *Cluster, b Backend) *Site {
	return &Site{
		Node:           n,
		Cluster:        c,
		Backend:        b,
		Replication:    3,
		MaxReplication: 3,
	}
}

func (s Site) ActiveAntiEntropy() {
	// it's the backend's responsibility
	s.Backend.ActiveAntiEntropy(s.Cluster, s)
}
