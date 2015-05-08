package main

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

type connection struct {
	sync.Mutex
	alive map[int]net.Conn
	id    int
}

var (
	connections = &connection{alive: make(map[int]net.Conn)}
)
func (c *connection) NextId() int {
	c.Lock()
	c.id = c.id + 1
	c.Unlock()
	return c.id
}

func send(m *string) {
	for i, conn := range connections.alive {
		_, err := fmt.Fprintln(conn, *m)
		if err != nil {
			log.Printf("Destination %s error: %s\n", i, err)
		}
	}
}

func outputGraphite(q <-chan []*string, ready chan bool) {

	destinations := strings.Split(options.destinations, ",")

tryConnections:
	for i, d := range destinations {
		conn, err := net.DialTimeout("tcp", d, time.Duration(5*time.Second))
		if err != nil {
			log.Println(err)
		} else {
			log.Printf("Connected to %s\n", d)
			connections.alive[connections.NextId()] = conn
		}

		// If this is the last endpoint to try and we
		// still haven't found any valid connections, sleep & restart.
		if i+1 == len(destinations) && len(connections.alive) < 1 {
			log.Println("No valid connections, retrying in 30s")
			time.Sleep(30 * time.Second)
			goto tryConnections
		}
	}

	ready <- true

	for messages := range q {
		for _, m := range messages {
			send(m)
		}
	}
}
