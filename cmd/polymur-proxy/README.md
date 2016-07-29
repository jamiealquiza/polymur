# Overview

Under active development.

Polymur-proxy is a daemon for off-site metrics forwarding over HTTPS. It connects to a remote [Polymur-gateway](https://github.com/jamiealquiza/polymur/tree/master/cmd/polymur-gateway) instance with a valid API key issued by the gateway service. Mechanically, Polymur-proxy is a standard [Polymur](https://github.com/jamiealquiza/polymur) daemon with an HTTPS output writer. Therefore, Polymur-proxy can natively accept inputs from collectd, statsd and any other tools that implement the standard 'Graphite protocol'.

Messages are batched, compressed (gzip; results in a ~5x reduction in outbound network bandwidth) and forwarded by a configurable number of workers (`-workers` directive) to the configured Polymur-gateway (`-gateway` directive).

![ScreenShot](https://raw.githubusercontent.com/jamiealquiza/catpics/master/polymur-proxy-gateway.png)

# Installation

Requires Go 1.6

- `go get -u github.com/jamiealquiza/polymur/...`
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

# Example Test Setup

### Setup Polymur-gateway
See the Polymur-gateway [readme](https://github.com/jamiealquiza/polymur/tree/master/cmd/polymur-gateway)

### Run Polymur-proxy
<pre>
$ ./polymur-proxy -cert="/path/to/cert.pem" -gateway="https://localhost:443" -api-key="test-key" -stat-addr="localhost:2021"
2016/07/29 14:26:53 Pinging gateway https://localhost:443
2016/07/29 14:26:53 Connection to gateway https://localhost:443 successful
2016/07/29 14:26:53 HTTP writer #2 started
2016/07/29 14:26:53 HTTP writer #0 started
2016/07/29 14:26:53 HTTP writer #1 started
2016/07/29 14:26:53 Metrics listener started: 0.0.0.0:20032016/07/29 14:26:53 Runstats started: localhost:2020
</pre>

Polymur-gateway authorizes key
<pre>
2016/07/29 14:26:53 [client 127.0.0.1:50352] key for polymur/gateway/keys/test-user is valid
</pre>

### Push Test Data
Send fake data point to Polymur-proxy
<pre>
$ echo "some.test.data 1337 $(date +%s)" | nc localhost 2003
</pre>

Polymur-proxy receives and forwards data
<pre>
2016/07/29 14:30:54 [worker #2] sending batch (1 data points)
2016/07/29 14:30:54 [worker #2] [gateway] Batch Received
2016/07/29 14:30:58 Last 5.00s: Received 1 data points | Avg: 0.20/sec.
</pre>

Polymur-gateway validates key and handles request (writes to console since the gateway was started with the `-console-out` flag)
<pre>
2016/07/29 14:30:54 [client 127.0.0.1:50352] Recieved batch from from polymur/gateway/keys/test-user
some.test.data 1337 1469802654
2016/07/29 14:30:55 Last 5.00s: Received 1 data points | Avg: 0.20/sec.
</pre>


