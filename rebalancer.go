package main

import (
	"bytes"
	"errors"
	"log"
)

type rebalanceRequest struct {
	Key        key
	chResponse chan error
}

type rebalancer struct {
	c   *cluster
	s   site
	chR chan rebalanceRequest
}

func newRebalancer(c *cluster, s site) *rebalancer {
	ch := make(chan rebalanceRequest)
	r := rebalancer{c, s, ch}
	go r.run()
	return &r
}

func (r *rebalancer) run() {
	for req := range r.chR {
		req.chResponse <- r.doRebalance(req.Key)
	}
}

func (r *rebalancer) Rebalance(key key) error {
	resp := make(chan error)
	req := rebalanceRequest{key, resp}
	r.chR <- req
	return <-resp
}

// check that the file is stored in at least Replication nodes
// and, if at all possible, those should be the ones at the front
// of the list
func (r rebalancer) doRebalance(key key) error {
	if r.c == nil {
		log.Println("can't rebalance on a nil cluster")
		return errors.New("nil cluster")
	}
	nodesToCheck := r.c.ReadOrder(key.String())
	satisfied, deleteLocal, foundReplicas := r.checkNodesForRebalance(key, nodesToCheck)
	if !satisfied {
		log.Printf("could not replicate %s to %d nodes", key, r.s.Replication)
	} else {
		log.Printf("%s has full replica set (%d of %d)\n", key, foundReplicas, r.s.Replication)
	}
	if satisfied && deleteLocal {
		r.cleanUpExcessReplica(key)
	}
	return nil
}

func (r rebalancer) checkNodesForRebalance(key key, nodesToCheck []node) (bool, bool, int) {
	var satisfied = false
	var foundReplicas = 0
	var deleteLocal = true

	for _, n := range nodesToCheck {
		if n.UUID == r.c.Myself.UUID {
			deleteLocal = false
			foundReplicas++
		} else {
			foundReplicas = foundReplicas + r.retrieveReplica(key, n, satisfied)
		}
		if foundReplicas >= r.s.Replication {
			satisfied = true
		}
		if foundReplicas >= r.s.MaxReplication {

			return satisfied, deleteLocal, foundReplicas
		}
	}
	return satisfied, deleteLocal, foundReplicas
}

func (r rebalancer) retrieveReplica(key key, n node, satisfied bool) int {
	local, err := n.RetrieveInfo(key, r.c.secret)
	if err == nil && local {
		return 1
	}
	if !n.Writeable {
		return 0
	}
	if !satisfied {
		b, err := r.s.Backend.Read(key)
		buf := bytes.NewBuffer(b)

		if err != nil {
			log.Printf("error reading from backend")
			return 0
		}
		if n.AddFile(key, buf, r.c.secret) {
			log.Printf("replicated %s\n", key)
			return 1
		}
		log.Println("write to the node failed, but what can we do?")
	}

	return 0
}

// our node is not at the front of the list, so
// we have an excess copy. clean that up and make room!
func (r rebalancer) cleanUpExcessReplica(key key) {
	err := r.s.Backend.Delete(key)
	if err != nil {
		log.Printf("could not clear out excess replica: %s\n", key)
		log.Println(err.Error())
	} else {
		log.Printf("cleared excess replica: %s\n", key)
	}
}
