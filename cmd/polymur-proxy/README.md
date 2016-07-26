# Overview

Under active development.

Polymur-proxy is a daemon for off-site metrics forwarding over HTTPS. It connects to a remote [polymur-gateway](https://github.com/jamiealquiza/polymur/tree/master/cmd/polymur-gateway) instance with a valid API key issued by the gateway service (functionality currently not implemented - '123' is the current skeleton key :)). Mechanically, polymur-proxy is a standard [polymur](https://github.com/jamiealquiza/polymur) daemon with an HTTPS output writer. Therefore, polymur-proxy can natively accept inputs from collectd, statsd and any other tools that implement the standard 'Graphite protocol'.

Messages are batched, compressed (gzip; results in a ~5x reduction in outbound network bandwidth) and forwarded by a configurable number of workers (`-workers` directive) to the configured polymur-gateway (`-gateway` directive).

![ScreenShot](https://raw.githubusercontent.com/jamiealquiza/catpics/master/polymur-proxy-gateway.jpg)

# Installation

- `go get github.com/jamiealquiza/polymur`
- `go install github.com/jamiealquiza/polymur/cmd/polymur-proxy`
- Binary will be found at `$GOPATH/bin/polymur-proxy`

# Usage

<pre>
Usage of ./polymur-proxy:
  -api-key string
        polymur gateway API key
  -cert string
        TLS Certificate
  -console-out
        Dump output to console
  -gateway string
        polymur gateway address
  -listen-addr string
        Polymur-proxy listen address (default "0.0.0.0:2003")
  -metrics-flush int
        Graphite flush interval for runtime metrics (0 is disabled)
  -queue-cap int
        In-flight message queue capacity (default 32768)
  -stat-addr string
        runstats listen address (default "localhost:2020")
  -workers int
        HTTP output workers (default 3)
</pre>
