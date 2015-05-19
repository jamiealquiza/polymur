# polymur

WIP.

### Overview

Polymur is a service that accepts Graphite plaintext protocol metrics (LF delimited messages) from tools like Collectd or Statsd, and either replicates or round-robins the output to one or more destinations. Polymur is efficient at terminating many thousands of connections and provides in-line buffering (should a destination become temporarily unavailable), runtime destination manipulation ("I want to try InfluxDB, let's mirror all our production metrics to x.x.x.x"), and failover redistribution (in round-robin mode: if node C fails, redistribute in-flight metrics for this destination to nodes A and B).

Polymur was created to introduce more flexibility into the way metrics streams are managed and to reduce the total number of components needed to operate Graphite deployments. It's built in a highly concurrent fashion and doesn't need multiple instances per-node with a local load-balancer if it's being used as a Carbon relay upstream from your Graphite servers. If it's being used as a Carbon relay on your Graphite server to distribute metrics to Carbon-cache daemons, daemons can self register themselves on start using Polymur's simple API.

#### Polymur replacing upstream relays

Terminating connections from all sending hosts and distributing to downstream Graphite servers:

![ScreenShot](https://d1n2314jgy7p59.cloudfront.net/polymur-relay-b.jpg)

#### Polymur replacing local relays

Polymur running on a Graphite server in round-robin mode, distributing metrics from upstream relays to local carbon-cache daemons:

![ScreenShot](https://d1n2314jgy7p59.cloudfront.net/polymur-relay-c.jpg)

### Installation

- `go get github.com/jamiealquiza/polymur`
- `go install github.com/jamiealquiza/polymur`
- Binary will be found at `$GOPATH/bin/polymur`

### Usage

<pre>
Usage of ./polymur:
  -console-out=false: Dump output to console
  -destinations="": Comma-delimited list of ip:port destinations
  -distribution="broadcast": Destination distribution methods: broadcast, balance-rr
  -listen-addr="0.0.0.0": bind address
  -listen-port="2003": bind port
  -metrics-flush=0: Graphite flush interval for runtime metrics (0 is disabled)
  -queue-cap=4096: In-flight message queue capacity to any single destination
</pre>

### Examples

#### Running

Listening for incoming metrics on `0.0.0.0:2003` and mirroring to `10.0.5.20:2003` and `10.0.5.30:2003`:
<pre>
./polymur -destinations=10.0.5.20:2003,10.0.5.30:2003 -metrics-flush=30 -listen-port=2003 -listen-addr=0.0.0.0 -distribution="broadcast"
2015/05/14 15:19:08 Registered destination 10.0.5.20:2003
2015/05/14 15:19:08 Registered destination 10.0.5.30:2003
2015/05/14 15:19:08 Adding destination to connection pool: 10.0.5.30:2003
2015/05/14 15:19:08 Adding destination to connection pool: 10.0.5.20:2003
2015/05/14 15:19:09 API started: localhost:2030
2015/05/14 15:19:09 Runstats started: localhost:2020
2015/05/14 15:19:09 Metrics listener started: 0.0.0.0:2003
2015/05/14 15:19:14 Last 5s: Received 7276 data points | Avg: 1455.20/sec. | Inbound queue length: 0
2015/05/14 15:19:19 Last 5s: Received 6471 data points | Avg: 1294.20/sec. | Inbound queue length: 0
2015/05/14 15:19:24 Last 5s: Received 6744 data points | Avg: 1348.80/sec. | Inbound queue length: 0
2015/05/14 15:19:29 Last 5s: Received 5806 data points | Avg: 1161.20/sec. | Inbound queue length: 0
</pre>

#### Changing destinations

Polymur started with no initial destinations:
<pre>
% echo getdest | nc localhost 2030
{
 "active": [],
 "registered": {}
}
</pre>

Add destination at runtime:
<pre>
% echo putdest localhost:6020 | nc localhost 2030          
Registered destination: localhost:6020

% echo getdest | nc localhost 2030
{
 "active": [
  "localhost:6020"
 ],
 "registered": {
  "localhost:6020": "2015-05-14T09:09:23.620410265-06:00"
 }
}
</pre>

Polymur output:
<pre>
./polymur -distribution="balance-rr" -listen-port="2003"
2015/05/14 09:09:15 Metrics listener started: 0.0.0.0:2003
2015/05/14 09:09:15 API started: localhost:2030
2015/05/14 09:09:15 Runstats started: localhost:2020
2015/05/14 09:09:23 Registered destination localhost:6020
2015/05/14 09:09:23 Adding destination to connection pool: localhost:6020
</pre>

### Internals

Terminology:

- **Destination**: where metrics will be forwarded using the Graphite plaintext protocol
- **Destination queue**: metrics in-flight for a given destination
- **Registered**: a candidate destination loaded into Polymur, but not necessarily active
- **Connection**: a registered destination with an active connection
- **Connection pool**: global list of all active connections and their respective destination queue
- **Distribution mode**: how metrics are distributed to destinations (round-robin, broadcast)
- **Retry queue**: messages that couldn't be sent to their destination are loaded into the retry queue (e.g. a round-robin node is removed from the connection pool)

Polymur listens on the configured addr:port for incoming connections, each connection handled in a dedicated Goroutine. A connection Goroutine reads the inbound stream and allocates a message string at LF boundaries. Messages are accumulated as slices of string pointers and flushed to a shared inbound queue, either after 5 seconds or when the slice hits 30 elements (to reduce [channel operations](https://grey-boundary.io/concurrent-communication-performance-in-go/)). 

Message batches from the inbound queue are distributed (broadcast or round-robin) to a dedicated queue for each output destination. Destination output is also handled using dedicated Goroutines, where temporary latency or full disconnects to one destination will not impact write performance to another destination. If a destination becomes unreachable, the endpoint will be retried at 15 second intervals while the respective destination queue buffers incoming messages. Per destination queue capacity is determined by the `-queue-cap` directive. Any destination queue with an outstanding length greater than 0 will be logged to stdout. Any destination queue that exceeds the configured `-queue-cap` will not receive any new messages until the queue is cleared. If the distribution mode is round-robin, three consecutive reconnect attempt failures will result in removing the connection from the connection pool and redistributing any in-flight messages to the retry queue.

Note: memory footprint for in-flight messages will be roughly:

- average worst case: *message size * `-queue-cap`* (assuming every queue is full)
- the finale to How I Met Your Mother worst case: *message size * number of destinations * `-queue-cap`* (assuming every queue is full and each queue is referencing unique messages not found in any other queue)

### FAQ

#### Why no consistent-hashing?
I no longer use multi-node Graphite [setups](https://grey-boundary.io/the-architecture-of-clustering-graphite/) with CH. While it functions and helper tools exist, CH distribution to storage without attributes such as hand-off or an ability to rebalance after changing a hash ring is operationally clumsy. Lastly, CH may be added if RR is found to have a negative performance impact while distributing metrics to local carbon-cache daemons as a local relay (see diagrams).

#### Why no pickle protocol?
Pickling is a native Python construct; Polymur is written in Go (acknowledging 3rd party libraries exist). More importantly, I have yet to encounter a situation where network was starved before any Carbon daemon became CPU bound, and data serialization certainly doesn't improve that situation. That said, I will be adding protobuf for Polymur to Polymur communication.
