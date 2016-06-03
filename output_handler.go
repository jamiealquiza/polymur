package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

var (
	pool = &Pool{
		Ring:       &HashRing{},
		Conns:      make(map[string]chan *string),
		Registered: make(map[string]time.Time),
	}

	distributionMethod = map[string]func([]*string){
		"broadcast":  broadcast,
		"hash-route": hashRoute,
	}

	retryQueue = make(chan []*string, 4096)
)

func broadcast(messages []*string) {
	// For each message in the batch,
	for _, m := range messages {
		// enqueue into each available destination queue.
		for _, q := range pool.Conns {
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

func hashRoute(messages []*string) {
	for _, m := range messages {

		key := strings.Fields(*m)[0]
		node := pool.Ring.GetNode(key)

		select {
		case pool.Conns[node] <- m:
			continue
		default:
			break
		}

		// If unavailable, load into failed messages for retry.
		failed := []*string{m}
		select {
		case retryQueue <- failed:
			continue
		// If retryQueue is full, don't block message distribution.
		default:
			continue
		}

	}
}

// retryMessageHandler catches any messages loaded
// into the failedMessage queue and retries distribution.
// TODO: needs exponential backoff when no destinations
// are available; messages will enter a tight loop.
func retryMessageHandler() {
	flushTimeout := time.Tick(15 * time.Second)
	messages := []*string{}
	batchSize := 30

	for {
		// We hit the flush timeout, load the current batch if present.
		select {
		case <-flushTimeout:
			if len(messages) > 0 {
				distributionMethod[options.distribution](messages)
				messages = []*string{}
			}
			messages = []*string{}
		case retry := <-retryQueue:
			// If this puts us at the batchSize threshold, enqueue
			// into the messageIncomingQueue.
			if len(messages)+1 >= batchSize {
				messages = append(messages, retry...)
				// Lazy latency injection to tame loops. See TODO.
				time.Sleep(500 * time.Millisecond)
				distributionMethod[options.distribution](messages)
				messages = []*string{}
			} else {
				// Otherwise, just append message to current batch.
				messages = append(messages, retry...)
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
func establishConn(dest destination) (net.Conn, error) {
	retry := 0
	retryMax := 3

	for {
		// If it's not registered, abort.
		if _, destinationRegistered := pool.Registered[dest.name]; !destinationRegistered {
			return nil, errors.New("Destination not registered")
		}

		_, connectionIsInPool := pool.Conns[dest.name]

		// Are we retrying a previously established connection that failed?
		if retry >= retryMax && connectionIsInPool {
			log.Printf("Exceeded retry count (%d) for destination %s\n", retryMax, dest.name)
			pool.removeConn(dest)
		}

		// Try a connection every 10s.
		conn, err := net.DialTimeout("tcp", dest.addr, time.Duration(3*time.Second))

		if err != nil {
			log.Printf("Destination error: %s, retrying in 10s\n", err)
			time.Sleep(10 * time.Second)
			// Increment failure count.
			retry++
			continue
		} else {
			// If this connection succeeds and is not in the pool
			if !connectionIsInPool {
				log.Printf("Adding destination to connection pool: %s\n", dest.name)
				pool.addConn(dest)
			} else {
				// If this connection is still in the pool, we're
				// likely here due to a temporary disconnect.
				log.Printf("Reconnected to destination: %s\n", dest.name)
			}

			return conn, nil
		}

	}

}

// destinationWriter requests a connection.
// It dequeues from the connection outbound buffer
// and writes to the respective destination.
func destinationWriter(dest destination) {

	// Get initial connection.
	pool.register(dest)
	conn, err := establishConn(dest)
	if err != nil {
		return
	}
	defer conn.Close()

	// Dequeue from destination outbound queue
	// and send.
	for m := range pool.Conns[dest.name] {
		_, err := fmt.Fprintln(conn, *m)
		// If we fail to send, reload the message into the
		// queue and attempt to reconnect.
		if err != nil {
			pool.Conns[dest.name] <- m
			log.Printf("Destination %s error: %s\n", dest.name, err)

			// Wait on a connection. If the destination isn't
			// registered, err and close this writer.
			newConn, err := establishConn(dest)
			if err != nil {
				break
			} else {
				conn = newConn
			}
		}
	}

}

func outputHandler(q chan []*string, ready chan bool) {
	go retryMessageHandler()

	destinations := strings.Split(options.destinations, ",")
	for _, addr := range destinations {
		if addr == "" {
			continue
		}

		dest, err := parseDestination(addr)
		if err != nil {
			fmt.Println(err)
			continue
		}

		go destinationWriter(dest)
	}

	// In case we want any initialization to block.
	// Lazily give writers a head start before the listener.
	// TODO just add WGs.
	time.Sleep(1 * time.Second)
	ready <- true

	// Pop messages from the incoming queue and distribute.
	for messages := range q {
		distributionMethod[options.distribution](messages)
	}
}
