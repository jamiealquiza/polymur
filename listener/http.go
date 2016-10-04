// Package listener handles the ingestion for polymur
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
package listener

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/chrissnell/polymur/keysync"
	"github.com/chrissnell/polymur/statstracker"
)

// HTTPListenerConfig hold configuration for the HTTP listener
type HTTPListenerConfig struct {
	Addr                  string
	Port                  string
	IncomingQueue         chan []*string
	CA                    string
	Cert                  string
	Key                   string
	KeyPrefix             bool
	UseCertAuthentication bool
	Stats                 *statstracker.Stats
	Keys                  *keysync.ApiKeys
}

// HTTPListener accepts connections from a polymur-proxy
// client. Upon a successful /ping client API key validation,
// batches of compressed messages are passed to /ingest handler.
func HTTPListener(config *HTTPListenerConfig) {
	http.HandleFunc("/ingest", func(w http.ResponseWriter, req *http.Request) { ingest(w, req, config) })
	http.HandleFunc("/ping", func(w http.ResponseWriter, req *http.Request) { ping(w, req, config) })

	if config.Cert != "" && config.Key != "" {
		go func() {
			var caPool *x509.CertPool

			if config.CA != "" {
				cert, err := ioutil.ReadFile(config.CA)
				if err != nil {
					log.Fatal(err)
					return
				}

				// Use client cert.
				caPool = x509.NewCertPool()
				ok := caPool.AppendCertsFromPEM(cert)
				if !ok {
					log.Fatal("Error parsing certificate")
				}

				tlsConfig := &tls.Config{
					// Reject any TLS certificate that cannot be validated
					ClientAuth: tls.RequireAndVerifyClientCert,
					// Ensure that we only use our "CA" to validate certificates
					ClientCAs: caPool,
					// PFS because we can but this will reject client with RSA certificates
					CipherSuites: []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
					// Force it server side
					PreferServerCipherSuites: true,
					// TLS 1.2 because we can
					MinVersion: tls.VersionTLS12,
				}

				tlsConfig.BuildNameToCertificate()

				httpServer := &http.Server{
					Addr:      fmt.Sprint(config.Addr, ":", config.Port),
					TLSConfig: tlsConfig,
				}

				err = httpServer.ListenAndServeTLS(config.Cert, config.Key)
				if err != nil {
					log.Fatalf("ListenAndServe: %s\n", err)
				}

			} else {

				log.Printf("HTTPS listening on %s\n", fmt.Sprint(config.Addr, ":", config.Port))
				err := http.ListenAndServeTLS(fmt.Sprint(config.Addr, ":", config.Port), config.Cert, config.Key, nil)
				if err != nil {
					log.Fatalf("ListenAndServeTLS: %s\n", err)
				}

			}
		}()

	} else {
		go func() {
			log.Printf("HTTP listening on %s\n", fmt.Sprint(config.Addr, ":", config.Port))
			err := http.ListenAndServe(fmt.Sprint(config.Addr, ":", config.Port), nil)
			if err != nil {
				log.Fatalf("ListenAndServe: %s\n", err)
			}
		}()
	}
}

// ingest is a handler that accepts a batch of compressed data points.
// Data points arive as a concatenated string with newline delimition.
// Each batch is broken up and populated into a []*string and pushed
// to the IncomingQueue for downstream destination writing.
func ingest(w http.ResponseWriter, req *http.Request, config *HTTPListenerConfig) {
	var requestKey, keyName string
	var valid bool

	var client string
	xff := req.Header.Get("x-forwarded-for")
	if xff != "" {
		client = xff
	} else {
		client = req.RemoteAddr
	}

	if !config.UseCertAuthentication {
		requestKey = req.Header.Get("X-Polymur-Key")

		keyName, valid = validateKey(requestKey, config.Keys)
		if !valid {
			log.Printf("[client %s] %s is not a valid key\n",
				client, requestKey)

			resp := fmt.Sprintf("invalid key")
			req.Close = true
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, resp)

			return
		}
	} else {
		keyName = req.TLS.PeerCertificates[0].Subject.CommonName
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
func ping(w http.ResponseWriter, req *http.Request, config *HTTPListenerConfig) {
	var client string
	xff := req.Header.Get("x-forwarded-for")
	if xff != "" {
		client = xff
	} else {
		client = req.RemoteAddr
	}

	if config.UseCertAuthentication {
		log.Println("TLS Certificate authentication suceeded for",
			req.TLS.PeerCertificates[0].Subject.CommonName)
		io.WriteString(w, "key is valid\n")
	} else {
		requestKey := req.Header.Get("X-Polymur-Key")
		keyName, valid := validateKey(requestKey, config.Keys)

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

}

// validateKey looks up if a key is registered in Consul
// and returns the key name and key.
func validateKey(k string, keys *keysync.ApiKeys) (string, bool) {
	keys.Lock()
	name, valid := keys.Keys[k]
	keys.Unlock()

	if valid {
		return name, true
	}
	return "", false

}
