# Overview

Under active development.

Polymur-gateway is a daemon for ingesting metrics forwarded over HTTPS. It accepts connections from remote [Polymur-proxy](https://github.com/jamiealquiza/polymur/tree/master/cmd/polymur-proxy) instances with valid API keys registered with the gateway service. Mechanically, polymur-gateway is a standard [Polymur](https://github.com/jamiealquiza/polymur) daemon with an HTTPS listener. Therefore, Polymur-gateway can receive metrics forwarded by a Polymur-proxy instance and distribute to downstream Graphite-compatible daemons (Polymur, Graphite carbon-relay and carbon-cache, etc.).

Messages batches are received, decompressed (gzip) and distributed to the configured `-destinations`.

![ScreenShot](https://raw.githubusercontent.com/jamiealquiza/catpics/master/polymur-proxy-gateway.png)

The Polymur-gateway API key service is backed with Consul's KV store and references KV pairs under the `/polymur/gateway/keys/` namespace. Keys are fetched on startup and synced every 30s to an in-memory cache. In the case that Consul becomes unreachable, the local key cache is simply not updated. 

Polymur-proxy checks the local API key cache with every batch received from connecting Polymur-proxy instances. If a previously valid key were to be unregistered, the connecting proxy instance will be disconnected with an invalid key error on the first batch attempt following a key synchronization within the Polymur-gateway instance.

# Installation

Requires Go 1.6

- `go get -u github.com/jamiealquiza/polymur/...`
- `go install github.com/jamiealquiza/polymur/cmd/polymur-gateway`
- Binary will be found at `$GOPATH/bin/polymur-gateway`

# Usage

<pre>
Usage of ./polymur-gateway:
  -api-addr string
        API listen address (default "localhost:2030")
  -cert string
        TLS Certificate
  -console-out
        Dump output to console
  -destinations string
        Comma-delimited list of ip:port destinations
  -dev-mode
        Dev mode: disables Consul API key store; uses '123'
  -distribution string
        Destination distribution methods: broadcast, hash-route (default "broadcast")
  -key string
        TLS Key
  -listen-addr string
        Polymur-gateway listen address (default "0.0.0.0:443")
  -metrics-flush int
        Graphite flush interval for runtime metrics (0 is disabled)
  -queue-cap int
        In-flight message queue capacity per destination (default 4096)
  -stat-addr string
        runstats listen address (default "localhost:2020")
</pre>

# Example Test Setup

### Generate cert/keys
Polymur-gateway currently uses a simple cert/key setup with clients distributed to connecting polymur-proxy instances. Generate test keys:
<pre>
$ openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout key.pem -out cert.pem
</pre>

### Initialize Consul key store
Start Consul
<pre>
$ ./consul agent -dev
==> Starting Consul agent...
==> Starting Consul agent RPC...
==> Consul agent running!
</pre>

Register a test key
<pre>
$ curl -XPUT localhost:8500/v1/kv/polymur/gateway/keys/test-user -d 'test-key'
true
</pre>

### Start Polymur-gateway
<pre>
$ ./polymur-gateway -key="/path/to/key.pem" -cert="/path/to/cert.pem" -console-out
2016/07/29 14:16:13 ::: Polymur-gatway :::
2016/07/29 14:16:13 Running API key sync
2016/07/29 14:16:13 API started: localhost:2030
2016/07/29 14:16:13 Runstats started: localhost:2020
2016/07/29 14:16:13 API keys refreshed: 1 new, 0 removed
</pre>
