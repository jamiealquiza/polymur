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

// KeyNameByKey returns a keys name by key lookup.
func (keys *ApiKeys) KeyNameByKey(k string) string {
	keys.Lock()
	name, valid := keys.Keys[k]
	keys.Unlock()

	if valid {
		return name
	} else {
		return ""
	}
}

// KeyNameExists returns whether or not a key by name
// exists.
func (keys *ApiKeys) KeyNameExists(k string) bool {
	keys.Lock()
	defer keys.Unlock()

	for _, keyName := range keys.Keys {
		if keyName == k {
			return true
		}
	}

	return false
}

func NewApiKeys() *ApiKeys {
	return &ApiKeys{
		Keys: make(map[string]string),
	}
}

func Run(localKeys *ApiKeys) {
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

		newKeys, removedKeys = Sync(localKeys, registeredKeys)

		log.Printf("API keys refreshed: %d new, %d removed\n",
			newKeys, removedKeys)

	wait:
		<-timer.C
	}
}

// Sync syncronizes a *ApiKeys with what is registered
// in Consul and returns the new keys and removed keys count delta.
func Sync(localKeys *ApiKeys, registeredKeys api.KVPairs) (uint8, uint8) {
	var newKeys, removedKeys uint8

	localKeys.Lock()
	// Update / add keys.
	for _, d := range registeredKeys {
		// Strip the namespace prefix.
		// Currently `polymur/gateway/keys/keyname`.
		name, apikey := string(d.Key[21:]), string(d.Value)

		if _, present := localKeys.Keys[apikey]; !present {
			localKeys.Keys[apikey] = name
			newKeys++
		}
	}
	// Purge keys.
	for k, _ := range localKeys.Keys {
		if !keyRegistered(k, registeredKeys) {
			delete(localKeys.Keys, k)
			removedKeys++
		}
	}
	localKeys.Unlock()

	return newKeys, removedKeys
}

// keyRegistered checks whether a key is
// registered in Consul.
func keyRegistered(k string, kvp api.KVPairs) bool {
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
