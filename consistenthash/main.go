// Package consistenthash is a Go port
// of the Graphite project's consistent-hashing
// algorithm. Givn the same settings (e.g. vnodes),
// consistenthash will yield the same node by name
// for a given key as you'd observe in carbon-relay.
package consistenthash

import (
	"crypto/md5"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"
)

// HashRing implmenents a consistent-hash
// ring with a configurable number of vnodes
// that are mapped to a list of real nodes.
type HashRing struct {
	sync.RWMutex
	Vnodes int
	nodes  nodeList
}

// node is used to reference a nodeName
// by a nodeID. A nodeID is a numeric value
// specifying the node's calculated hash-ring
// position, nodeName references the node's string
// name in a polymur connection pool.
type node struct {
	nodeID   int
	nodeName string
}

type nodeList []*node

// Implement functions for sort interface.
// This is to allow the bisection search / insort
// ring operations.
func (n nodeList) Len() int {
	return len(n)
}

func (n nodeList) Less(i, j int) bool {
	return n[i].nodeID < n[j].nodeID
}

func (n nodeList) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

// Hash ring operations.

// AddNode takes a node keyname and name.
// The name will populate the node.nodeName field,
// but we pass an explicit keyname value so that
// the hashing function is using the same naming convention
// as the consistent hashing implementation in Graphite.
// The Graphite project hashes nodes using the
// following format: "('127.0.0.1', 'a'):0".
func (h *HashRing) AddNode(keyname, name string) {
	h.Lock()

	for i := 0; i < h.Vnodes; i++ {
		nodeName := fmt.Sprintf("%s:%d", keyname, i)
		key := getHashKey(nodeName)
		h.nodes = append(h.nodes, &node{nodeID: key, nodeName: name})
	}

	sort.Sort(h.nodes)

	h.Unlock()
}

// RemoveNode drops a node from the hash ring.
func (h *HashRing) RemoveNode(name string) {
	h.Lock()

	newNodes := []*node{}
	for _, n := range h.nodes {
		if n.nodeName != name {
			newNodes = append(newNodes, n)
		}
	}

	h.nodes = newNodes

	h.Unlock()
}

// GetNode takes a key and returns the
// destination nodeName from the ring.
func (h *HashRing) GetNode(k string) (string, error) {
	if len(h.nodes) == 0 {
		return "", errors.New("Hash ring is empty")
	}

	h.RLock()

	// Hash the reference key.
	hk := getHashKey(k)

	// Get index in the ring.
	i := sort.Search(len(h.nodes), func(i int) bool { return h.nodes[i].nodeID >= hk }) % len(h.nodes)

	node := h.nodes[i].nodeName

	h.RUnlock()

	return node, nil
}

// getKey takes an input string (e.g. a metric or node name)
// and returns a hash key.
func getHashKey(s string) int {
	bigHash := md5.Sum([]byte(s))
	smallHash := fmt.Sprintf("%x", bigHash[:2])

	k, _ := strconv.ParseInt(smallHash, 16, 32)

	return int(k)
}
