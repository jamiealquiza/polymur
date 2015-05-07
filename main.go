package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

var (
	messageIncomingQueue = make(chan []string, options.queuecap)

	options struct {
		addr         string
		port         string
		queuecap     int
		console      bool
		destinations string
	}

	// Internals, may be updated dynamically by the app.
	config struct {
		batchSize    int
		flushTimeout int
	}

	sig_chan = make(chan os.Signal)
)

func init() {
	flag.StringVar(&options.addr, "listen-addr", "localhost", "bind address")
	flag.StringVar(&options.port, "listen-port", "2005", "bind port")
	flag.IntVar(&options.queuecap, "queue-cap", 1000, "In-flight message queue capacity")
	flag.BoolVar(&options.console, "console-out", false, "Dump output to console")
	flag.StringVar(&options.destinations, "destinations", "", "Comma-delimited list of ip:port destinations")
	flag.Parse()

	if options.destinations == "" {
		log.Fatal("Destinations must be set")
	}

	messageIncomingQueue = make(chan []string, options.queuecap)

	config.batchSize = 30
	config.flushTimeout = 5

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
	ready := make(chan bool)

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
	runControl()
}
