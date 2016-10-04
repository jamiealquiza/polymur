// Package output handles sending stats to a polymur-gateway
//
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
)

// HTTPWriterConfig holds configuration for the HTTP Writer
type HTTPWriterConfig struct {
	CACert                string
	ClientCert            string
	ClientKey             string
	UseCertAuthentication bool
	APIKey                string
	Gateway               string
	IncomingQueue         chan []*string
	Workers               int
	client                *http.Client
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
	var roots *x509.CertPool

	if config.ClientCert != "" {
		// Load our TLS key pair to use for authentication
		clientCert, err := tls.LoadX509KeyPair(config.ClientCert, config.ClientKey)
		if err != nil {
			log.Fatalln("Unable to load client cert:", err)
		}

		if config.CACert != "" {
			caCert, err := ioutil.ReadFile(config.CACert)
			if err != nil {
				log.Fatal(err)
				return
			}

			// Add the CA cert into a pool
			roots = x509.NewCertPool()
			ok := roots.AppendCertsFromPEM(caCert)
			if !ok {
				log.Fatal("Error parsing CA certificate")
			}

		}

		// Lock our
		tlsConf := &tls.Config{
			RootCAs:      roots,
			Certificates: []tls.Certificate{clientCert},
		}

		tr := &http.Transport{TLSClientConfig: tlsConf}
		config.client = &http.Client{Transport: tr}
	} else {
		config.client = &http.Client{}
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

	for m := range config.IncomingQueue {
		data, count := packDataPoints(m)

		log.Printf("[worker #%d] sending batch (%d data points)\n",
			workerID,
			count)

		response, err := apiPost(config, "/ingest", data)
		if err != nil {
			// TODO need failure / retry logic.
			log.Printf("[worker #%d] [gateway]: %s", workerID, err)
			continue
		}

		log.Printf("[worker #%d] [gateway] %s", workerID, response.String)
	}
}

// apiPost is a convenience wrapper for submitting requests to
// a polymur-gateway and returning GwResp's.
func apiPost(config *HTTPWriterConfig, path string, postData io.Reader) (*GwResp, error) {
	req, err := http.NewRequest("POST", config.Gateway+path, postData)
	if err != nil {
		return nil, err
	}

	if !config.UseCertAuthentication {
		req.Header.Add("X-polymur-key", config.APIKey)
	}

	resp, err := config.client.Do(req)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	resp.Body.Close()

	return &GwResp{String: string(data), Code: resp.StatusCode}, nil
}

// packDataPoints takes a []*string batch of data points,
// compresses them and returns the reader.
func packDataPoints(d []*string) (io.Reader, int) {
	var count int

	var compressed bytes.Buffer
	w := gzip.NewWriter(&compressed)

	for _, s := range d {
		if s == nil {
			break
		}
		w.Write([]byte(*s))
		w.Write([]byte{10})
		count++
	}

	w.Close()

	return &compressed, count
}
