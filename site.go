package main

type Site struct {
	Node           *Node
	Cluster        *Cluster
	Backend        Backend
	Replication    int
	MaxReplication int
}

func NewSite(n *Node, c *Cluster, b Backend, replication, max_replication int) *Site {
	// couple sanity checks
	if replication < 1 {
		replication = 1
	}
	if max_replication < replication {
		max_replication = replication
	}
	return &Site{
		Node:           n,
		Cluster:        c,
		Backend:        b,
		Replication:    replication,
		MaxReplication: max_replication,
	}
}

func (s Site) ActiveAntiEntropy() {
	// it's the backend's responsibility
	s.Backend.ActiveAntiEntropy(s.Cluster, s)
}
