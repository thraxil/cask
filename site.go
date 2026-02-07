package main

type site struct {
	Node           *node
	Cluster        *cluster
	Backend        backend
	Replication    int
	MaxReplication int
	ClusterSecret  string
	AAEInterval    int
	MaxUploadSize  int64
	verifier       verifier
	rebalancer     *rebalancer
}

func newSite(n *node, c *cluster, b backend, replication, maxReplication int, clusterSecret string, aaeInterval int, maxUploadSize int64) *site {
	// couple sanity checks
	if replication < 1 {
		replication = 1
	}
	if maxReplication < replication {
		maxReplication = replication
	}
	if aaeInterval < 1 {
		// unset. default to 5 seconds
		aaeInterval = 5
	}
	s := &site{
		Node:           n,
		Cluster:        c,
		Backend:        b,
		Replication:    replication,
		MaxReplication: maxReplication,
		ClusterSecret:  clusterSecret,
		AAEInterval:    aaeInterval,
		MaxUploadSize:  maxUploadSize,
	}
	s.verifier = b.NewVerifier(c)
	s.rebalancer = newRebalancer(c, *s)
	return s
}

func (s site) ActiveAntiEntropy() {
	// it's the backend's responsibility
	s.Backend.ActiveAntiEntropy(s.Cluster, s, s.AAEInterval)
}

func (s site) Rebalance(key key) error {
	return s.rebalancer.Rebalance(key)
}

func (s site) Verify(path string, key key, h string) error {
	return s.verifier.Verify(path, key, h)
}

func (s site) VerifyKey(key key) error {
	return s.verifier.VerifyKey(key)
}
