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
package runstats

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/jamiealquiza/polymur/pool"
)

var (
	startTime = time.Now()
)

type Statser interface {
	GetRate() float64
}

func WriteGraphite(c chan []*string, i int, s Statser) {
	interval := time.Tick(time.Duration(i) * time.Second)
	hostname, _ := os.Hostname()
	for {
		<-interval
		now := time.Now()
		ts := int64(now.Unix())
		metrics := []*string{}
		stats := buildStats()

		for k, v := range stats["runtime-meminfo"] {
			value := fmt.Sprintf("%s.polymur.runtime.%s %d %d", hostname, k, v, ts)
			metrics = append(metrics, &value)
		}

		rate := fmt.Sprintf("%s.polymur.rate %.2f %d", hostname, s.GetRate(), ts)
		metrics = append(metrics, &rate)

		// Drop the metrics into Polymur's
		// incoming channel.
		c <- metrics
	}
}

// WriteGraphiteWithBackendMetrics takes a pointer to backend pool, incoming queue, incoming queue limit and a statser interface.
func WriteGraphiteWithBackendMetrics(p *pool.Pool, c chan []*string, ic int, i int, s Statser) {
	interval := time.Tick(time.Duration(i) * time.Second)
	hostname, _ := os.Hostname()
	for {
		<-interval
		now := time.Now()
		ts := int64(now.Unix())
		metrics := []*string{}
		stats := buildStats()

		for k, v := range stats["runtime-meminfo"] {
			value := fmt.Sprintf("%s.polymur.runtime.%s %d %d", hostname, k, v, ts)
			metrics = append(metrics, &value)
		}

		rate := fmt.Sprintf("%s.polymur.rate %.2f %d", hostname, s.GetRate(), ts)
		metrics = append(metrics, &rate)

		incomingQueue := fmt.Sprintf("%s.polymur.incoming-queue.current-size %d %d", hostname, len(c), ts)
		incomingQueueCap := fmt.Sprintf("%s.polymur.incoming-queue.limit %d %d", hostname, ic, ts)
		metrics = append(metrics, &incomingQueue, &incomingQueueCap)

		p.Lock()
		for dest, destQueue := range p.Conns {
			destQueueSize := fmt.Sprintf("%s.polymur.outgoing-queue.%s.current-size %d %d", hostname, strings.Replace(dest, ".", "_", -1), len(destQueue), ts)
			destQueueLimit := fmt.Sprintf("%s.polymur.outgoing-queue.%s.limit %d %d", hostname, strings.Replace(dest, ".", "_", -1), p.QueueCap, ts)
			metrics = append(metrics, &destQueueSize, &destQueueLimit)
		}
		p.Unlock()
		// Drop the metrics into Polymur's
		// incoming channel.
		c <- metrics
	}
}

func Start(address string) {
	log.Printf("Runstats started: %s\n", address)

	server, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Runstats error: %s\n", err)
	}
	defer server.Close()

	for {
		conn, err := server.Accept()
		if err != nil {
			log.Printf("Runstats listener error: %s\n", err)
			continue
		}
		reqHandler(conn)
	}
}

func reqHandler(conn net.Conn) {
	defer conn.Close()
	reqBuf := make([]byte, 8)
	mlen, err := conn.Read(reqBuf)
	if err != nil && err != io.EOF {
		fmt.Println(err.Error())
	}

	req := strings.TrimSpace(string(reqBuf[:mlen]))
	switch req {
	case "stats":
		r := buildStats()

		response, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			log.Printf("Error parsing: %s", err)
		}
		// Append LF.
		response = append(response, 10)

		conn.Write(response)
	default:
		m := fmt.Sprintf("Not a command: %s\n", req)
		conn.Write([]byte(m))
	}
}

// Generate stats response.
func buildStats() map[string]map[string]interface{} {

	// Object that will carry all response info.
	stats := make(map[string]map[string]interface{})

	// Append default service info.
	stats["service"] = make(map[string]interface{})
	stats["service"]["start-time"] = startTime.Format(time.RFC3339)
	uptime := int64(time.Since(startTime).Seconds())
	stats["service"]["uptime-seconds"] = uptime

	// Get current MemStats.
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	// We swipe the stats we want.
	// Reference: http://golang.org/pkg/runtime/#ReadMemStats
	stats["runtime-meminfo"] = make(map[string]interface{})
	stats["runtime-meminfo"]["Alloc"] = mem.Alloc
	stats["runtime-meminfo"]["TotalAlloc"] = mem.TotalAlloc
	stats["runtime-meminfo"]["Sys"] = mem.Sys
	stats["runtime-meminfo"]["Lookups"] = mem.Lookups
	stats["runtime-meminfo"]["Mallocs"] = mem.Mallocs
	stats["runtime-meminfo"]["Frees"] = mem.Frees
	stats["runtime-meminfo"]["HeapAlloc"] = mem.HeapAlloc
	stats["runtime-meminfo"]["HeapSys"] = mem.HeapSys
	stats["runtime-meminfo"]["HeapIdle"] = mem.HeapIdle
	stats["runtime-meminfo"]["HeapInuse"] = mem.HeapInuse
	stats["runtime-meminfo"]["HeapReleased"] = mem.HeapReleased
	stats["runtime-meminfo"]["HeapObjects"] = mem.HeapObjects
	stats["runtime-meminfo"]["StackInuse"] = mem.StackInuse
	stats["runtime-meminfo"]["StackSys"] = mem.StackSys
	stats["runtime-meminfo"]["MSpanInuse"] = mem.MSpanInuse
	stats["runtime-meminfo"]["MSpanSys"] = mem.MSpanSys
	stats["runtime-meminfo"]["MCacheInuse"] = mem.MCacheInuse
	stats["runtime-meminfo"]["MCacheSys"] = mem.MCacheSys
	stats["runtime-meminfo"]["BuckHashSys"] = mem.BuckHashSys
	stats["runtime-meminfo"]["GCSys"] = mem.GCSys
	stats["runtime-meminfo"]["OtherSys"] = mem.OtherSys
	stats["runtime-meminfo"]["NextGC"] = mem.NextGC
	stats["runtime-meminfo"]["LastGC"] = mem.LastGC
	stats["runtime-meminfo"]["PauseTotalNs"] = mem.PauseTotalNs
	stats["runtime-meminfo"]["NumGC"] = mem.NumGC

	return stats
}
