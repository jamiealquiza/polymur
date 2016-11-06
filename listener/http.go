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

	"github.com/jamiealquiza/polymur/keysync"
	"github.com/jamiealquiza/polymur/statstracker"
)

type HttpListenerConfig struct {
	Addr          string
	HttpPort      string
	HttpsPort     string
	IncomingQueue chan []*string
	Cert          string
	Key           string
	KeyPrefix     bool
	Stats         *statstracker.Stats
	Keys          *keysync.ApiKeys
}

// HttpListener accepts connections from a polymur-proxy
// client. Upon a successful /ping client API key validation,
// batches of compressed messages are passed to /ingest handler.
func HttpListener(config *HttpListenerConfig) {
	http.HandleFunc("/ingest", func(w http.ResponseWriter, req *http.Request) { ingest(w, req, config) })
	http.HandleFunc("/ping", func(w http.ResponseWriter, req *http.Request) { ping(w, req, config.Keys) })

	var httpsPort string
	if config.HttpsPort != "" {
		httpsPort = config.HttpsPort
	} else {
		httpsPort = "443"
	}

	if config.Cert != "" && config.Key != "" {
		go func() {
			log.Printf("HTTPS listening on %s:%s\n", config.Addr, httpsPort)
			err := http.ListenAndServeTLS(config.Addr+":"+httpsPort, config.Cert, config.Key, nil)
			if err != nil {
				log.Fatalf("ListenAndServe: %s\n", err)
			}
		}()

	}

	var httpPort string
	if config.HttpPort != "" {
		httpPort = config.HttpPort
	} else {
		httpPort = "80"
	}

	go func() {
		log.Printf("HTTP listening on %s:%s\n", config.Addr, httpPort)
		err := http.ListenAndServe(config.Addr+":"+httpPort, nil)
		if err != nil {
			log.Fatalf("ListenAndServe: %s\n", err)
		}
	}()
}

// ingest is a handler that accepts a batch of compressed data points.
// Data points arive as a concatenated string with newline delimition.
// Each batch is broken up and populated into a []*string and pushed
// to the IncomingQueue for downstream destination writing.
func ingest(w http.ResponseWriter, req *http.Request, config *HttpListenerConfig) {

	// Validate key on every batch.
	// May or may not be a good idea.
	requestKey := req.Header.Get("X-Polymur-Key")

	var client string
	xff := req.Header.Get("x-forwarded-for")
	if xff != "" {
		client = xff
	} else {
		client = req.RemoteAddr
	}

	keyName, valid := validateKey(requestKey, config.Keys)
	if !valid {
		log.Printf("[client %s] %s is not a valid key\n",
			client, requestKey)

		resp := fmt.Sprintf("invalid key")
		req.Close = true
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, resp)

		return
	}

	io.WriteString(w, "Batch Received\n")
	log.Printf("[client %s] Recieved batch from from %s\n",
		client, keyName)

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
			if config.KeyPrefix {
				m = fmt.Sprintf("%s.%s", keyName, m)
			}
			batch = append(batch, &m)
			config.Stats.UpdateCount(1)
		}
		if err != nil {
			break
		}
	}

	config.IncomingQueue <- batch
}

// ping validates a connecting polymur-proxy's API key.
func ping(w http.ResponseWriter, req *http.Request, keys *keysync.ApiKeys) {
	requestKey := req.Header.Get("X-Polymur-Key")
	keyName, valid := validateKey(requestKey, keys)

	var client string
	xff := req.Header.Get("x-forwarded-for")
	if xff != "" {
		client = xff
	} else {
		client = req.RemoteAddr
	}

	if valid {
		log.Printf("[client %s] key for %s is valid\n",
			client, keyName)
		io.WriteString(w, "key is valid\n")
	} else {
		log.Printf("[client %s] %s is not a valid key\n",
			client, requestKey)

		resp := fmt.Sprintf("invalid key")
		req.Close = true
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, resp)
	}
}

// validateKey looks up if a key is registered in Consul
// and returns the key name and key.
func validateKey(k string, keys *keysync.ApiKeys) (string, bool) {
	keys.Lock()
	name, valid := keys.Keys[k]
	keys.Unlock()

	if valid {
		return name, true
	} else {
		return "", false
	}
}
