// Package listener http.go implements
// an HTTP metrics listener.
package listener

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/jamiealquiza/polymur/auth"
	"github.com/jamiealquiza/polymur/statstracker"
)

// HTTPListenerConfig holds HTTP listener config.
type HTTPListenerConfig struct {
	Addr          string
	HTTPPort      string
	HTTPSPort     string
	IncomingQueue chan []*string
	Cert          string
	Key           string
	KeyPrefix     bool
	Stats         *statstracker.Stats
	Authorizer    auth.Authorizer
}

// HTTPListener accepts connections from a polymur-proxy
// client. Upon a successful /ping client API key validation,
// batches of compressed messages are passed to /ingest handler.
func HTTPListener(config *HTTPListenerConfig) {
	http.HandleFunc("/ingest", func(w http.ResponseWriter, req *http.Request) { ingest(w, req, config) })
	http.HandleFunc("/ping", func(w http.ResponseWriter, req *http.Request) { ping(w, req, config.Authorizer) })

	var httpsPort string
	if config.HTTPSPort != "" {
		httpsPort = config.HTTPSPort
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
	if config.HTTPPort != "" {
		httpPort = config.HTTPPort
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
func ingest(w http.ResponseWriter, req *http.Request, config *HTTPListenerConfig) {

	var client string
	xff := req.Header.Get("x-forwarded-for")
	if xff != "" {
		client = xff
	} else {
		client = req.RemoteAddr
	}

	// Validate key on every batch.
	// May or may not be a good idea.
	keyName, valid := config.Authorizer.Validate(req)
	if !valid {
		log.Printf("[client %s] %s is not a valid key\n",
			client, config.Authorizer.GetValidationKey(req))

		resp := fmt.Sprintf("invalid key")
		req.Close = true
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, resp)

		return
	}

	log.Printf("[client %s] Recieved batch from from %s\n",
		client, keyName)

	read, err := gzip.NewReader(req.Body)
	if err != nil {
		log.Println(err)
	}

	var b bytes.Buffer
	_, err = b.ReadFrom(read)
	if err != nil {
		log.Printf("[client %s] Batch Error: %s\n", client, err.Error())
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, "Batch Malformed\n")
		return
	}

	io.WriteString(w, "Batch Received\n")

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
func ping(w http.ResponseWriter, req *http.Request, a auth.Authorizer) {
	keyName, valid := a.Validate(req)

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
			client, a.GetValidationKey(req))

		resp := fmt.Sprintf("invalid key")
		req.Close = true
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, resp)
	}
}