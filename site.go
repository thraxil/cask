package main

type Site struct {
	Node    *Node
	Cluster *Cluster
	Backend Backend
}

func NewSite(n *Node, c *Cluster, b Backend) *Site {
	return &Site{
		Node:    n,
		Cluster: c,
		Backend: b,
	}
}

func (s Site) ActiveAntiEntropy() {
	// it's the backend's responsibility
	s.Backend.ActiveAntiEntropy(s.Cluster)
}
