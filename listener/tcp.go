// Package listener handles the ingestion for polymur
//
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
// all copies or substantial Portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
package listener

import (
	"bufio"
	"log"
	"net"
	"time"

	"github.com/chrissnell/polymur/statstracker"
)

// TCPListenerConfig holds configuration for the TCP listener
type TCPListenerConfig struct {
	Addr          string
	IncomingQueue chan []*string
	FlushTimeout  int
	FlushSize     int
	Stats         *statstracker.Stats
}

// TCPListener Listens for messages.
func TCPListener(config *TCPListenerConfig) {
	log.Printf("Metrics listener started: %s\n", config.Addr)
	server, err := net.Listen("tcp", config.Addr)
	if err != nil {
		log.Fatalf("Listener error: %s\n", err)
	}
	defer server.Close()

	// Connection handler loop.
	for {
		conn, err := server.Accept()
		if err != nil {
			log.Printf("Connection handler error: %s\n", err)
			time.Sleep(1 * time.Second)
			continue
		}
		go connectionHandler(config, conn)
	}
}

func connectionHandler(config *TCPListenerConfig, c net.Conn) {
	messages := make(chan string, 128)
	go messageBatcher(messages, config)

	inbound := bufio.NewScanner(c)
	defer c.Close()

	for inbound.Scan() {
		m := inbound.Text()
		messages <- m
		config.Stats.UpdateCount(1)
	}

	close(messages)
}

func messageBatcher(messages chan string, config *TCPListenerConfig) {
	flushTimeout := time.NewTicker(time.Duration(config.FlushTimeout) * time.Second)
	defer flushTimeout.Stop()

	batch := make([]*string, config.FlushSize)
	pos := 0

run:
	for {
		// We hit the flush timeout, load the current batch if present.
		select {
		case <-flushTimeout.C:
			if len(batch) > 0 {
				config.IncomingQueue <- batch
				batch = make([]*string, config.FlushSize)
				pos = 0
			}
		case m, ok := <-messages:
			if !ok {
				break run
			}

			// Drop message and respond if the incoming queue is at capacity.
			if len(config.IncomingQueue) >= 32768 {
				log.Printf("Incoming queue capacity %d reached\n", 32768)
				// Needs some flow control logic.
			}

			// If this puts us at the FlushSize threshold, enqueue
			// into the q.
			if pos+1 >= config.FlushSize {
				batch[config.FlushSize-1] = &m
				config.IncomingQueue <- batch
				batch = make([]*string, config.FlushSize)
				pos = 0
			} else {
				// Otherwise, just append message to current batch.
				batch[pos] = &m
				pos++
			}
		}
	}

	// Load any partial batch before
	// we return.
	config.IncomingQueue <- batch
}
