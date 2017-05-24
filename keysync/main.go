package keysync

import (
	"log"
	"sync"
	"time"

	"github.com/jamiealquiza/consul/api"
)

type APIKeys struct {
	sync.Mutex
	Keys map[string]string
}

// KeyNameByKey returns a keys name by key lookup.
func (keys *APIKeys) KeyNameByKey(k string) string {
	keys.Lock()
	name, valid := keys.Keys[k]
	keys.Unlock()

	if valid {
		return name
	}

	return ""
}

// KeyNameExists returns whether or not a key by name
// exists.
func (keys *APIKeys) KeyNameExists(k string) bool {
	keys.Lock()
	defer keys.Unlock()

	for _, keyName := range keys.Keys {
		if keyName == k {
			return true
		}
	}

	return false
}

func NewAPIKeys() *APIKeys {
	return &APIKeys{
		Keys: make(map[string]string),
	}
}

func Run(localKeys *APIKeys) {
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

// Sync syncronizes a *APIKeys with what is registered
// in Consul and returns the new keys and removed keys count delta.
func Sync(localKeys *APIKeys, registeredKeys api.KVPairs) (uint8, uint8) {
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
	for k := range localKeys.Keys {
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
		}
	}

	return false
}
