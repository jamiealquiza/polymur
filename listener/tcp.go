package listener

import (
	"bufio"
	"log"
	"net"
	"time"

	"github.com/jamiealquiza/polymur/statstracker"
)

type TCPListenerConfig struct {
	Addr          string
	IncomingQueue chan []*string
	FlushTimeout  int
	FlushSize     int
	Stats         *statstracker.Stats
}

// Listens for messages.
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
			if len(config.IncomingQueue) == cap(config.IncomingQueue) {
				log.Printf("Incoming queue capacity %d reached\n", cap(config.IncomingQueue))
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
