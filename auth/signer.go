package auth

import "net/http"

type Signer interface {
	Sign(request *http.Request, key string)
}

//AWS API Gateway validation path:
//proxy sends API key
//API Gateway validates key and sends request along to gateway, injecting polymur consul key
//gateway validates off of polymur key sent by the API gateway against consul
type APIGatewaySigner struct{}

func (v APIGatewaySigner) Sign(request *http.Request, key string) {
	request.Header.Add("x-api-key", key)
}

type ConsulSigner struct{}

func (v ConsulSigner) Sign(request *http.Request, key string) {
	request.Header.Add("X-polymur-key", key)
}

func NewSigner(auth string) Signer {
	var a Signer
	if auth == "api-gateway" {
		a = APIGatewaySigner{}
	} else {
		a = ConsulSigner{}
	}
	return a
}