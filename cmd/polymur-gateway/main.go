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

	"github.com/jamiealquiza/polymur"
	"github.com/jamiealquiza/polymur/keysync"
	"github.com/jamiealquiza/polymur/listener"
	"github.com/jamiealquiza/polymur/output"
	"github.com/jamiealquiza/polymur/pool"
	"github.com/jamiealquiza/polymur/statstracker"
	"github.com/jamiealquiza/runstats"

	"github.com/jamiealquiza/envy"
)

var (
	options struct {
		addr             string
		apiAddr          string
		httpPort         string
		httpsPort        string
		statAddr         string
		incomingQueuecap int
		outgoingQueuecap int
		console          bool
		destinations     string
		metricsFlush     int
		distribution     string
		cert             string
		key              string
		devMode          bool
		keyPrefix        bool
	}

	sig_chan = make(chan os.Signal)
)

func init() {
	flag.StringVar(&options.addr, "listen-addr", "0.0.0.0", "Polymur-gateway listen address")
	flag.StringVar(&options.httpPort, "listen-http-port", "", "Polymur-gateway listen port (http)")
	flag.StringVar(&options.httpsPort, "listen-https-port", "", "Polymur-gateway listen port (https)")
	flag.StringVar(&options.apiAddr, "api-addr", "localhost:2030", "API listen address")
	flag.StringVar(&options.statAddr, "stat-addr", "localhost:2020", "runstats listen address")
	flag.IntVar(&options.outgoingQueuecap, "outgoing-queue-cap", 4096, "In-flight message queue capacity per destination (number of data points)")
	flag.IntVar(&options.incomingQueuecap, "incoming-queue-cap", 32768, "In-flight incoming message queue capacity (number of data point batches [100 points max per batch])")
	flag.BoolVar(&options.console, "console-out", false, "Dump output to console")
	flag.StringVar(&options.destinations, "destinations", "", "Comma-delimited list of ip:port destinations")
	flag.IntVar(&options.metricsFlush, "metrics-flush", 0, "Graphite flush interval for runtime metrics (0 is disabled)")
	flag.StringVar(&options.distribution, "distribution", "broadcast", "Destination distribution methods: broadcast, hash-route")
	flag.StringVar(&options.cert, "cert", "", "TLS Certificate")
	flag.StringVar(&options.key, "key", "", "TLS Key")
	flag.BoolVar(&options.devMode, "dev-mode", false, "Dev mode: disables Consul API key store; uses '123'")
	flag.BoolVar(&options.keyPrefix, "key-prefix", false, "If enabled, prepends all metrics with the origin polymur-proxy API key's name")

	envy.Parse("POLYMUR_GW")
	flag.Parse()
}

// Handles signal events.
func runControl() {
	signal.Notify(sig_chan, syscall.SIGINT)
	<-sig_chan
	log.Printf("Shutting down")
	os.Exit(0)
}

func main() {
	log.Println("::: Polymur-gateway :::")

	ready := make(chan bool, 1)

	incomingQueue := make(chan []*string, options.incomingQueuecap)

	pool := pool.NewPool()

	// Output writer.
	if options.console {
		go output.OutputConsole(incomingQueue)
		ready <- true
	} else {
		go output.TcpWriter(
			pool,
			&output.TcpWriterConfig{
				Destinations:  options.destinations,
				Distribution:  options.distribution,
				IncomingQueue: incomingQueue,
				QueueCap:      options.outgoingQueuecap,
			},
			ready)
	}

	<-ready

	// Stat counters.
	sentCntr := &statstracker.Stats{}
	go statstracker.StatsTracker(pool, sentCntr)

	// API key sync service.
	apiKeys := keysync.NewApiKeys()
	if !options.devMode {
		go keysync.Run(apiKeys)
	} else {
		apiKeys.Keys["123"] = "dev"
		log.Println("Running in dev-mode: API-key set to '123'")
	}

	// HTTP Listener.
	go listener.HttpListener(&listener.HttpListenerConfig{
		Addr:          options.addr,
		HttpPort:      options.httpPort,
		HttpsPort:     options.httpsPort,
		IncomingQueue: incomingQueue,
		Cert:          options.cert,
		KeyPrefix:     options.keyPrefix,
		Key:           options.key,
		Stats:         sentCntr,
		Keys:          apiKeys,
	})

	// API listener.
	go polymur.Api(pool, options.apiAddr)

	// Polymur stats writer.
	if options.metricsFlush > 0 {
		go runstats.WriteGraphiteWithBackendMetrics(pool, incomingQueue, options.incomingQueuecap, options.metricsFlush, sentCntr)
	}

	// Runtime stats listener.
	go runstats.Start(options.statAddr)

	runControl()
}
