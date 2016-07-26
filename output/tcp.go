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
package output

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/jamiealquiza/polymur/pool"
)

type TcpWriterConfig struct {
	Destinations  string
	Distribution  string
	IncomingQueue chan []*string
	QueueCap      int
}

func TcpWriter(p *pool.Pool, config *TcpWriterConfig, ready chan bool) {
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

		go DestinationWriter(p, dest)
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

// DestinationWriter requests a connection.
// It dequeues from the connection outbound buffer
// and writes to the respective destination.
func DestinationWriter(p *pool.Pool, dest pool.Destination) {

	// Get initial connection.
	p.Register(dest)
	conn, err := establishConn(p, dest)
	if err != nil {
		return
	}
	defer conn.Close()

	// Dequeue from destination outbound queue
	// and send.
	n := 1
	for {
		// Exponential backoff var
		// if the channel is empty.
		if n < 1000 {
			n = n * 2
		}

		// Need to make sure the connection
		// exists. It's possible that it becomes
		// unregistered (therefore doesn't exist) between
		// checking if it exists and attempting to read from it.
		p.Lock()
		_, ok := p.Conns[dest.Name]
		if !ok {
			p.Unlock()
			return
		}

		// Have to do a non-blocking read attempt, otherwise
		// unlocking the mutex will be blocked.
		select {
		case m, ok := <-p.Conns[dest.Name]:
			p.Unlock()

			if !ok {
				return
			}

			_, err := fmt.Fprintln(conn, *m)
			// If we fail to send, reload the message into the
			// queue and attempt to reconnect.
			if err != nil {
				p.Conns[dest.Name] <- m
				log.Printf("Destination %s error: %s\n", dest.Name, err)

				// Wait on a connection. If the destination isn't
				// registered, err and close this writer.
				newConn, err := establishConn(p, dest)
				if err != nil {
					break
				} else {
					conn = newConn
				}
			}
			// Reset backoff var.
			n = 1
		default:
			p.Unlock()
			time.Sleep(time.Duration(n) * time.Millisecond)
			continue
		}

	}
}

// establishConn manages TCP connections. Upon successful
// connection, establishConn will add the respective outbound
// queue to the global connection pool.	If a previously existing connection is
// being retried but fails for 3 consecutive attempts, it will be removed
// from the global pool. Background attempts will continue and the connection
// will rejoin the pool upon success.
func establishConn(p *pool.Pool, dest pool.Destination) (net.Conn, error) {
	retry := 0
	retryMax := 3

	for {
		// If it's not registered, abort.
		if _, destinationRegistered := p.Registered[dest.Name]; !destinationRegistered {
			return nil, errors.New("Destination not registered")
		}

		_, connectionIsInPool := p.Conns[dest.Name]

		// Are we retrying a previously established connection that failed?
		if retry >= retryMax && connectionIsInPool {
			log.Printf("Exceeded retry count (%d) for destination %s\n", retryMax, dest.Name)
			p.RemoveConn(dest)
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
				p.AddConn(dest)
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
func retryMessageHandler(p *pool.Pool) {
	flushTimeout := time.Tick(15 * time.Second)
	messages := []*string{}
	batchSize := 30

	for {
		// We hit the flush timeout, load the current batch if present.
		select {
		case <-flushTimeout:
			if len(messages) > 0 {
				p.DistributionMethod[p.Distribution](p, messages)
				messages = []*string{}
			}
			messages = []*string{}
		case retry := <-p.RetryQueue:
			// If this puts us at the batchSize threshold, enqueue
			// into the messageIncomingQueue.
			if len(messages)+1 >= batchSize {
				messages = append(messages, retry...)
				// Lazy latency injection to tame loops. See TODO.
				time.Sleep(500 * time.Millisecond)
				p.DistributionMethod[p.Distribution](p, messages)
				messages = []*string{}
			} else {
				// Otherwise, just append message to current batch.
				messages = append(messages, retry...)
			}
		}
	}
}
