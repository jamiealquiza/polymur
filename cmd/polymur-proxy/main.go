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

	"github.com/chrissnell/polymur/listener"
	"github.com/chrissnell/polymur/output"
	"github.com/chrissnell/polymur/statstracker"
	"github.com/chrissnell/polymur/runstats"
)

var (
	options struct {
		clientCert            string
		clientKey             string
		CACert                string
		useCertAuthentication bool
		APIKey                string
		gateway               string
		addr                  string
		statAddr              string
		queuecap              int
		workers               int
		console               bool
		metricsFlush          int
	}

	sigChan = make(chan os.Signal)
)

func init() {
	flag.StringVar(&options.clientCert, "client-cert", "", "Client TLS Certificate")
	flag.StringVar(&options.clientKey, "client-key", "", "Client TLS Private Key")
	flag.StringVar(&options.CACert, "ca-cert", "", "CA Root Certificate - if server is using a cert that wasn't signed by a root CA that we recognize automatically")
	flag.BoolVar(&options.useCertAuthentication, "use-cert-auth", false, "Use TLS certificate-based authentication in lieu of API keys")
	flag.StringVar(&options.APIKey, "api-key", "", "polymur gateway API key")
	flag.StringVar(&options.gateway, "gateway", "", "polymur gateway address")
	flag.StringVar(&options.addr, "listen-addr", "0.0.0.0:2003", "Polymur-proxy listen address")
	flag.StringVar(&options.statAddr, "stat-addr", "localhost:2020", "runstats listen address")
	flag.IntVar(&options.queuecap, "queue-cap", 32768, "In-flight message queue capacity")
	flag.IntVar(&options.workers, "workers", 3, "HTTP output workers")
	flag.BoolVar(&options.console, "console-out", false, "Dump output to console")
	flag.IntVar(&options.metricsFlush, "metrics-flush", 0, "Graphite flush interval for runtime metrics (0 is disabled)")
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
	log.Println("::: Polymur-proxy :::")
	ready := make(chan bool, 1)

	incomingQueue := make(chan []*string, options.queuecap)

	// If we're going to use certificate auth to talk to the server, we have to be configured with
	// a client certificate and key pair.
	if options.useCertAuthentication && (options.clientCert == "" || options.clientKey == "") {
		log.Fatalln("Cannot use certificate-based authentication without supplying a cert via -cert")
	}

	// Output writer.
	if options.console {
		go output.Console(incomingQueue)
		ready <- true
	} else {
		go output.HTTPWriter(
			&output.HTTPWriterConfig{
				ClientCert:            options.clientCert,
				ClientKey:             options.clientKey,
				CACert:                options.CACert,
				UseCertAuthentication: options.useCertAuthentication,
				APIKey:                options.APIKey,
				Gateway:               options.gateway,
				Workers:               options.workers,
				IncomingQueue:         incomingQueue,
			},
			ready)
	}

	<-ready

	// Stat counters.
	sentCntr := &statstracker.Stats{}
	go statstracker.StatsTracker(nil, sentCntr)

	// TCP Listener.
	go listener.TCPListener(&listener.TCPListenerConfig{
		Addr:          options.addr,
		IncomingQueue: incomingQueue,
		FlushTimeout:  15,
		FlushSize:     5000,
		Stats:         sentCntr,
	})

	// Polymur stats writer.
	if options.metricsFlush > 0 {
		go runstats.WriteGraphite(incomingQueue, options.metricsFlush, sentCntr)
	}

	// Runtime stats listener.
	go runstats.Start(options.statAddr)

	runControl()
}
