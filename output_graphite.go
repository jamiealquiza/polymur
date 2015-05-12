package main

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

type Pool struct {
	sync.Mutex
	Conns     map[string]chan *string
	ConnsList []chan *string
	RRCurrent int
}

var (
	pool           = &Pool{Conns: make(map[string]chan *string)}
	failedMessages = make(chan []*string, 4096)

	distributionMethod = map[string]func([]*string){
		"broadcast":  broadcast,
		"balance-rr": balanceRR,
	}
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

func balanceRR(messages []*string) {
	// Fetch current the RR ID.
	pos := pool.RRCurrent

	for _, m := range messages {

		// If no destinations are available, we're
		// going to get a negative position val.
		// Load the whole batch into the retry.
		pos = pool.nextRR(pos)

		if pos < 0 {
			log.Println("No destinations available, further messages will be dropped")
			break
		}

		// Attempt current RR node.
		select {
		case pool.ConnsList[pos] <- m:
			continue
		default:
			break
		}

		// If unavailable, load into failed messages for retry.
		failed := []*string{m}
		select {
		case failedMessages <- failed:
			continue
		// If failedMessages is full, don't block message distribution.
		default:
			continue
		}

	}

	// Commit the next RR ID to continue with.
	pool.commitRR(pos)
}

func (p *Pool) commitRR(pos int) {
	p.Lock()
	defer p.Unlock()
	p.RRCurrent = pos
}

func (p *Pool) nextRR(pos int) int {
	max := len(p.ConnsList) - 1
	if max < 0 {
		return -1
	}

	pos++
	if pos > max {
		// We're at the end, wrap around.
		return 0
	} else {
		// Next position.
		return pos
	}
}

// addConn adds a connection's outbound queue
// to the global connection pool lists.
func (p *Pool) addConn(addr string) {
	p.Lock()
	defer p.Unlock()

	p.Conns[addr] = make(chan *string, options.queuecap)
	p.ConnsList = append(p.ConnsList, p.Conns[addr])
}

// removeConn removes a connection's outbound queue
// from the global connection pool lists.
// Additionally, it will redistribute any in-flight messages.
func (p *Pool) removeConn(addr string) {
	p.Lock()
	// Grab the queue to redistribute any message it's holding.
	q := p.Conns[addr]
	// Remove.
	delete(p.Conns, addr)
	p.ConnsList = []chan *string{}
	for _, conn := range p.Conns {
		p.ConnsList = append(p.ConnsList, conn)
	}
	p.Unlock()

	// If the queue had any in-flight messages, redistribute them.
	close(q)
	if len(q) > 0 {
		log.Printf("Redistributing in-flight messages for %s", addr)
		for m := range q {
			failed := []*string{m}
			failedMessages <- failed
		}
	}
}

// faildMessageHandler catches any messages loaded
// into the failedMessage queue and retries distribution.
func failedMessageHandler() {
	for messages := range failedMessages {
		distributionMethod[options.distribution](messages)
	}
}

// establishConn manages TCP connections. Upon successful
// connection, establishConn will add the respective outbound
// queue to the global connection pool.	If a previously existing connection is
// being retried but fails for 3 consecutive attempts, it will be removed
// from the global pool. Background attempts will continue and the connection
// will rejoin the pool upon success.
func establishConn(addr string) net.Conn {
	retry := 0
	retryMax := 3

	for {
		_, connectionIsInPool := pool.Conns[addr]
		// Are we retrying a previously established connection that failed?
		if retry > retryMax && connectionIsInPool {
			log.Printf("Exceeded retry count (%d), removing destination %s from connection pool\n", retryMax, addr)
			pool.removeConn(addr)
		}

		// Try a connection every 15s.
		conn, err := net.DialTimeout("tcp", addr, time.Duration(5*time.Second))
		if err != nil {
			log.Printf("Destination error: %s, retrying in 15s\n", err)
			time.Sleep(15 * time.Second)
			// Increment failure count.
			retry++
			continue
		} else {
			// If this connection succeeds and is not in the pool
			if !connectionIsInPool {
				log.Printf("Adding destination to connection pool: %s\n", addr)
				pool.addConn(addr)
			} else {
				// If this connection is still in the pool, we're
				// likely here due to a temporary disconnect.
				log.Printf("Reconnected to destination: %s\n", addr)
			}
			
			return conn
		}

	}

}

// destinationWriter requests a connection.
// It dequeues from the connection outbound buffer
// and writes to the respective destination.
func destinationWriter(addr string) {
	conn := establishConn(addr)
	defer conn.Close()

	for m := range pool.Conns[addr] {
		_, err := fmt.Fprintln(conn, *m)
		if err != nil {
			pool.Conns[addr] <- m
			log.Printf("Destination %s error: %s\n", addr, err)
			conn = establishConn(addr)
		}
	}

}

func outputGraphite(q chan []*string, ready chan bool) {

	go failedMessageHandler()

	destinations := strings.Split(options.destinations, ",")
	for _, addr := range destinations {
		go destinationWriter(addr)
	}

	// In case we want any initialization to block.
	// Lazily give writers a head start before the listener.
	time.Sleep(1 * time.Second)
	ready <- true

	// Pop messages from the incoming queue and distribute.
	for messages := range q {
		distributionMethod[options.distribution](messages)
	}
}
