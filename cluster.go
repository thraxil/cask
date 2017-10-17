package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/memberlist"
)

const replicas = 16

type cluster struct {
	Myself            *node
	secret            string
	neighbors         map[string]node
	chF               chan func()
	HeartbeatInterval int
}

func newCluster(myself *node, secret string, heartbeatInterval int) *cluster {
	rand.Seed(time.Now().UnixNano())
	if heartbeatInterval < 1 {
		// unset. default to 1 minute
		heartbeatInterval = 60
	}
	c := &cluster{
		Myself:            myself,
		secret:            secret,
		neighbors:         make(map[string]node),
		chF:               make(chan func()),
		HeartbeatInterval: heartbeatInterval,
	}
	go c.backend()

	return c
}

// implement memberlist.EventDelegate interface
func (c *cluster) NotifyJoin(node *memberlist.Node) {
	return
}

func (c *cluster) NotifyLeave(node *memberlist.Node) {
	return
}

func (c *cluster) NotifyUpdate(node *memberlist.Node) {
	return
}

// serialize all reads/writes through here
func (c *cluster) backend() {
	for f := range c.chF {
		f()
	}
}

func (c *cluster) AddNeighbor(n node) {
	c.chF <- func() {
		c.neighbors[n.UUID] = n
	}
}

func (c *cluster) RemoveNeighbor(n node) {
	c.chF <- func() {
		delete(c.neighbors, n.UUID)
	}
}

type gnresp struct {
	N []node
}

func (c *cluster) GetNeighbors() []node {
	r := make(chan gnresp)
	go func() {
		c.chF <- func() {
			neighbs := make([]node, len(c.neighbors))
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
	N   *node
	Err bool
}

func (c cluster) FindNeighborByUUID(uuid string) (*node, bool) {
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

func (c *cluster) UpdateNeighbor(neighbor node) {
	c.chF <- func() {
		if n, ok := c.neighbors[neighbor.UUID]; ok {
			n.BaseURL = neighbor.BaseURL
			n.Writeable = neighbor.Writeable
			if neighbor.LastSeen.Sub(n.LastSeen) > 0 {
				n.LastSeen = neighbor.LastSeen
			}
			c.neighbors[neighbor.UUID] = n
		}
	}
}

func (c *cluster) FailedNeighbor(neighbor node) {
	c.chF <- func() {
		if n, ok := c.neighbors[neighbor.UUID]; ok {
			n.Writeable = false
			n.LastFailed = time.Now()
			c.neighbors[neighbor.UUID] = n
		}
	}
}

type listResp struct {
	Ns []node
}

func (c cluster) NeighborsInclusive() []node {
	r := make(chan listResp)
	go func() {
		c.chF <- func() {
			a := make([]node, 1)
			a[0] = *c.Myself

			neighbs := make([]node, len(c.neighbors))
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

func (c cluster) WriteableNeighbors() []node {
	var all = c.NeighborsInclusive()
	var p []node // == nil
	for _, i := range all {
		if i.Writeable {
			p = append(p, i)
		}
	}
	return p
}

type ringEntry struct {
	Node node
	Hash string
}

type ringEntryList []ringEntry

func (p ringEntryList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p ringEntryList) Len() int           { return len(p) }
func (p ringEntryList) Less(i, j int) bool { return p[i].Hash < p[j].Hash }

func (c cluster) Ring() ringEntryList {
	// TODO: cache the ring so we don't have to regenerate
	// every time. it only changes when a node joins or leaves
	return neighborsToRing(c.NeighborsInclusive())
}

func (c cluster) WriteRing() ringEntryList {
	return neighborsToRing(c.WriteableNeighbors())
}

func neighborsToRing(neighbors []node) ringEntryList {
	keys := make(ringEntryList, replicas*len(neighbors))
	for i := range neighbors {
		node := neighbors[i]
		nkeys := node.HashKeys()
		for j := range nkeys {
			keys[i*replicas+j] = ringEntry{Node: node, Hash: nkeys[j]}
		}
	}
	sort.Sort(keys)
	return keys
}

// returns the list of all nodes in the order
// that the given hash will choose to write to them
func (c cluster) WriteOrder(hash string) []node {
	return hashOrder(hash, len(c.GetNeighbors())+1, c.WriteRing())
}

// returns the list of all nodes in the order
// that the given hash will choose to try to read from them
func (c cluster) ReadOrder(hash string) []node {
	return hashOrder(hash, len(c.GetNeighbors())+1, c.Ring())
}

func hashOrder(hash string, size int, ring []ringEntry) []node {
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
		// TODO: how will we support other hash types?
		if "sha1:"+r.Hash > hash {
			partitionIndex = i
			break
		}
	}
	// yay, slices
	reordered := make([]ringEntry, len(ring))
	reordered = append(ring[partitionIndex:], ring[:partitionIndex]...)

	results := make([]node, size)
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

func (c *cluster) updateNeighbor(neighbor node) {
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

func (c *cluster) Retrieve(key key) ([]byte, error) {
	// we don't have the full-size, so check the cluster
	nodesToCheck := c.ReadOrder(key.String())
	// this is where we go down the list and ask the other
	// nodes for the image
	// TODO: parallelize this
	for _, n := range nodesToCheck {
		if n.UUID == c.Myself.UUID {
			// checking ourself would be silly
			continue
		}
		log.Printf("ask node %s for it\n", n.UUID)
		f, err := n.Retrieve(key, c.secret)
		if err == nil {
			// got it, return it
			log.Println("   they had it")
			return f, nil
		}
		log.Println("   they didn't have it. try another")
		// that node didn't have it so we keep going
	}
	return nil, errors.New("not found in the cluster")
}

func (c *cluster) AddFile(key key, f multipart.File, replication int, minReplication int) bool {
	nodes := c.WriteOrder(key.String())
	var saveCount = 0
	for _, n := range nodes {
		if n.AddFile(key, f, c.secret) {
			saveCount++
			n.LastSeen = time.Now()
			c.UpdateNeighbor(n)
		} else {
			c.FailedNeighbor(n)
		}
		f.Seek(0, 0)
		if saveCount > replication {
			break
		}
	}
	return saveCount >= minReplication
}

func (c *cluster) JoinNeighbor(u string) (*node, error) {
	configURL := u + "/config/"
	res, err := http.Get(configURL)
	if err != nil {
		log.Println(err)
		return nil, errors.New("error retrieving config")
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, errors.New("error reading body of response")
	}

	var n node
	err = json.Unmarshal(body, &n)
	if err != nil {
		return nil, errors.New("error parsing json")
	}
	if n.UUID == c.Myself.UUID {
		return nil, errors.New("I can't join myself, silly!")
	}
	_, ok := c.FindNeighborByUUID(n.UUID)
	if ok {
		// let's not do updates through this. Let gossip handle that.
		return nil, errors.New("already have a node with that UUID in the cluster")
	}
	n.LastSeen = time.Now()
	c.AddNeighbor(n)
	// join the node to all our neighbors too
	for _, neighbor := range c.GetNeighbors() {
		if neighbor.UUID == n.UUID {
			// obviously, skip the one we just added
			continue
		}
		res, err = http.PostForm(neighbor.BaseURL+"/join/",
			url.Values{"url": {u}, "secret": {c.secret}})
		if err != nil {
			log.Println(err)
		} else {
			res.Body.Close()
		}
	}
	// reciprocate
	res, err = http.PostForm(n.BaseURL+"/join/",
		url.Values{"url": {c.Myself.BaseURL}, "secret": {c.secret}})
	if err != nil {
		return nil, err
	}
	res.Body.Close()
	return &n, nil
}

func (c *cluster) BootstrapNeighbors(neighbors string) {
	// wait a few seconds for other nodes to hopefully
	// have started up
	jitter := rand.Intn(5)
	time.Sleep(time.Duration(jitter) * time.Second)

	for _, n := range strings.Split(neighbors, ",") {
		// ignore any results/errors
		c.JoinNeighbor(n)
	}
}

type heartbeat struct {
	UUID      string `json:"uuid"`
	BaseURL   string `json:"base_url"`
	Writeable bool   `json:"writeable"`
	Secret    string `json:"secret"`

	Neighbors []nodeHeartbeat `json:"neighbors"`
}

func (c *cluster) Heartbeat() {
	baseTime := c.HeartbeatInterval
	for {
		jitter := rand.Intn(5)
		time.Sleep(time.Duration(baseTime+jitter) * time.Second)
		log.Println(" * heartbeat " + c.Myself.UUID + " *")
		neighbors := c.GetNeighbors()
		neighborHBS := make([]nodeHeartbeat, len(neighbors))
		for i, n := range neighbors {
			neighborHBS[i] = n.NodeHeartbeat()
		}
		var hb = heartbeat{
			UUID:      c.Myself.UUID,
			BaseURL:   c.Myself.BaseURL,
			Writeable: c.Myself.Writeable,
			Neighbors: neighborHBS,
			Secret:    c.secret,
		}
		for _, n := range neighbors {
			n.SendHeartbeat(hb)
		}
	}
}

func (c *cluster) Reaper() {
	// sleep for a couple heartbeat cycles when we first start up
	// to make sure everything has had time to settle
	baseTime := c.HeartbeatInterval
	jitter := rand.Intn(5)
	time.Sleep(time.Duration((baseTime*3)+jitter) * time.Second)
	// now on with the reaping

	var toReap []node
	// if we haven't heard from a node in over three heartbeats...
	reapPeriod := time.Duration(baseTime*3) * time.Second

	for {
		log.Println("Reaper wakes up...")
		neighbors := c.GetNeighbors()
		toReap = make([]node, 0)
		for _, n := range neighbors {
			if time.Since(n.LastSeen) > reapPeriod {
				// it's dead, Jim!
				toReap = append(toReap, n)
			}
		}
		for _, n := range toReap {
			log.Printf("reaping %s\n", n.UUID)
			c.RemoveNeighbor(n)
		}
		jitter := rand.Intn(5)
		time.Sleep(time.Duration(baseTime+jitter) * time.Second)
	}
}

func (c cluster) CheckSecret(s string) bool {
	return c.secret == s
}
