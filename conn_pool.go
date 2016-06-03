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
package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

type destination struct {
	ip   string
	port string
	id   string
	addr string
	name string
}

func parseDestination(s string) (destination, error) {
	d := destination{name: s}
	parts := strings.Split(s, ":")

	switch len(parts) {
	case 2:
		d.ip, d.port = parts[0], parts[1]
	case 3:
		d.ip, d.port, d.id = parts[0], parts[1], parts[2]
	default:
		return d, fmt.Errorf("Destination %s not valid\n", s)
	}

	d.addr = d.ip + ":" + d.port

	return d, nil
}

type Pool struct {
	sync.Mutex
	Ring       *HashRing
	Conns      map[string]chan *string
	Registered map[string]time.Time
}

func (p *Pool) register(dest destination) {
	p.Lock()
	defer p.Unlock()

	log.Printf("Registered destination %s\n", dest.name)
	p.Registered[dest.name] = time.Now()
}

func (p *Pool) unregister(dest destination) {
	p.Lock()
	delete(p.Registered, dest.name)
	p.Unlock()

	log.Printf("Unregistered destination %s\n", dest.name)
	p.removeConn(dest)
}

// addConn adds a connection's outbound queue
// to the global connection pool lists.
func (p *Pool) addConn(dest destination) {
	p.Lock()

	p.Conns[dest.name] = make(chan *string, options.queuecap)
	p.Ring.AddNode(dest)

	p.Unlock()
}

// removeConn removes a connection's outbound queue
// from the global connection pool lists.
// Additionally, it will redistribute any in-flight messages.
func (p *Pool) removeConn(dest destination) {
	// Check if it exists, first.
	if _, connectionIsInPool := pool.Conns[dest.name]; !connectionIsInPool {
		return
	}

	log.Printf("Removing destination %s from connection pool\n", dest.name)

	p.Lock()

	// Grab the queue to redistribute any message it's holding.
	q := p.Conns[dest.name]

	// Remove.
	delete(p.Conns, dest.name)
	p.Ring.RemoveNode(dest)

	p.Unlock()

	// Don't need to redistribute in-flight for broadcast.
	if options.distribution == "broadcast" {
		return
	}
	// If the queue had any in-flight messages, redistribute them.
	close(q)
	if len(q) > 0 {
		log.Printf("Redistributing in-flight messages for %s", dest.name)
		for m := range q {
			failed := []*string{m}
			retryQueue <- failed
		}
	}
}
