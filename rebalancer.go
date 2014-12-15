package main

import (
	"bytes"
	"errors"
	"log"
)

type Rebalancer struct {
	c    *Cluster
	s    Site
	key  Key
	path string
}

func NewRebalancer(path string, key Key, c *Cluster, s Site) *Rebalancer {
	return &Rebalancer{c, s, key, path}
}

// check that the file is stored in at least Replication nodes
// and, if at all possible, those should be the ones at the front
// of the list
func (r Rebalancer) Rebalance() error {
	if r.c == nil {
		log.Println("can't rebalance on a nil cluster")
		return errors.New("nil cluster")
	}
	nodes_to_check := r.c.ReadOrder(r.key.String())
	satisfied, delete_local, found_replicas := r.checkNodesForRebalance(nodes_to_check)
	if !satisfied {
		log.Printf("could not replicate %s to %d nodes", r.key, r.s.Replication)
	} else {
		log.Printf("%s has full replica set (%d of %d)\n", r.key, found_replicas, r.s.Replication)
	}
	if satisfied && delete_local {
		r.clean_up_excess_replica()
	}
	return nil
}

func (r Rebalancer) checkNodesForRebalance(nodes_to_check []Node) (bool, bool, int) {
	var satisfied = false
	var found_replicas = 0
	var delete_local = true

	for _, n := range nodes_to_check {
		if n.UUID == r.c.Myself.UUID {
			delete_local = false
			found_replicas++
		} else {
			found_replicas = found_replicas + r.retrieveReplica(n, satisfied)
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

func (r Rebalancer) retrieveReplica(n Node, satisfied bool) int {
	file_info, err := n.RetrieveInfo(r.key)
	if err == nil && file_info != nil && file_info.Local {
		return 1
	} else {
		if !satisfied {
			b, err := r.s.Backend.Read(r.key)
			buf := bytes.NewBuffer(b)

			if err != nil {
				log.Printf("error reading from backend")
				return 0
			}
			if n.AddFile(r.key, buf) {
				log.Printf("replicated %s\n", r.key)
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
func (r Rebalancer) clean_up_excess_replica() {
	err := r.s.Backend.Delete(r.key)
	if err != nil {
		log.Printf("could not clear out excess replica: %s\n", r.key)
		log.Println(err.Error())
	} else {
		log.Printf("cleared excess replica: %s\n", r.key)
	}
}
