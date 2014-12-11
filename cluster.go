package main

import (
	"errors"
	"sort"
	"time"
)

const REPLICAS = 16

type Cluster struct {
	Myself    Node
	neighbors map[string]Node
	chF       chan func()
}

func NewCluster(myself Node) *Cluster {
	c := &Cluster{
		Myself:    myself,
		neighbors: make(map[string]Node),
		chF:       make(chan func()),
	}
	go c.backend()
	return c
}

// serialize all reads/writes through here
func (c *Cluster) backend() {
	for f := range c.chF {
		f()
	}
}

func (c *Cluster) AddNeighbor(n Node) {
	c.chF <- func() {
		c.neighbors[n.UUID] = n
	}
}

func (c *Cluster) RemoveNeighbor(n Node) {
	c.chF <- func() {
		delete(c.neighbors, n.UUID)
	}
}

type gnresp struct {
	N []Node
}

func (c *Cluster) GetNeighbors() []Node {
	r := make(chan gnresp)
	go func() {
		c.chF <- func() {
			neighbs := make([]Node, len(c.neighbors))
			var i = 0
			for _, value := range c.neighbors {
				neighbs[i] = value
				i++
			}
			r <- gnresp{neighbs}
		}
	}()
	resp := <-r
	return resp.N
}

type fResp struct {
	N   *Node
	Err bool
}

func (c Cluster) FindNeighborByUUID(uuid string) (*Node, bool) {
	r := make(chan fResp)
	go func() {
		c.chF <- func() {
			n, ok := c.neighbors[uuid]
			r <- fResp{&n, ok}
		}
	}()
	resp := <-r
	return resp.N, resp.Err
}

func (c *Cluster) UpdateNeighbor(neighbor Node) {
	c.chF <- func() {
		if n, ok := c.neighbors[neighbor.UUID]; ok {
			n.BaseUrl = neighbor.BaseUrl
			n.Writeable = neighbor.Writeable
			if neighbor.LastSeen.Sub(n.LastSeen) > 0 {
				n.LastSeen = neighbor.LastSeen
			}
			c.neighbors[neighbor.UUID] = n
		}
	}
}

func (c *Cluster) FailedNeighbor(neighbor Node) {
	c.chF <- func() {
		if n, ok := c.neighbors[neighbor.UUID]; ok {
			n.Writeable = false
			n.LastFailed = time.Now()
			c.neighbors[neighbor.UUID] = n
		}
	}
}

type listResp struct {
	Ns []Node
}

func (c Cluster) NeighborsInclusive() []Node {
	r := make(chan listResp)
	go func() {
		c.chF <- func() {
			a := make([]Node, 1)
			a[0] = c.Myself

			neighbs := make([]Node, len(c.neighbors))
			var i = 0
			for _, value := range c.neighbors {
				neighbs[i] = value
				i++
			}

			a = append(a, neighbs...)
			r <- listResp{a}
		}
	}()
	resp := <-r
	return resp.Ns
}

func (c Cluster) WriteableNeighbors() []Node {
	var all = c.NeighborsInclusive()
	var p []Node // == nil
	for _, i := range all {
		if i.Writeable {
			p = append(p, i)
		}
	}
	return p
}

type RingEntry struct {
	Node Node
	Hash string
}

type RingEntryList []RingEntry

func (p RingEntryList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p RingEntryList) Len() int           { return len(p) }
func (p RingEntryList) Less(i, j int) bool { return p[i].Hash < p[j].Hash }

func (c Cluster) Ring() RingEntryList {
	// TODO: cache the ring so we don't have to regenerate
	// every time. it only changes when a node joins or leaves
	return neighborsToRing(c.NeighborsInclusive())
}

func (c Cluster) WriteRing() RingEntryList {
	return neighborsToRing(c.WriteableNeighbors())
}

func (cluster *Cluster) Stash(key Key, replication int, min_replication int) []string {
	// we don't have the full-size, so check the cluster
	nodes_to_check := cluster.WriteOrder(key.String())
	saved_to := make([]string, replication)
	var save_count = 0
	// TODO: parallelize this
	for _, n := range nodes_to_check {
		// TODO: detect when the node to stash to is the current one
		// and just save directly instead of doing a POST to ourself
		if n.Stash(key) {
			saved_to[save_count] = n.UUID
			save_count++
			n.LastSeen = time.Now()
			cluster.UpdateNeighbor(n)
		} else {
			cluster.FailedNeighbor(n)
		}
		// TODO: if we've hit min_replication, we can return
		// immediately and leave any additional stash attempts
		// as background processes
		// that node didn't have it so we keep going
		if save_count >= replication {
			// got as many as we need
			break
		}
	}
	return saved_to
}

func neighborsToRing(neighbors []Node) RingEntryList {
	keys := make(RingEntryList, REPLICAS*len(neighbors))
	for i := range neighbors {
		node := neighbors[i]
		nkeys := node.HashKeys()
		for j := range nkeys {
			keys[i*REPLICAS+j] = RingEntry{Node: node, Hash: nkeys[j]}
		}
	}
	sort.Sort(keys)
	return keys
}

// returns the list of all nodes in the order
// that the given hash will choose to write to them
func (c Cluster) WriteOrder(hash string) []Node {
	return hashOrder(hash, len(c.GetNeighbors())+1, c.WriteRing())
}

// returns the list of all nodes in the order
// that the given hash will choose to try to read from them
func (c Cluster) ReadOrder(hash string) []Node {
	return hashOrder(hash, len(c.GetNeighbors())+1, c.Ring())
}

func hashOrder(hash string, size int, ring []RingEntry) []Node {
	// our approach is to find the first bucket after our hash,
	// partition the ring on that and put the first part on the
	// end. Then go through and extract the ordering.

	// so, with a ring of [1,2,3,4,5,6,7,8,9,10]
	// and a hash of 7, we partition it into
	// [1,2,3,4,5,6] and [7,8,9,10]
	// then recombine them into
	// [7,8,9,10] + [1,2,3,4,5,6]
	// [7,8,9,10,1,2,3,4,5,6]
	var partitionIndex = 0
	for i, r := range ring {
		if r.Hash > hash {
			partitionIndex = i
			break
		}
	}
	// yay, slices
	reordered := make([]RingEntry, len(ring))
	reordered = append(ring[partitionIndex:], ring[:partitionIndex]...)

	results := make([]Node, size)
	var seen = map[string]bool{}
	var i = 0
	for _, r := range reordered {
		if !seen[r.Node.UUID] {
			results[i] = r.Node
			i++
			seen[r.Node.UUID] = true
		}
	}
	return results
}

func (c *Cluster) updateNeighbor(neighbor Node) {
	if neighbor.UUID == c.Myself.UUID {
		// as usual, skip ourself
		return
	}
	// TODO: convert these to a single atomic
	// UpdateOrAddNeighbor type operation
	if _, ok := c.FindNeighborByUUID(neighbor.UUID); ok {
		c.UpdateNeighbor(neighbor)
	} else {
		// heard about another node second hand
		c.AddNeighbor(neighbor)
	}
}

func (c *Cluster) Retrieve(key Key) ([]byte, error) {
	// we don't have the full-size, so check the cluster
	nodes_to_check := c.ReadOrder(key.String())
	// this is where we go down the list and ask the other
	// nodes for the image
	// TODO: parallelize this
	for _, n := range nodes_to_check {
		if n.UUID == c.Myself.UUID {
			// checking ourself would be silly
			continue
		}
		f, err := n.Retrieve(key)
		if err == nil {
			// got it, return it
			return f, nil
		}
		// that node didn't have it so we keep going
	}
	return nil, errors.New("not found in the cluster")
}