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

	"github.com/jamiealquiza/polymur/runstats"
)

var (
	messageIncomingQueue = make(chan []*string, options.queuecap)

	options struct {
		addr         string
		port         string
		apiAddr      string
		apiPort      string
		statAddr     string
		statPort     string
		queuecap     int
		console      bool
		destinations string
		metricsFlush int
		distribution string
	}

	// Internals, may be updated dynamically by the app.
	config struct {
		batchSize    int
		flushTimeout int
	}

	sig_chan = make(chan os.Signal)
)

func init() {
	flag.StringVar(&options.addr, "listen-addr", "0.0.0.0", "Polymur listen address")
	flag.StringVar(&options.port, "listen-port", "2003", "Polymur listen port")
	flag.StringVar(&options.apiAddr, "api-addr", "localhost", "API listen address")
	flag.StringVar(&options.apiPort, "api-port", "2030", "API listen port")
	flag.StringVar(&options.statAddr, "stat-addr", "localhost", "runstats listen address")
	flag.StringVar(&options.statPort, "stat-port", "2020", "runstats listen port")
	flag.IntVar(&options.queuecap, "queue-cap", 4096, "In-flight message queue capacity per destination")
	flag.BoolVar(&options.console, "console-out", false, "Dump output to console")
	flag.StringVar(&options.destinations, "destinations", "", "Comma-delimited list of ip:port destinations")
	flag.IntVar(&options.metricsFlush, "metrics-flush", 0, "Graphite flush interval for runtime metrics (0 is disabled)")
	flag.StringVar(&options.distribution, "distribution", "broadcast", "Destination distribution methods: broadcast, hash-route")
	flag.Parse()

	messageIncomingQueue = make(chan []*string, 512)
}

// Handles signal events.
func runControl() {
	signal.Notify(sig_chan, syscall.SIGINT)
	<-sig_chan
	log.Printf("Shutting down")
	os.Exit(0)
}

func main() {
	ready := make(chan bool, 1)

	if options.console {
		go outputConsole(messageIncomingQueue)
		ready <- true
	} else {
		go outputHandler(messageIncomingQueue, ready)
	}

	<-ready

	sentCnt := NewStatser()
	go statsTracker(sentCnt)
	go listener(sentCnt)
	go api(options.apiAddr, options.apiPort)
	if options.metricsFlush > 0 {
		go runstats.WriteGraphite(messageIncomingQueue, options.metricsFlush)
	}
	go runstats.Start(options.statAddr, options.statPort, nil)

	runControl()
}
