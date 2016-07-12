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
package statstracker

import (
	"log"
	"sync"
	"time"

	"github.com/jamiealquiza/polymur/pool"
)

type Stats struct {
	sync.Mutex
	count int64
	rate  float64
}

func (s *Stats) UpdateCount(v int64) {
	s.Lock()
	s.count += v
	s.Unlock()
}

func (s *Stats) GetCount() int64 {
	s.Lock()
	defer s.Unlock()
	return s.count
}

func (s *Stats) UpdateRate(v float64) {
	s.Lock()
	s.rate = v
	s.Unlock()
}

func (s *Stats) GetRate() float64 {
	s.Lock()
	defer s.Unlock()
	return s.rate
}

// Outputs periodic info summary.
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
