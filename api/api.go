// Package API implements Polymur API
// methods.
package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/jamiealquiza/polymur/output"
	"github.com/jamiealquiza/polymur/pool"
)

// Available API commands.
var (
	commands = map[string]func(r Request) string{
		"getdest": getdest,
		"putdest": putdest,
		"deldest": deldest,
	}
)

// Request holds API request parameters.
type Request struct {
	pool    *pool.Pool
	command string
	param   string
}

// getdest returns registered destinations from the pool.
func getdest(r Request) string {
	dests := make(map[string]interface{})
	// Get all registered destinations.
	dests["registered"] = r.pool.Registered

	// Get all active.
	active := []string{}
	for k := range r.pool.Conns {
		active = append(active, k)
	}
	dests["active"] = active

	// Json.
	response, _ := json.MarshalIndent(dests, "", " ")
	return fmt.Sprintf("%s\n", response)
}

// putdest registers a destination with the pool.
func putdest(r Request) string {
	if r.param == "" {
		return fmt.Sprintf("Must provide destination\n")
	}

	dest, err := pool.ParseDestination(r.param)
	if err != nil {
		return fmt.Sprintln(err)
	}

	// TODO replace this func with an
	// add destination method on the pool.
	go output.DestinationWriter(r.pool, dest)

	return fmt.Sprintf("Registered destination: %s\n", r.param)
}

// deldest unregisters a destination with the pool.
func deldest(r Request) string {
	if r.param == "" {
		return fmt.Sprintf("Must provide destination\n")
	}

	dest, err := pool.ParseDestination(r.param)
	if err != nil {
		return fmt.Sprintln(err)
	}

	r.pool.Unregister(dest)

	return fmt.Sprintf("Unregistered destination: %s\n", r.param)
}

// API is a simple TCP listener that
// listens for requests.
func API(p *pool.Pool, address string) {
	log.Printf("API started: %s\n", address)

	server, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Listener error: %s\n", err)
	}
	defer server.Close()

	for {
		conn, err := server.Accept()
		if err != nil {
			log.Printf("API error: %s\n", err)
			continue
		}
		apiHandler(p, conn)
	}
}

// apiHandler reads and handles API requests.
func apiHandler(p *pool.Pool, conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	buf, err := reader.ReadBytes('\n')
	if err != nil {
		log.Printf("API error: %s\n", err)
	}

	input := strings.Fields(string(buf[:len(buf)-1]))
	request := Request{command: input[0]}
	if len(input) > 1 {
		request.param = input[1]
	}

	request.pool = p

	if command, valid := commands[request.command]; valid {
		response := command(request)
		conn.Write([]byte(response))
	} else {
		m := fmt.Sprintf("Not a command: %s\n", request.command)
		conn.Write([]byte(m))
	}
}
