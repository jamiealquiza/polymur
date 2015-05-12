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
	Connections map[string]chan *string
	RRList      []chan *string
	RRCurrent   int
}

var (
	pool = &Pool{Connections: make(map[string]chan *string)}
)

func (p *Pool) commitRR(pos int) {
	p.Lock()
	defer p.Unlock()
	p.RRCurrent = pos
}

func (p *Pool) nextRR(pos int) int {
	max := len(p.RRList) - 1
	var next int

	if pos == max {
		next = 0
	} else {
		pos++
		next = pos
	}

	return next
}

func broadcast(messages []*string) {
	// For each message in the batch,
	for _, m := range messages {
		// enqueue into each available destination queue.
		for _, q := range pool.Connections {
			select {
			case q <- m:
				continue
			// Skip if it's full.
			default:
				continue
			}
		}
	}
}

func balanceRR(messages []*string) {
	// Fetch current the RR ID.
	i := pool.RRCurrent

	for _, m := range messages {
		// Needs logic to retry next.
		select {
		case pool.RRList[i] <- m:
			i = pool.nextRR(i)
		default:
			i = pool.nextRR(i)
		}
	}

	// Commit the next RR ID to continue with.
	pool.commitRR(i)
}

func establishConn(addr string) net.Conn {
	var conn net.Conn
	var err error

	for {
		conn, err = net.DialTimeout("tcp", addr, time.Duration(5*time.Second))
		if err != nil {
			log.Printf("Destination error: %s, retrying in 15s\n", err)
			time.Sleep(15 * time.Second)
			continue
		} else {
			log.Printf("Connected to destination: %s\n", addr)
			break
		}
	}

	return conn
}

func destinationWriter(addr string, q <-chan *string) {
	conn := establishConn(addr)
	defer conn.Close()

	for m := range q {
	retry:
		_, err := fmt.Fprintln(conn, *m)
		if err != nil {
			log.Printf("Destination %s error: %s\n", addr, err)
			log.Printf("Attempting to establish new connection to %s\n", addr)
			conn = establishConn(addr)
			goto retry
		}
	}

}

func outputGraphite(q <-chan []*string, cap int, ready chan bool) {

	destinations := strings.Split(options.destinations, ",")

	for _, d := range destinations {
		pool.Lock()
		pool.Connections[d] = make(chan *string, cap)
		pool.RRList = append(pool.RRList, pool.Connections[d])
		pool.Unlock()
	}

	for addr, queue := range pool.Connections {
		go destinationWriter(addr, queue)
	}

	// In case we want any initialization to block.
	// Lazily give writers a head start before the listener.
	time.Sleep(1 * time.Second)
	ready <- true

	for messages := range q {
		balanceRR(messages)
	}
}
