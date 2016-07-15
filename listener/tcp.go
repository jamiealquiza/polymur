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

	"github.com/jamiealquiza/polymur/statstracker"
)

type ListenerConfig struct {
	Addr          string
	IncomingQueue chan []*string
}

// Listens for messages.
func ListenTcp(config *ListenerConfig, s *statstracker.Stats) {
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
		go connectionHandler(conn, config.IncomingQueue, s)
	}
}

func connectionHandler(c net.Conn, q chan []*string, s *statstracker.Stats) {
	flushTimeout := time.NewTicker(5 * time.Second)
	defer flushTimeout.Stop()

	messages := []*string{}

	inbound := bufio.NewScanner(c)
	defer c.Close()

	for inbound.Scan() {

		// We hit the flush timeout, load the current batch if present.
		select {
		case <-flushTimeout.C:
			if len(messages) > 0 {
				q <- messages
				messages = []*string{}
			}
			messages = []*string{}
		default:
			break
		}

		m := inbound.Text()
		s.UpdateCount(1)

		// Drop message and respond if the incoming queue is at capacity.
		if len(q) >= 32768 {
			log.Printf("Incoming queue capacity %d reached\n", 32768)
			// Impose flow control. This needs to be significantly smarter.
			time.Sleep(1 * time.Second)
		}

		// If this puts us at the batchSize threshold, enqueue
		// into the q.
		if len(messages)+1 >= 30 {
			messages = append(messages, &m)
			q <- messages
			messages = []*string{}
		} else {
			// Otherwise, just append message to current batch.
			messages = append(messages, &m)
		}

	}

	q <- messages

}
