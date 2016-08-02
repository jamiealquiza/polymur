# Overview

pgw-key is a helper utility for managing [Polymur-gateway](https://github.com/jamiealquiza/polymur/tree/master/cmd/polymur-gateway) API key registration in Consul.

# Installation

Requires Go 1.6

- `go get -u github.com/jamiealquiza/polymur/...`
- `go install github.com/jamiealquiza/polymur/cmd/util/pgw-key`
- Binary will be found at `$GOPATH/bin/pgw-key`

# Usage
<pre>
$ ./pgw-key 
Commands ('pgw-key &ltcommand&gt' for help):
	list
	create
	regen
	delete
</pre>

# Example

### Create a test key
<pre>
% ./pgw-key create test
Successfully registered test with key 0a8208f91d178f2b634fc0f6
$ ./pgw-key list all
test: 0a8208f91d178f2b634fc0f6
</pre>

### Key sync'd in Polymur-gateway
<pre>
2016/08/02 14:21:33 Running API key sync
2016/08/02 14:21:33 API keys refreshed: 1 new, 0 removed
</pre>

### Running Polymur-proxy with the test key
<pre>
$ ./polymur-proxy -gateway="https://localhost:443" -cert="/path/to/cert.pem"  -api-key="0a8208f91d178f2b634fc0f6"
2016/08/02 14:21:35 ::: Polymur-proxy :::
2016/08/02 14:21:35 Pinging gateway https://localhost:443
2016/08/02 14:21:35 Connection to gateway https://localhost:443 successful
</pre>

### Key validated by the gateway
<pre>
2016/08/02 14:21:35 [client 127.0.0.1:52647] key for test is valid
</pre>
