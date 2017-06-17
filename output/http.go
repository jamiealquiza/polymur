// Package output http.go writes
// datapoints to an HTTP destination.
package output

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

// HTTPWriterConfig holds HTTP output
// configuration.
type HTTPWriterConfig struct {
	Cert          string
	APIKey        string
	Gateway       string
	IncomingQueue chan []*string
	Workers       int
	HttpTimeout   int
	client        *http.Client
	Verbose       bool
}

// GwResp captures the response string
// and numeric code from a polymur-gateway.
type GwResp struct {
	String string
	Code   int
}

// HTTPWriter writes compressesed message batches over HTTPS
// to a polymur-gateway instance. Initial connection is OK'd
// by hitting the /ping path with a valid client API key registered
// with the polymur-gateway.
func HTTPWriter(config *HTTPWriterConfig, ready chan bool) {

	if config.Cert != "" {
		cert, err := ioutil.ReadFile(config.Cert)
		if err != nil {
			log.Fatal(err)
			return
		}

		// Use client cert.
		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(cert)
		if !ok {
			log.Fatal("Error parsing certificate")
		}

		tlsConf := &tls.Config{RootCAs: roots}
		tr := &http.Transport{TLSClientConfig: tlsConf}
		config.client = &http.Client{
			Transport: tr,
			Timeout:   time.Duration(config.HttpTimeout) * time.Second,
		}
	} else {
		config.client = &http.Client{Timeout: time.Duration(config.HttpTimeout) * time.Second}
	}

	// Try connection, verify api key.
	log.Printf("Pinging gateway %s\n", config.Gateway)
	response, err := apiPost(config, "/ping", nil)
	if err != nil {
		log.Fatal(err)
	}

	// Check if not 200 and exit.
	if response.Code != 200 {
		log.Fatalf("[gateway]: %s\n", response.String)
	} else {
		log.Printf("Connection to gateway %s successful\n", config.Gateway)
	}

	ready <- true

	// Start up writers.
	for i := 0; i < config.Workers; i++ {
		go writeStream(config, i)
	}
}

// writeStream reads data point batches from the IncomingQueue,
// compresses and writes to the downstream polymur-gateway.
func writeStream(config *HTTPWriterConfig, workerID int) {
	log.Printf("HTTP writer #%d started\n", workerID)

	var data bytes.Buffer
	w := gzip.NewWriter(&data)
	var count int

	for m := range config.IncomingQueue {
		count = packDataPoints(w, m)

		if config.Verbose {
			log.Printf("[worker #%d] sending batch (%d data points)\n",
				workerID,
				count)
		}

		start := time.Now()
		response, err := apiPost(config, "/ingest", &data)
		w.Reset(&data)

		if err != nil {
			// TODO need failure / retry logic.
			log.Printf("[worker #%d] gateway]: %s",
				workerID, err)
			count = 0
			continue
		}

		// If it's a non-200, log.
		if response.Code != 200 {
			log.Printf("[worker #%d] %s [gateway] %s",
				workerID, time.Since(start), response.String)
		} else {
			// If it's a 200 but verbosity is true,
			// log.
			if config.Verbose {
				log.Printf("[worker #%d] %s [gateway] %s",
					workerID, time.Since(start), response.String)
			}
		}

		count = 0
	}
}

// apiPost is a convenience wrapper for submitting requests to
// a polymur-gateway and returning GwResp's.
func apiPost(config *HTTPWriterConfig, path string, postData io.Reader) (*GwResp, error) {
	req, err := http.NewRequest("POST", config.Gateway+path, postData)
	if err != nil {
		return nil, err
	}

	req.Header.Add("X-polymur-key", config.APIKey)
	resp, err := config.client.Do(req)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &GwResp{String: string(data), Code: resp.StatusCode}, nil
}

// packDataPoints takes a []*string batch of data points,
// compresses them and returns the reader.
func packDataPoints(w *gzip.Writer, d []*string) int {
	var count int
	for _, s := range d {
		if s == nil {
			break
		}
		w.Write([]byte(*s))
		w.Write([]byte{10})
		count++
	}

	w.Close()

	return count
}
