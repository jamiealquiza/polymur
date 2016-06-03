package main

import (
	"crypto/md5"
	"fmt"
	"sort"
	"strconv"
	"sync"
)

// HashRing provides a consistent hashing
// mechanism that mirrors the placement algorithm
// used in the Graphite project carbon-cache daemon.
type HashRing struct {
	sync.Mutex
	nodes nodeList
}

type node struct {
	nodeId int
	nodeName string
}

type nodeList []*node

// Implement functions for sort interface.
func (n nodeList) Len() int {
	return len(n)
}

func (n nodeList) Less(i, j int) bool {
	return n[i].nodeId < n[j].nodeId
}

func (n nodeList) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}


// Hash ring operations.

func (h *HashRing) AddNode(dest destination) {
	h.Lock()

	// This replicates the destination key setup in
	// the carbon-cache implementation. It's a string composed of the
	// (destination IP, instance) tuple + :replica count. E.g. "('127.0.0.1', 'a'):0" for 
	// the first replica for instance a listening on 127.0.0.1.
	// We statically append '0' since polymur isn't doing any replication handling.
	destString := fmt.Sprintf("('%s', '%s'):0", dest.ip, dest.id)

	key := getHashKey(destString)
	h.nodes = append(h.nodes, &node{nodeId: key, nodeName: dest.name})
	sort.Sort(h.nodes)

	h.Unlock()
}

func (h *HashRing) RemoveNode(dest destination) {
	h.Lock()

	newNodes := []*node{}
	for _, n := range h.nodes {
		if n.nodeName != dest.addr {
			newNodes = append(newNodes, n)
		}
	}

	h.nodes = newNodes

	h.Unlock()
}

// GetNode takes a key and returns the
// destination nodeName from the ring.
func (h *HashRing) GetNode(k string) string {
	h.Lock()

	// Hash the reference key.
	hk := getHashKey(k)
	// Get index in the ring.
	i := sort.Search(len(h.nodes), func(i int) bool { return h.nodes[i].nodeId >= hk })

	node := h.nodes[i].nodeName

	h.Unlock()

	return node
}

// getKey takes an input string (e.g. a metric or node name)
// and returns a hash key.
func getHashKey(s string) int {
	bigHash := md5.Sum([]byte(s))
	smallHash := fmt.Sprintf("%x", bigHash[:2])

	k, _ := strconv.ParseInt(smallHash, 16, 32)

	return int(k)
}