# Overview

Under active development.

Polymur-gateway is a daemon for ingesting metrics forwarded over HTTPS. It accepts connections from remote [polymur-proxy](https://github.com/jamiealquiza/polymur/tree/master/cmd/polymur-proxy) instances with valid API keys registered within the gateway service (functionality currently not implemented - '123' is the current skeleton key :)). Mechanically, polymur-gateway is a standard [polymur](https://github.com/jamiealquiza/polymur) daemon with an HTTPS listener. Therefore, polymur-gateway can receive metrics forwarded by a polymur-proxy instance and distribute to downstream Graphite-compatible daemons (Polymur, Graphite carbon-relay and carbon-cache, etc.).

Messages batches are received, decompressed (gzip) and distributed to the configured `-destinations`.

![ScreenShot](https://raw.githubusercontent.com/jamiealquiza/catpics/master/polymur-proxy-gateway.png)

# Installation

- `go get github.com/jamiealquiza/polymur`
- `go install github.com/jamiealquiza/polymur/cmd/polymur-gateway`
- Binary will be found at `$GOPATH/bin/polymur-gateway`

# Usage

<pre>
Usage of ./polymur-gateway:  -api-addr string
        API listen address (default "localhost:2030")
  -cert string
        TLS Certificate
  -console-out
        Dump output to console
  -destinations string
        Comma-delimited list of ip:port destinations
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

# Configuration

Polymur-gateway currently uses a simple cert/key setup with clients distributed to connecting polymur-proxy instances. Generate test keys:
<pre>
openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout key.pem -out cert.pem
</pre>
