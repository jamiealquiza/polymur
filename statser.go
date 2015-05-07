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

func (s *Statser) IncrSent(v int64) {
	i := <-s.value
	s.value <- i + v
}

func (s *Statser) FetchSent() int64 {
	i := <-s.value
	s.value <- i
	return i
}

// Outputs periodic info summary.
func statsTracker(s *Statser) {
	tick := time.Tick(5 * time.Second)
	var currCnt, lastCnt int64
	for {
		select {
		case <-tick:
			lastCnt = currCnt
			currCnt = s.FetchSent()
			deltaCnt := currCnt - lastCnt
			if deltaCnt > 0 {
				log.Printf("Last 5s: Received %d messages | Avg: %.2f messages/sec. | Inbound queue length: %d\n",
					deltaCnt,
					float64(deltaCnt)/5,
					len(messageIncomingQueue))
			}
		}
	}
}
