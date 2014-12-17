package main

type Site struct {
	Node           *Node
	Cluster        *Cluster
	Backend        Backend
	Replication    int
	MaxReplication int
	ClusterSecret  string
	AAEInterval    int
	rebalancer     *Rebalancer
}

func NewSite(n *Node, c *Cluster, b Backend, replication, max_replication int, cluster_secret string, aae_interval int) *Site {
	// couple sanity checks
	if replication < 1 {
		replication = 1
	}
	if max_replication < replication {
		max_replication = replication
	}
	if aae_interval < 1 {
		// unset. default to 5 seconds
		aae_interval = 5
	}
	s := &Site{
		Node:           n,
		Cluster:        c,
		Backend:        b,
		Replication:    replication,
		MaxReplication: max_replication,
		ClusterSecret:  cluster_secret,
		AAEInterval:    aae_interval,
	}
	s.rebalancer = NewRebalancer(c, *s)
	return s
}

func (s Site) ActiveAntiEntropy() {
	// it's the backend's responsibility
	s.Backend.ActiveAntiEntropy(s.Cluster, s, s.AAEInterval)
}

func (s Site) Rebalance(key Key) error {
	return s.rebalancer.Rebalance(key)
}
