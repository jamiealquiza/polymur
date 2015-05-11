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
}

var (
	pool = &Pool{Connections: make(map[string]chan *string)}
)

func broadcast(m *string) {
	for _, q := range pool.Connections {
		select {
		case q <- m:
			continue
		default:
			continue
		}
	}
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

func connectionWriter(addr string, q <- chan *string) {
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
		pool.Unlock()
	}

	for addr, queue := range pool.Connections {
		go connectionWriter(addr, queue)
	}

	// In case we want any initialization to block.
	ready <- true

	for messages := range q {
		for _, m := range messages {
			broadcast(m)
		}
	}
}
