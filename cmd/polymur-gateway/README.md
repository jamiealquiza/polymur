# Overview

Polymur-gateway is a daemon for ingesting metrics forwarded over HTTPS. It accepts connections from remote [Polymur-proxy](https://github.com/jamiealquiza/polymur/tree/master/cmd/polymur-proxy) instances with valid API keys registered with the gateway service. Mechanically, polymur-gateway is a standard [Polymur](https://github.com/jamiealquiza/polymur) daemon with HTTP/HTTPS listeners. Therefore, Polymur-gateway can receive metrics forwarded by a Polymur-proxy instance and distribute to downstream Graphite-compatible daemons (Polymur, Graphite carbon-relay and carbon-cache, etc.).

![ScreenShot](https://raw.githubusercontent.com/jamiealquiza/catpics/master/polymur-proxy-gateway.png)

Messages batches are received, decompressed (gzip) and distributed to the configured `-destinations`.

Optionally (via `-key-prefix`), all ingested metrics can be prefixed with the name of the connecting Polymur-proxy's API key name, allowing automatic, per API user namespace separation with no changes required on the sending infrastructure. For instance, if the metric `web01.app.rate` originated from a Polymur-proxy instance configured with the API key where the key name is `customer-a`, the metric will be rewritten inline as `customer-a.web01.app.rate` before being sent the downstream destinations.

The HTTPS listener is optional and will only be initialized if both the `-cert` and `-key` parameters are specified.

Polymur-gateway checks for x-forwarded-for headers and if present, will use the xff IP for logging purposes (example: with Polymur-gateway configured behind and AWS ELB, the exit IP of the connecting Polymur-proxy will automatically be used in logging references rather than the ELB IP).

The Polymur-gateway API key service is backed with Consul's KV store and references KV pairs under the `/polymur/gateway/keys/` namespace. Keys are fetched on startup and synced every 30s to an in-memory cache. In the case that Consul becomes unreachable, the local key cache is simply not updated. 

Polymur-proxy checks the local API key cache with every batch received from connecting Polymur-proxy instances. If a previously valid key were to be unregistered, the connecting proxy instance will be disconnected with an invalid key error on the first batch attempt following a key synchronization within the Polymur-gateway instance.

# Installation

Requires Go 1.6

- `go get -u github.com/jamiealquiza/polymur/...`
- `go install github.com/jamiealquiza/polymur/cmd/polymur-gateway`
- Binary will be found at `$GOPATH/bin/polymur-gateway`

# Usage

Polymur-gateway uses [Envy](https://github.com/jamiealquiza/envy) to automatically accept all options as env vars (variables in brackets).

<pre>
Usage of polymur-gateway:
  -api-addr string
        API listen address [POLYMUR_GW_API_ADDR] (default "localhost:2030")
  -cert string
        TLS Certificate [POLYMUR_GW_CERT]
  -console-out
        Dump output to console [POLYMUR_GW_CONSOLE_OUT]
  -destinations string
        Comma-delimited list of ip:port destinations [POLYMUR_GW_DESTINATIONS]
  -dev-mode
        Dev mode: disables Consul API key store; uses '123' [POLYMUR_GW_DEV_MODE]
  -distribution string
        Destination distribution methods: broadcast, hash-route [POLYMUR_GW_DISTRIBUTION] (default "broadcast")
  -incoming-queue-cap int
        In-flight incoming message queue capacity (number of data point batches [100 points max per batch]) [POLYMUR_GW_INCOMING_QUEUE_CAP] (default 32768)
  -key string
        TLS Key [POLYMUR_GW_KEY]
  -key-prefix
        If enabled, prepends all metrics with the origin polymur-proxy API key's name [POLYMUR_GW_KEY_PREFIX]
  -listen-addr string
        Polymur-gateway listen address [POLYMUR_GW_LISTEN_ADDR] (default "0.0.0.0")
  -listen-http-port string
        Polymur-gateway listen port (http) [POLYMUR_GW_LISTEN_HTTP_PORT]
  -listen-https-port string
        Polymur-gateway listen port (https) [POLYMUR_GW_LISTEN_HTTPS_PORT]
  -metrics-flush int
        Graphite flush interval for runtime metrics (0 is disabled) [POLYMUR_GW_METRICS_FLUSH]
  -outgoing-queue-cap int
        In-flight message queue capacity per destination (number of data points) [POLYMUR_GW_OUTGOING_QUEUE_CAP] (default 4096)
  -stat-addr string
        runstats listen address [POLYMUR_GW_STAT_ADDR] (default "localhost:2020")
</pre>

# Example Test Setup

### Generate cert/keys
Polymur-gateway optionally supports HTTPS. Generate test keys:
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

Register a test key (with curl)
<pre>
$ curl -XPUT localhost:8500/v1/kv/polymur/gateway/keys/test-user -d 'test-key'
true
</pre>

**or**

Register a test key with [pgw-key](https://github.com/jamiealquiza/polymur/tree/master/cmd/utils/pgw-key)
<pre>
% ./pgw-key create test-key
Successfully registered test-key with key 19d2d0f0130a9c9a6f97bdb0
</pre>

### Start Polymur-gateway
<pre>
$ ./polymur-gateway -key="/path/to/key.pem" -cert="/path/to/cert.pem" -console-out
2016/07/29 14:16:13 ::: Polymur-gatway :::
2016/07/29 14:16:13 Running API key sync
2016/07/29 14:16:13 HTTP listening on 0.0.0.0:80
2016/07/29 14:16:13 HTTPS listening on 0.0.0.0:443
2016/07/29 14:16:13 API started: localhost:2030
2016/07/29 14:16:13 Runstats started: localhost:2020
2016/07/29 14:16:13 API keys refreshed: 1 new, 0 removed
</pre>
