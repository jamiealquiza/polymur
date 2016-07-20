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
package listener

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
)

type HttpListenerConfig struct {
	Addr          string
	IncomingQueue chan []*string
	Cert          string
	Key           string
}

// HttpListener accepts connections from a polymur-proxy
// client. Upon a successful /ping client API key validation,
// batches of compressed messages are passed to /ingest handler.
func HttpListener(config *HttpListenerConfig) {
	http.HandleFunc("/ingest", func(w http.ResponseWriter, req *http.Request) { ingest(w, req, config.IncomingQueue) })
	http.HandleFunc("/ping", ping)

	err := http.ListenAndServeTLS(":443", config.Cert, config.Key, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

// ingest is a handler that accepts a batch of compressed data points.
// Data points arive as a concatenated string with newline delimition.
// Each batch is broken up and populated into a []*string and pushed
// to the IncomingQueue for downstream destination writing.
func ingest(w http.ResponseWriter, req *http.Request, q chan []*string) {
	io.WriteString(w, "Batch Received\n")
	log.Printf("Recieved batch from from %s\n", req.Header["X-Polymur-Key"][0])

	read, err := gzip.NewReader(req.Body)
	if err != nil {
		log.Println(err)
	}

	var b bytes.Buffer
	b.ReadFrom(read)
	req.Body.Close()

	batch := []*string{}
	// Probably want to just pass a header that
	// specifies how many data points are in the batch
	// so that we can avoid using append().
	for {
		// This should have a timeout so
		// malformed messages without a delim
		// don't hang forever.
		l, err := b.ReadBytes(10)

		if len(l) > 0 {
			m := string(l[:len(l)-1])
			batch = append(batch, &m)
		}
		if err != nil {
			break
		}
	}

	q <- batch
}

// ping validates a connecting polymur-proxy's API key.
func ping(w http.ResponseWriter, req *http.Request) {
	k := req.Header["X-Polymur-Key"][0]
	if validKey(k) {
		log.Printf("Key %s is valid\n", k)
		io.WriteString(w, "valid\n")
	} else {
		resp := fmt.Sprintf("Key %s is invalid", k)
		log.Println(resp)
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, resp)
	}
}

// validKey looks up a keys validity. This is
// obviously a dummy function until a backing store
// is implemented.
func validKey(k string) bool {
	log.Printf("Validating key: %s\n", k)
	if k == "123" {
		return true
	} else {
		return false
	}
}
