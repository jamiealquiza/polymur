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
package keysync

import (
	"log"
	"sync"
	"time"

	"github.com/jamiealquiza/consul/api"
)

type ApiKeys struct {
	sync.Mutex
	Keys map[string]string
}

func NewApiKeys() *ApiKeys {
	return &ApiKeys{
		Keys: make(map[string]string),
	}
}

func Run(keys *ApiKeys) {
	interval := 30
	timer := time.NewTicker(time.Duration(interval) * time.Second)
	defer timer.Stop()

	var newKeys, removedKeys uint8

	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		log.Fatal(err)
	}

	kv := client.KV()

	for {
		log.Printf("Running API key sync")

		registeredKeys, _, err := kv.List("/polymur/gateway/keys", nil)
		if err != nil {
			log.Printf("Key sync error: %s\n", err)
			log.Printf("Reattepting key sync in %ds\n", interval)
			goto wait
		}

		keys.Lock()
		// Update / add keys.
		for _, d := range registeredKeys {
			name, apikey := string(d.Key), string(d.Value)

			if _, present := keys.Keys[apikey]; !present {
				keys.Keys[apikey] = name
				newKeys++
			}
		}
		// Purge keys.
		for k, _ := range keys.Keys {
			if !keyPresent(k, registeredKeys) {
				delete(keys.Keys, k)
				removedKeys++
			}
		}
		keys.Unlock()

		log.Printf("API keys refreshed: %d new, %d removed\n",
			newKeys, removedKeys)

		newKeys, removedKeys = 0, 0

	wait:
		<-timer.C
	}
}

func keyPresent(k string, kvp api.KVPairs) bool {
	for _, kv := range kvp {
		key := string(kv.Value)
		if k == key {
			return true
		} else {
			continue
		}
	}
	return false
}
