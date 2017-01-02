#!/usr/bin/env python
# This script can be used to feed test metrics to the polymur-gateway. It can be useful to test the functionality of the polymur infrastructure.
import argparse
import calendar
import consulate
import os
import StringIO
import time
import gzip
import requests

parser = argparse.ArgumentParser()
parser.add_argument("gateway_url", nargs="?", default="http://localhost")
args = parser.parse_args()

# The following function builds some test metrics. The format of test metrics is as follows
# (test-host-<service_name><number>.load.midterm <value> <timestamp>). It generates 10 such metrics per service.
def gen_metric(service):
	"gen_metric creates some test metrics and returns that after compressing it"

	t = int(calendar.timegm(time.gmtime()))
	metric_prefix = "test-host-%s" % (service)
	metric = ""
	for i in range(10):
		m = "%s%s" % (metric_prefix,  str(i))
		metric += "%s.load.midterm %d %d\n" % (m, i, t)
	print metric
	out = StringIO.StringIO()
	with gzip.GzipFile(fileobj=out, mode="w") as f:
		f.write(metric)
	return out.getvalue()


def send_metric(payload, api_key):
	"Sends the metrics via http post with the payload and the api key"

	headers = {'X-polymur-key': api_key}
	url = "%s%s" % (args.gateway_url, "/ingest")
	r = requests.post(url, headers=headers, data=payload)

def main():
        c = consulate.Consul()
        # find all the polymur services from consul and send test metrics against every service.
	for srv, api_key in c.kv.find("polymur/gateway/keys").items():
	    p = gen_metric(os.path.basename(srv))
	    send_metric(p, api_key)

if __name__ == "__main__":
  main()
