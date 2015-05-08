package runstats

import (
	"runtime"
	"encoding/json"
	"net"
	"fmt"
	"log"
	"io"
	"strings"
	"time"
)

var (
	startTime = time.Now()
)

func Start(address, port string, serviceInfo map[string]interface{}) {
	log.Printf("Runstats started: %s:%s\n",
		address,
		port)

	server, err := net.Listen("tcp", address+":"+port)
	if err != nil {
		log.Fatalf("Listener error: %s\n", err)
	}
	defer server.Close()

	for {
		conn, err := server.Accept()
		if err != nil {
			log.Printf("Listener down: %s\n", err)
			continue
		}
		go reqHandler(conn, serviceInfo)
	}
}

func reqHandler(conn net.Conn, serviceInfo map[string]interface{}) {
	defer conn.Close()
	reqBuf := make([]byte, 8)
	mlen, err := conn.Read(reqBuf)
	if err != nil && err != io.EOF {
		fmt.Println(err.Error())
	}

	req := strings.TrimSpace(string(reqBuf[:mlen]))
	switch req {
	case "stats":
		r := buildStats(serviceInfo)
		conn.Write(r)
	default:
		m := fmt.Sprintf("Not a command: %s\n", req)
		conn.Write([]byte(m))
	}	
}

// Generate stats response.
func buildStats(serviceInfo map[string]interface{}) []byte {

	// Object that will carry all response info.
	stats := make(map[string]map[string]interface{})

	// Add supplied service info.
	stats["service"] = make(map[string]interface{})
	if serviceInfo != nil {
		stats["service"] = serviceInfo
	}
	// Append default service info.
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

	response, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		log.Printf("Error parsing: %s", err)
	}
	// Append LF.
	response = append(response, 10)
	return response
}