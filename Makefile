BINS=polymur-gateway polymur polymur-proxy
all: clean polymur-gateway polymur-proxy polymur

polymur-gateway: cmd/polymur-gateway/main.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go get -u github.com/jamiealquiza/consul
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go get github.com/chrissnell/polymur/...
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o polymur-gateway ./cmd/polymur-gateway

polymur-proxy: cmd/polymur-proxy/main.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o polymur-proxy ./cmd/polymur-proxy

polymur: cmd/polymur/main.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o polymur ./cmd/polymur

clean:
	rm -rf $(BINS)
