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
package polymur

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
	
	"github.com/jamiealquiza/polymur/pool"
)

type OutputTcpConfig struct {
	Destinations string
	Distribution string
	IncomingQueue chan []*string
	QueueCap int
}


func OutputTcp(p *pool.Pool, config *OutputTcpConfig, ready chan bool) {
	p.Lock()
	p.Distribution = config.Distribution
	p.QueueCap = config.QueueCap
	p.Unlock()

	go retryMessageHandler(p)

	Destinations := strings.Split(config.Destinations, ",")
	for _, addr := range Destinations {
		if addr == "" {
			continue
		}

		dest, err := pool.ParseDestination(addr)
		if err != nil {
			fmt.Println(err)
			continue
		}

		go destinationWriter(p, dest)
	}

	// In case we want any initialization to block.
	// Lazily give writers a head start before the listener.
	// TODO just add WGs.
	time.Sleep(1 * time.Second)
	ready <- true

	// Pop messages from the incoming queue and distribute.
	for messages := range config.IncomingQueue {
		p.DistributionMethod[config.Distribution](p, messages)
	}
}

// destinationWriter requests a connection.
// It dequeues from the connection outbound buffer
// and writes to the respective destination.
func destinationWriter(pool *pool.Pool, dest pool.Destination) {

	// Get initial connection.
	pool.Register(dest)
	conn, err := establishConn(pool, dest)
	if err != nil {
		return
	}
	defer conn.Close()

	// Dequeue from destination outbound queue
	// and send.
	for m := range pool.Conns[dest.Name] {
		_, err := fmt.Fprintln(conn, *m)
		// If we fail to send, reload the message into the
		// queue and attempt to reconnect.
		if err != nil {
			pool.Conns[dest.Name] <- m
			log.Printf("Destination %s error: %s\n", dest.Name, err)

			// Wait on a connection. If the destination isn't
			// registered, err and close this writer.
			newConn, err := establishConn(pool, dest)
			if err != nil {
				break
			} else {
				conn = newConn
			}
		}
	}

}

// establishConn manages TCP connections. Upon successful
// connection, establishConn will add the respective outbound
// queue to the global connection pool.	If a previously existing connection is
// being retried but fails for 3 consecutive attempts, it will be removed
// from the global pool. Background attempts will continue and the connection
// will rejoin the pool upon success.

// Should probably embed all this logic directly in the pool.
func establishConn(pool *pool.Pool, dest pool.Destination) (net.Conn, error) {
	retry := 0
	retryMax := 3

	for {
		// If it's not registered, abort.
		if _, destinationRegistered := pool.Registered[dest.Name]; !destinationRegistered {
			return nil, errors.New("Destination not registered")
		}

		_, connectionIsInPool := pool.Conns[dest.Name]

		// Are we retrying a previously established connection that failed?
		if retry >= retryMax && connectionIsInPool {
			log.Printf("Exceeded retry count (%d) for destination %s\n", retryMax, dest.Name)
			pool.RemoveConn(dest)
		}

		// Try a connection every 10s.
		conn, err := net.DialTimeout("tcp", dest.Addr, time.Duration(3*time.Second))

		if err != nil {
			log.Printf("Destination error: %s, retrying in 10s\n", err)
			time.Sleep(10 * time.Second)
			// Increment failure count.
			retry++
			continue
		} else {
			// If this connection succeeds and is not in the pool
			if !connectionIsInPool {
				log.Printf("Adding destination to connection pool: %s\n", dest.Name)
				pool.AddConn(dest)
			} else {
				// If this connection is still in the pool, we're
				// likely here due to a temporary disconnect.
				log.Printf("Reconnected to destination: %s\n", dest.Name)
			}

			return conn, nil
		}

	}

}

// retryMessageHandler catches any messages loaded
// into the failedMessage queue and retries Distribution.
// TODO: needs exponential backoff when no Destinations
// are available; messages will enter a tight loop.
func retryMessageHandler(pool *pool.Pool) {
	flushTimeout := time.Tick(15 * time.Second)
	messages := []*string{}
	batchSize := 30

	for {
		// We hit the flush timeout, load the current batch if present.
		select {
		case <-flushTimeout:
			if len(messages) > 0 {
				pool.DistributionMethod[pool.Distribution](pool, messages)
				messages = []*string{}
			}
			messages = []*string{}
		case retry := <-pool.RetryQueue:
			// If this puts us at the batchSize threshold, enqueue
			// into the messageIncomingQueue.
			if len(messages)+1 >= batchSize {
				messages = append(messages, retry...)
				// Lazy latency injection to tame loops. See TODO.
				time.Sleep(500 * time.Millisecond)
				pool.DistributionMethod[pool.Distribution](pool, messages)
				messages = []*string{}
			} else {
				// Otherwise, just append message to current batch.
				messages = append(messages, retry...)
			}
		}
	}
}