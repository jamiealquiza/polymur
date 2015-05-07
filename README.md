# graphite-multiplier
Mirrors Graphite metrics to multiple destinations

### Overview

Receives Graphite plaintext protocol (LF delimited messages) from collectd/statsd/etc. and replicates to multiple destinations. Intended to be a higher performance replacement for carbon-relay mirroring with additional health checks / failure mode controls.
