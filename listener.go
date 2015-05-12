package main

import (
	"bufio"
	"log"
	"net"
	"time"
)

func init() {
	config.batchSize = 30
	config.flushTimeout = 5
}

// Listens for messages.
func listener(s *Statser) {
	log.Printf("Metrics listener started: %s:%s\n",
		options.addr,
		options.port)
	server, err := net.Listen("tcp", options.addr+":"+options.port)
	if err != nil {
		log.Fatalf("Listener error: %s\n", err)
	}
	defer server.Close()
	// Connection handler loop.
	for {
		conn, err := server.Accept()
		if err != nil {
			log.Printf("Listener down: %s\n", err)
			continue
		}
		go connectionHandler(conn, s)
	}
}

func connectionHandler(c net.Conn, s *Statser) {
	flushTimeout := time.Tick(time.Duration(config.flushTimeout) * time.Second)
	messages := []*string{}

	inbound := bufio.NewScanner(c)
	defer c.Close()

	for inbound.Scan() {

		// We hit the flush timeout, load the current batch if present.
		select {
		case <-flushTimeout:
			if len(messages) > 0 {
				messageIncomingQueue <- messages
				messages = []*string{}
			}
			messages = []*string{}
		default:
			break
		}

		m := inbound.Text()
		s.IncrRecv(1)

		// Drop message and respond if the incoming queue is at capacity.
		if len(messageIncomingQueue) >= options.queuecap {
			log.Printf("Queue capacity %d reached\n", options.queuecap)
			// Impose flow control. This needs to be significantly smarter.
			time.Sleep(1 * time.Second)
		}

		// If this puts us at the batchSize threshold, enqueue
		// into the messageIncomingQueue.
		if len(messages)+1 >= config.batchSize {
			messages = append(messages, &m)
			messageIncomingQueue <- messages
			messages = []*string{}
		} else {
			// Otherwise, just append message to current batch.
			messages = append(messages, &m)
		}

	}

	messageIncomingQueue <- messages

}
