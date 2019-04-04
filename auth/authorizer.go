package auth

import (
	"log"
	"net/http"

	"github.com/jamiealquiza/polymur/keysync"
)

type Authorizer interface {
	GetValidationKey(request *http.Request) string
	Validate(request *http.Request) (string, bool)
}

type ConsulAuthorizer struct {
	Keys *keysync.APIKeys
}

func InitConsulKeys(devMode bool) *keysync.APIKeys {
	// API key sync service.
	apiKeys := keysync.NewAPIKeys()
	if !devMode {
		go keysync.Run(apiKeys)
	} else {
		apiKeys.Keys["123"] = "dev"
		log.Println("Running in dev-mode: API-key set to '123'")
	}
	return apiKeys
}

// validateKey looks up if a key is registered in Consul
// and returns the key name and key.
func validateConsulKey(k string, keys *keysync.APIKeys) (string, bool) {
	keys.Lock()
	name, valid := keys.Keys[k]
	keys.Unlock()

	if valid {
		return name, true
	}

	return "", false
}

func (v ConsulAuthorizer) GetValidationKey(request *http.Request) string {
	return request.Header.Get("X-Polymur-Key")
}

func (v ConsulAuthorizer) Validate(request *http.Request) (string, bool) {
	key := v.GetValidationKey(request)
	return validateConsulKey(key, v.Keys)
}

func NewConsulAuthorizer(devMode bool) ConsulAuthorizer {
	return ConsulAuthorizer{Keys: InitConsulKeys(devMode)}
}

func NewAuthorizer(authMethod string, devMode bool) Authorizer {
	//currently only authorizing with consul keys on the backend
	return NewConsulAuthorizer(devMode)
}