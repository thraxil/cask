package main

import (
	"bytes"
	"errors"
	"log"
)

type rebalanceRequest struct {
	Key        Key
	chResponse chan error
}

type Rebalancer struct {
	c   *Cluster
	s   Site
	chR chan rebalanceRequest
}

func NewRebalancer(c *Cluster, s Site) *Rebalancer {
	ch := make(chan rebalanceRequest)
	r := Rebalancer{c, s, ch}
	go r.run()
	return &r
}

func (r *Rebalancer) run() {
	for req := range r.chR {
		req.chResponse <- r.doRebalance(req.Key)
	}
}

func (r *Rebalancer) Rebalance(key Key) error {
	resp := make(chan error)
	req := rebalanceRequest{key, resp}
	r.chR <- req
	return <-resp
}

// check that the file is stored in at least Replication nodes
// and, if at all possible, those should be the ones at the front
// of the list
func (r Rebalancer) doRebalance(key Key) error {
	if r.c == nil {
		log.Println("can't rebalance on a nil cluster")
		return errors.New("nil cluster")
	}
	nodes_to_check := r.c.ReadOrder(key.String())
	satisfied, delete_local, found_replicas := r.checkNodesForRebalance(key, nodes_to_check)
	if !satisfied {
		log.Printf("could not replicate %s to %d nodes", key, r.s.Replication)
	} else {
		log.Printf("%s has full replica set (%d of %d)\n", key, found_replicas, r.s.Replication)
	}
	if satisfied && delete_local {
		r.clean_up_excess_replica(key)
	}
	return nil
}

func (r Rebalancer) checkNodesForRebalance(key Key, nodes_to_check []Node) (bool, bool, int) {
	var satisfied = false
	var found_replicas = 0
	var delete_local = true

	for _, n := range nodes_to_check {
		if n.UUID == r.c.Myself.UUID {
			delete_local = false
			found_replicas++
		} else {
			found_replicas = found_replicas + r.retrieveReplica(key, n, satisfied)
		}
		if found_replicas >= r.s.Replication {
			satisfied = true
		}
		if found_replicas >= r.s.MaxReplication {

			return satisfied, delete_local, found_replicas
		}
	}
	return satisfied, delete_local, found_replicas
}

func (r Rebalancer) retrieveReplica(key Key, n Node, satisfied bool) int {
	local, err := n.RetrieveInfo(key, r.c.secret)
	if err == nil && local {
		return 1
	} else {
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
			} else {
				log.Println("write to the node failed, but what can we do?")
			}
		}
	}
	return 0
}

// our node is not at the front of the list, so
// we have an excess copy. clean that up and make room!
func (r Rebalancer) clean_up_excess_replica(key Key) {
	err := r.s.Backend.Delete(key)
	if err != nil {
		log.Printf("could not clear out excess replica: %s\n", key)
		log.Println(err.Error())
	} else {
		log.Printf("cleared excess replica: %s\n", key)
	}
}
