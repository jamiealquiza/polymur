// Package statstracker facilitates
// statistics collection and reporting.
package statstracker

import (
	"log"
	"sync"
	"time"

	"github.com/jamiealquiza/polymur/pool"
)

// Stats holds stats data.
type Stats struct {
	sync.Mutex
	count int64
	rate  float64
}

// UpdateCount updates a counter.
func (s *Stats) UpdateCount(v int64) {
	s.Lock()
	s.count += v
	s.Unlock()
}

// Getcount gets a counter value.
func (s *Stats) GetCount() int64 {
	s.Lock()
	defer s.Unlock()
	return s.count
}

// UpdateRate updates a rate.
func (s *Stats) UpdateRate(v float64) {
	s.Lock()
	s.rate = v
	s.Unlock()
}

// GetRate gets a rate value.
func (s *Stats) GetRate() float64 {
	s.Lock()
	defer s.Unlock()
	return s.rate
}

// StatsTracker outputs periodic info summary.
func StatsTracker(pool *pool.Pool, s *Stats) {
	tick := time.Tick(5 * time.Second)
	lastInterval := time.Now()
	var currCnt, lastCnt int64

	for {
		<-tick
		sinceLastInterval := float64(time.Since(lastInterval).Seconds())
		lastInterval = time.Now()

		// Inbound rates.
		lastCnt = currCnt
		currCnt = s.GetCount()
		deltaCnt := currCnt - lastCnt
		if deltaCnt > 0 {
			s.UpdateRate(float64(deltaCnt) / sinceLastInterval)
			log.Printf("Last %.2fs: Received %d data points | Avg: %.2f/sec.\n",
				sinceLastInterval,
				deltaCnt,
				s.GetRate())
		} else {
			s.UpdateRate(0)
		}

		if pool == nil {
			continue
		}
		// Outbound queues.
		pool.Lock()
		for dest, outboundQueue := range pool.Conns {
			currLen := len(outboundQueue)
			switch {
			case currLen == pool.QueueCap:
				log.Printf("Destination %s queue is at capacity (%d) - further messages will be dropped", dest, currLen)
			case currLen > 0:
				log.Printf("Destination %s queue length: %d\n", dest, currLen)
			}
		}
		pool.Unlock()

		// Misc. internal queues.
		if l := len(pool.RetryQueue); l > 0 {
			log.Printf("Retry message queue length: %d\n", l)
		}

	}
}
