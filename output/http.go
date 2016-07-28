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

type HttpWriterConfig struct {
	Cert          string
	ApiKey        string
	Gateway       string
	IncomingQueue chan []*string
	Workers       int
	client        *http.Client
}

// GwResp captures the response string
// and numeric code from a polymur-gateway.
type GwResp struct {
	String string
	Code   int
}

// HttpWriter writes compressesed message batches over HTTPS
// to a polymur-gateway instance. Initial connection is OK'd
// by hitting the /ping path with a valid client API key registered
// with the polymur-gateway.
func HttpWriter(config *HttpWriterConfig, ready chan bool) {
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
	config.client = &http.Client{Transport: tr}

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
func writeStream(config *HttpWriterConfig, workerId int) {
	log.Printf("HTTP writer #%d started\n", workerId)

	for m := range config.IncomingQueue {
		log.Printf("[worker #%d] sending batch (%d data points)\n",
			workerId,
			len(m))

		data := packDataPoints(m)

		response, err := apiPost(config, "/ingest", data)
		if err != nil {
			// TODO need failure / retry logic.
			log.Printf("[worker #%d] [gateway]: %s", workerId, err)
			continue
		}

		log.Printf("[worker #%d] [gateway] %s", workerId, response.String)
	}
}

// apiPost is a convenience wrapper for submitting requests to
// a polymur-gateway and returning GwResp's.
func apiPost(config *HttpWriterConfig, path string, postData io.Reader) (*GwResp, error) {
	req, err := http.NewRequest("POST", config.Gateway+path, postData)
	if err != nil {
		return nil, err
	}

	req.Header.Add("X-polymur-key", config.ApiKey)
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
func packDataPoints(d []*string) io.Reader {
	var compressed bytes.Buffer
	w := gzip.NewWriter(&compressed)

	for _, s := range d {
		if s == nil {
			break
		}
		w.Write([]byte(*s))
		w.Write([]byte{10})
	}

	w.Close()

	return &compressed
}
