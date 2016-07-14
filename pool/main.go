// The MIT License (MIT)
//
// Copyright (c) 2016 Jamie Alquiza
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
package pool

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jamiealquiza/polymur/consistenthash"
)

type Destination struct {
	Ip   string
	Port string
	Id   string
	Addr string
	Name string
}

type Pool struct {
	sync.Mutex
	Ring               *consistenthash.HashRing
	Conns              map[string]chan *string
	Registered         map[string]time.Time
	DistributionMethod map[string]func(*Pool, []*string)
	Distribution       string
	QueueCap           int
	RetryQueue         chan []*string
}

func NewPool() *Pool {
	pool := &Pool{
		Ring:       &consistenthash.HashRing{},
		Conns:      make(map[string]chan *string),
		Registered: make(map[string]time.Time),
		DistributionMethod: map[string]func(*Pool, []*string){
			"broadcast":  (*Pool).broadcast,
			"hash-route": (*Pool).hashRoute,
		},
		RetryQueue: make(chan []*string, 4096),
	}

	return pool
}

// Distribution functions.

func (p *Pool) broadcast(messages []*string) {
	// For each message in the batch,
	for _, m := range messages {
		// enqueue into each available destination queue.
		for _, q := range p.Conns {
			select {
			case q <- m:
				continue
			default:
				// Skip if it's full.
				continue
			}
		}
	}
}

func (p *Pool) hashRoute(messages []*string) {
	for _, m := range messages {

		key := strings.Fields(*m)[0]
		node, err := p.Ring.GetNode(key)
		// Current failure mode if
		// the hash ring is empty.
		if err != nil {
			continue
		}

		select {
		case p.Conns[node] <- m:
			continue
		default:
			break
		}

		// If unavailable, load into failed messages for retry.
		failed := []*string{m}
		select {
		case p.RetryQueue <- failed:
			continue
		// If retryQueue is full, don't block message Distribution.
		default:
			continue
		}

	}
}

// Pool state update methods.

func (p *Pool) Register(dest Destination) {
	p.Lock()
	defer p.Unlock()

	log.Printf("Registered destination %s\n", dest.Name)
	p.Registered[dest.Name] = time.Now()
}

func (p *Pool) Unregister(dest Destination) {
	p.Lock()
	delete(p.Registered, dest.Name)
	p.Unlock()

	log.Printf("Unregistered destination %s\n", dest.Name)
	p.RemoveConn(dest)
}

// AddConn adds a connection's outbound queue
// to the global connection pool lists.
func (p *Pool) AddConn(dest Destination) {
	p.Lock()
	p.Conns[dest.Name] = make(chan *string, p.QueueCap)
	p.Unlock()

	// This replicates the destination key setup in
	// the carbon-cache implementation. It's a string composed of the
	// (destination IP, instance) tuple + :replica count. E.g. "('127.0.0.1', 'a'):0" for
	// the first replica for instance a listening on 127.0.0.1.
	// We statically append '0' since polymur isn't doing any replication handling.
	destString := fmt.Sprintf("('%s', '%s'):0", dest.Ip, dest.Id)
	p.Ring.AddNode(destString, dest.Name)
}

// RemoveConn removes a connection's outbound queue
// from the global connection pool lists.
// Additionally, it will redistribute any in-flight messages.
func (p *Pool) RemoveConn(dest Destination) {
	p.Lock()
	// Check if it exists, first.
	if _, connectionIsInPool := p.Conns[dest.Name]; !connectionIsInPool {
		p.Unlock()
		return
	}

	log.Printf("Removing destination %s from connection pool\n", dest.Name)

	// Grab the queue to redistribute any message it's holding.
	q := p.Conns[dest.Name]

	// Remove.
	delete(p.Conns, dest.Name)
	p.Unlock()

	p.Ring.RemoveNode(dest.Name)

	// Don't need to redistribute in-flight for broadcast.
	if p.Distribution == "broadcast" {
		return
	}
	// If the queue had any in-flight messages, redistribute them.
	close(q)
	if len(q) > 0 {
		log.Printf("Redistributing in-flight messages for %s", dest.Name)
		for m := range q {
			failed := []*string{m}
			p.RetryQueue <- failed
		}
	}
}

func ParseDestination(s string) (Destination, error) {
	d := Destination{Name: s}
	parts := strings.Split(s, ":")

	switch len(parts) {
	case 2:
		d.Ip, d.Port = parts[0], parts[1]
	case 3:
		d.Ip, d.Port, d.Id = parts[0], parts[1], parts[2]
	default:
		return d, fmt.Errorf("Destination %s not valid\n", s)
	}

	d.Addr = d.Ip + ":" + d.Port

	return d, nil
}
