package main

import (
	"log"
	"time"
)

type Statser struct {
	value chan int64
}

func NewStatser() *Statser {
	s := &Statser{make(chan int64, 1)}
	s.init()
	return s
}

func (s *Statser) init() {
	s.value <- 0
}

func (s *Statser) IncrRecv(v int64) {
	i := <-s.value
	s.value <- i + v
}

func (s *Statser) FetchRecv() int64 {
	i := <-s.value
	s.value <- i
	return i
}

// Outputs periodic info summary.
func statsTracker(s *Statser) {
	tick := time.Tick(5 * time.Second)
	lastInterval := time.Now()
	var currCnt, lastCnt int64

	for {
		<-tick
		sinceLastInterval := float64(time.Since(lastInterval).Seconds())
		lastInterval = time.Now()

		// Inbound rates.
		lastCnt = currCnt
		currCnt = s.FetchRecv()
		deltaCnt := currCnt - lastCnt
		if deltaCnt > 0 {
			log.Printf("Last %.2fs: Received %d data points | Avg: %.2f/sec. | Inbound queue length: %d\n",
				sinceLastInterval,
				deltaCnt,
				float64(deltaCnt)/sinceLastInterval,
				len(messageIncomingQueue))
		}

		// Outbound queues.
		pool.Lock()
		for dest, outboundQueue := range pool.Conns {
			currLen := len(outboundQueue)
			switch {
			case currLen == options.queuecap:
				log.Printf("Destination %s queue is at capacity (%d) - further messages will be dropped", dest, currLen)
			case currLen > 0:
				log.Printf("Destination %s queue length: %d\n", dest, currLen)
			}
		}
		pool.Unlock()

		// Misc. internal queues.
		if l := len(retryQueue); l > 0 {
			log.Printf("Retry message queue length: %d\n", l)
		}

	}
}
