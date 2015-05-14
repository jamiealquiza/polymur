package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

type Pool struct {
	sync.Mutex
	Conns      map[string]chan *string
	ConnsList  []chan *string
	RRCurrent  int
	Registered map[string]time.Time
}

var (
	pool = &Pool{
		Conns:      make(map[string]chan *string),
		Registered: make(map[string]time.Time),
	}

	distributionMethod = map[string]func([]*string){
		"broadcast":  broadcast,
		"balance-rr": balanceRR,
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

func balanceRR(messages []*string) {
	// Fetch current the RR ID.
	pos := pool.RRCurrent

	for _, m := range messages {

		// If no destinations are available, we're
		// going to get a negative position val.
		// Load the whole batch into the retry.
		pos = pool.nextRR(pos)

		if pos < 0 {
			log.Println("No destinations available, loading batch into retry queue")
		} else {
			// Attempt current RR node.
			select {
			case pool.ConnsList[pos] <- m:
				continue
			default:
				break
			}
		}

		// If unavailable, load into failed messages for retry.
		failed := []*string{m}
		select {
		case retryQueue <- failed:
			fmt.Println("failed loaded")
		// If retryQueue is full, don't block message distribution.
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

func (p *Pool) register(addr string) {
	p.Lock()
	defer p.Unlock()

	log.Printf("Registered destination %s\n", addr)
	p.Registered[addr] = time.Now()
}

func (p *Pool) unregister(addr string) {
	p.Lock()
	delete(p.Registered, addr)
	p.Unlock()

	log.Printf("Unregistered destination %s\n", addr)
	p.removeConn(addr)
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
	// Check if it exists, first.
	if _, connectionIsInPool := pool.Conns[addr]; !connectionIsInPool {
		return
	}

	log.Printf("Removing destination %s from connection pool\n", addr)

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

	// Don't need to redistribute in-flight for broadcast.
	if options.distribution == "broadcast" {
		return
	}
	// If the queue had any in-flight messages, redistribute them.
	close(q)
	if len(q) > 0 {
		log.Printf("Redistributing in-flight messages for %s", addr)
		for m := range q {
			failed := []*string{m}
			retryQueue <- failed
		}
	}
}

// retryMessageHandler catches any messages loaded
// into the failedMessage queue and retries distribution.
// TODO: needs exponential backoff when using RR and no destinations
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
				time.Sleep(100 * time.Millisecond)
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
func establishConn(addr string) (net.Conn, error) {
	retry := 0
	retryMax := 3

	for {
		// If it's not registered, abort.
		if _, destinationRegistered := pool.Registered[addr]; !destinationRegistered {
			return nil, errors.New("Destination not registered")
		}

		_, connectionIsInPool := pool.Conns[addr]
		// Are we retrying a previously established connection that failed?
		if retry > retryMax && connectionIsInPool {
			log.Printf("Exceeded retry count (%d) for destination %s\n", retryMax, addr)
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

			return conn, nil
		}

	}

}

// destinationWriter requests a connection.
// It dequeues from the connection outbound buffer
// and writes to the respective destination.
func destinationWriter(addr string) {
	pool.register(addr)
	conn, err := establishConn(addr)
	if err != nil {
		// If we have an error here, it likely means
		// that this connection never
		return
	}
	defer conn.Close()

	for m := range pool.Conns[addr] {
		_, err := fmt.Fprintln(conn, *m)
		if err != nil {
			pool.Conns[addr] <- m
			log.Printf("Destination %s error: %s\n", addr, err)

			// Wait on a connection. If the destination isn't
			// registered, err and close this writer.
			newConn, err := establishConn(addr)
			if err != nil {
				break
			} else {
				conn = newConn
			}
		}
	}

}

func outputGraphite(q chan []*string, ready chan bool) {
	go retryMessageHandler()

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
