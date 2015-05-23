package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/jamiealquiza/polymur/runstats"
)

var (
	messageIncomingQueue = make(chan []*string, options.queuecap)

	options struct {
		addr         string
		port         string
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
	flag.StringVar(&options.addr, "listen-addr", "0.0.0.0", "bind address")
	flag.StringVar(&options.port, "listen-port", "2003", "bind port")
	flag.IntVar(&options.queuecap, "queue-cap", 4096, "In-flight message queue capacity to any single destination")
	flag.BoolVar(&options.console, "console-out", false, "Dump output to console")
	flag.StringVar(&options.destinations, "destinations", "", "Comma-delimited list of ip:port destinations")
	flag.IntVar(&options.metricsFlush, "metrics-flush", 0, "Graphite flush interval for runtime metrics (0 is disabled)")
	flag.StringVar(&options.distribution, "distribution", "broadcast", "Destination distribution methods: broadcast, balance-rr, balance-hr")
	flag.Parse()

	messageIncomingQueue = make(chan []*string, 512)

	runtime.GOMAXPROCS(runtime.NumCPU())
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
		go outputGraphite(messageIncomingQueue, ready)
	}

	<-ready

	sentCnt := NewStatser()
	go statsTracker(sentCnt)
	go listener(sentCnt)
	go api("localhost", "2030")
	if options.metricsFlush > 0 {
		go runstats.WriteGraphite(messageIncomingQueue, options.metricsFlush)
	}
	go runstats.Start("localhost", "2020", nil)

	runControl()
}
