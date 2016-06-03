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
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
)

var (
	commands = map[string]func(r Request) string{
		"getdest": getdest,
		"putdest": putdest,
		"deldest": deldest,
	}
)

type Request struct {
	command string
	param   string
}

func getdest(r Request) string {
	dests := make(map[string]interface{})
	// Get all registered destinations.
	dests["registered"] = pool.Registered

	// Get all active.
	active := []string{}
	for k, _ := range pool.Conns {
		active = append(active, k)
	}
	dests["active"] = active

	// Json.
	response, _ := json.MarshalIndent(dests, "", " ")
	return fmt.Sprintf("%s\n", response)
}

func putdest(r Request) string {
	if r.param == "" {
		return fmt.Sprintf("Must provide destination\n")
	}

	dest, err := parseDestination(r.param)
	if err != nil {
		return fmt.Sprintln(err)
	}

	go destinationWriter(dest)

	return fmt.Sprintf("Registered destination: %s\n", r.param)
}

func deldest(r Request) string {
	if r.param == "" {
		return fmt.Sprintf("Must provide destination\n")
	}

	dest, err := parseDestination(r.param)
	if err != nil {
		return fmt.Sprintln(err)
	}

	pool.unregister(dest)

	return fmt.Sprintf("Unregistered destination: %s\n", r.param)
}

func api(address, port string) {
	log.Printf("API started: %s:%s\n",
		address,
		port)

	server, err := net.Listen("tcp", address+":"+port)
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
		apiHandler(conn)
	}
}

func apiHandler(conn net.Conn) {
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

	if command, valid := commands[request.command]; valid {
		response := command(request)
		conn.Write([]byte(response))
	} else {
		m := fmt.Sprintf("Not a command: %s\n", request.command)
		conn.Write([]byte(m))
	}
}
