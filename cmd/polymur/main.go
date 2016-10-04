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
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/chrissnell/polymur"
	"github.com/chrissnell/polymur/listener"
	"github.com/chrissnell/polymur/output"
	"github.com/chrissnell/polymur/pool"
	"github.com/chrissnell/polymur/statstracker"
	"github.com/chrissnell/polymur/runstats"
)

var (
	options struct {
		addr         string
		apiAddr      string
		statAddr     string
		queuecap     int
		console      bool
		destinations string
		metricsFlush int
		distribution string
	}

	sigChan = make(chan os.Signal)
)

func init() {
	flag.StringVar(&options.addr, "listen-addr", "0.0.0.0:2003", "Polymur listen address")
	flag.StringVar(&options.apiAddr, "api-addr", "localhost:2030", "API listen address")
	flag.StringVar(&options.statAddr, "stat-addr", "localhost:2020", "runstats listen address")
	flag.IntVar(&options.queuecap, "queue-cap", 4096, "In-flight message queue capacity per destination")
	flag.BoolVar(&options.console, "console-out", false, "Dump output to console")
	flag.StringVar(&options.destinations, "destinations", "", "Comma-delimited list of ip:port destinations")
	flag.IntVar(&options.metricsFlush, "metrics-flush", 0, "Graphite flush interval for runtime metrics (0 is disabled)")
	flag.StringVar(&options.distribution, "distribution", "broadcast", "Destination distribution methods: broadcast, hash-route")
	flag.Parse()
}

// Handles signal events.
func runControl() {
	signal.Notify(sigChan, syscall.SIGINT)
	<-sigChan
	log.Printf("Shutting down")
	os.Exit(0)
}

func main() {
	log.Println("::: Polymur :::")
	ready := make(chan bool, 1)

	incomingQueue := make(chan []*string, 32768)

	pool := pool.NewPool()

	// Output writer.
	if options.console {
		go output.Console(incomingQueue)
		ready <- true
	} else {
		go output.TCPWriter(
			pool,
			&output.TCPWriterConfig{
				Destinations:  options.destinations,
				Distribution:  options.distribution,
				IncomingQueue: incomingQueue,
				QueueCap:      options.queuecap,
			},
			ready)
	}

	<-ready

	// Stat counters.
	sentCntr := &statstracker.Stats{}
	go statstracker.StatsTracker(pool, sentCntr)

	// TCP Listener.
	go listener.TCPListener(&listener.TCPListenerConfig{
		Addr:          options.addr,
		IncomingQueue: incomingQueue,
		FlushTimeout:  5,
		FlushSize:     100,
		Stats:         sentCntr,
	})

	// API listener.
	go polymur.Api(pool, options.apiAddr)

	// Polymur stats writer.
	if options.metricsFlush > 0 {
		go runstats.WriteGraphite(incomingQueue, options.metricsFlush, sentCntr)
	}

	// Runtime stats listener.
	go runstats.Start(options.statAddr)

	runControl()
}
