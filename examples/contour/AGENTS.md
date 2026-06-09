# Instructions for Using Contour and Envoy Test Bench

Operational reference for working with the example environment.
See [README.md](README.md) for initial setup, exposed ports and deployment instructions.

The envirornment included Contour and Envoy, with the `echoserver` as upstream service.
Echoserver is an HTTP server that echoes request details as JSON and exposes various endpoints for testing.
See https://github.com/tsaarni/echoserver for more details.

### Sending Requests

Echoserver is exposed via Envoy on following virtual hosts:

```bash
# plain HTTP, no authentication
curl http://echoserver.127.0.0.1.nip.io/

# HTTPS with TLS termination at Envoy
curl --cacert certs/external-ca.pem https://echoserver-tls.127.0.0.1.nip.io/

# HTTPS with client certificate authentication
curl --cacert certs/external-ca.pem --cert certs/external-client.pem --key certs/external-client-key.pem https://echoserver-cert-auth.127.0.0.1.nip.io/

# HTTPS with JWT authentication, requires a valid JWT token
curl --cacert certs/external-ca.pem -H "Authorization: Bearer $(go run -C tools . jwt sign)" https://echoserver-jwt.127.0.0.1.nip.io/
```

The `HTTPProxy` resources for above virtual hosts are defined in `example/contour/exposure.yaml`.

## Credentials

### Certificates

When `setup.sh` is run, `certyaml` generates certificates using the configuration in `examples/contour/configs/certs.yaml`. The certificates are stored in `examples/contour/certs/` and mounted into the pods via hostPath volumes.

| Certificate | Issuer | Purpose |
|-------------|--------|---------|
| `external-ca` | self-signed | CA for external clients |
| `external-client` | `external-ca` | Client cert for testing cert auth endpoint |
| `cluster-internal-ca` | self-signed | CA for cluster-internal services |
| `echoserver` | `cluster-internal-ca` | Server cert for echoserver (TLS termination at Envoy) |
| `echoserver-backend` | `cluster-internal-ca` | Server cert for echoserver backend (re-encrypt) |
| `untrusted-ca` | self-signed | CA not trusted by Envoy, for negative testing |
| `untrusted-client` | `untrusted-ca` | Client cert not trusted by Envoy |

### JWT Signing Keys

| File | Purpose |
|------|---------|
| `jwt-signing-key.pem` | ECDSA P-256 private key for signing JWT tokens |
| `jwks.json` | JWKS public key set matching `jwt-signing-key.pem` |
| `jwt-signing-key-other.pem` | Alternate signing key (for testing key mismatch) |
| `jwks-other.json` | JWKS for the alternate key |


### Tokens

Generate a JWT token for testing the JWT auth endpoint:

```
TOKEN=$(go run -C examples/contour/tools . jwt sign)
curl -k -H "Authorization: Bearer $TOKEN" https://echoserver-jwt.127.0.0.1.nip.io/
```

Token options:

```bash
go run -C examples/contour/tools . jwt sign \
  --issuer https://example.com \
  --audience echoserver \
  --subject alice \
  --expiry 30s
```

Note: Envoy's jwt_authn filter has a default clock skew tolerance of 60 seconds, meaning expired tokens are still accepted for up to 60s after their `exp` time.


## Tools CLI

The `examples/contour/tools/` directory contains a helper CLI for generating different types of traffic to the echoserver endpoints, as well as a combined resource summary command that collects statistics from Envoy, Contour, echoserver.

Examples:

```bash
go run -C examples/contour/tools . stats
go run -C examples/contour/tools . http --rps 2000 --duration 120s
go run -C examples/contour/tools . tls --rps 2000 --duration 120s
go run -C examples/contour/tools . cert-auth --rps 500 --duration 60s
go run -C examples/contour/tools . jwt-load --rps 1000 --duration 60s
go run -C examples/contour/tools . upload --size 1048576 --rps 50 --duration 60s
go run -C examples/contour/tools . connections --rps 3000 --duration 60s
```

Run `go run -C examples/contour/tools . --help` for full usage.

## Envoy Admin Interface

Contour configures Envoy's admin interface as a Unix domain socket. The `setup.sh` script injects an `admin-proxy` socat sidecar that bridges the socket to TCP port 30901, which is then exposed externally on the host as `localhost:9001` via kind's NodePort mapping.

Upstream docs: https://www.envoyproxy.io/docs/envoy/latest/operations/admin

Full list of endpoints:

```bash
curl -s http://localhost:9001/help
```

Config Dump

```bash
# Full config dump
curl -s http://localhost:9001/config_dump | jq .

# Include EDS (endpoint discovery)
curl -s http://localhost:9001/config_dump?include_eds | jq .

# Filter by name regex
curl -s 'http://localhost:9001/config_dump?name_regex=echoserver' | jq .

# Filter by resource type (dynamic_listeners, dynamic_active_clusters, dynamic_route_configs)
curl -s 'http://localhost:9001/config_dump?resource=dynamic_listeners' | jq .
```

Contour names clusters as `namespace/service/port/hash`. The hash changes when cluster config changes (e.g., load balancer policy).
The `alt_stat_name` is `namespace_service_port` (used in stats).

```bash
# All clusters with health status
curl -s http://localhost:9001/clusters

# As JSON - show name, endpoint IPs, health
curl -s 'http://localhost:9001/clusters?format=json' | jq '.cluster_statuses[] | {name: .name, hosts: [.host_statuses[]? | {addr: .address.socket_address, health: .health_status.eds_health_status}]}'
```

```bash
# List active listeners
curl -s 'http://localhost:9001/listeners?format=json' | jq '.listener_statuses[] | {name, addr: .local_address.socket_address}'

# HTTP filters in use
curl -s 'http://localhost:9001/config_dump?resource=dynamic_listeners' | jq '[.configs[0].active_state.listener.filter_chains[].filters[].typed_config.http_filters[]?.name] | unique'
```

```bash
# Virtual hosts and route-to-cluster mapping
curl -s http://localhost:9001/config_dump | jq '.configs[] | select(."@type" | contains("Routes")) | .dynamic_route_configs[]?.route_config.virtual_hosts[] | {name, domains, routes: [.routes[]? | {match: .match, cluster: .route.cluster}]}'
```

```bash
# Which pod IPs serve each cluster
curl -s 'http://localhost:9001/config_dump?include_eds' | jq '.configs[] | select(."@type" | contains("Endpoints")) | .dynamic_endpoint_configs[]? | {cluster: .endpoint_config.cluster_name, endpoints: [.endpoint_config.endpoints[]?.lb_endpoints[]?.endpoint.address.socket_address | "\(.address):\(.port_value)"]}'
```

```bash
curl -s http://localhost:9001/server_info | jq '{version: .version, state: .state, uptime: .uptime_current_epoch, concurrency: .command_line_options.concurrency}'
```

Memory

```bash
curl -s http://localhost:9001/memory | jq .
```

Fields (mapped from tcmalloc):
- `allocated` — `generic.current_allocated_bytes` — bytes currently used by app
- `heap_size` — `generic.heap_size + pageheap_unmapped` — total heap (mapped + unmapped)
- `pageheap_unmapped` — `tcmalloc.pageheap_unmapped_bytes` — released back to OS
- `pageheap_free` — `tcmalloc.pageheap_free_bytes` — free mapped pages ready for use
- `total_thread_cache` — `tcmalloc.current_total_thread_cache_bytes` — thread-local caches
- `total_physical_bytes` — `generic.physical_memory_used` — total physical memory

Heap pressure formula (overload manager):
```
pressure = (heap_size - pageheap_unmapped) / configured_max_heap_size
```

Stats

Upstream docs: https://www.envoyproxy.io/docs/envoy/latest/operations/stats_overview

```bash
# All stats (text)
curl -s http://localhost:9001/stats

# Used only (skip zeros)
curl -s 'http://localhost:9001/stats?usedonly'

# Filter by regex
curl -s 'http://localhost:9001/stats?filter=upstream_rq'

# Prometheus format
curl -s http://localhost:9001/stats/prometheus

# Counter types only
curl -s 'http://localhost:9001/stats?type=Counters&usedonly'

# Gauges only
curl -s 'http://localhost:9001/stats?type=Gauges&usedonly'

# Histograms
curl -s 'http://localhost:9001/stats?type=Histograms&histogram_buckets=summary'
```

Key stat patterns:
- `cluster.<alt_stat_name>.upstream_rq_*` — upstream request counts
- `cluster.<alt_stat_name>.upstream_cx_*` — upstream connection counts
- `cluster.<alt_stat_name>.membership_*` — endpoint count and health
- `http.ingress_http.downstream_rq_*` — downstream request counts
- `http.ingress_http.downstream_cx_*` — downstream connection counts
- `server.memory_*` — memory stats (also available via `/memory`)

Reset Counters

```bash
curl -s -X POST http://localhost:9001/reset_counters
```

Note: CDS updates (cluster config changes) can also reset some per-cluster counters. Changing HTTPProxy load balancer policy triggers a CDS update which creates a new cluster (new hash in name), effectively losing the old cluster's stats.

Runtime Settings

```bash
# View current runtime values
curl -s http://localhost:9001/runtime | jq .

# Modify runtime value (live, no restart)
curl -s -X POST 'http://localhost:9001/runtime_modify?envoy.reloadable_features.some_feature=false'

# Clear a runtime override (set to empty)
curl -s -X POST 'http://localhost:9001/runtime_modify?envoy.reloadable_features.some_feature='
```

Logging

```bash
# Show current log levels
curl -s -X POST http://localhost:9001/logging

# Set all to debug
curl -s -X POST 'http://localhost:9001/logging?level=debug'

# Set all to info
curl -s -X POST 'http://localhost:9001/logging?level=info'

# Set specific component (config, connection, http, router, upstream, main, client)
curl -s -X POST 'http://localhost:9001/logging?config=debug'
curl -s -X POST 'http://localhost:9001/logging?connection=trace'
curl -s -X POST 'http://localhost:9001/logging?router=debug'
```

Ready State

```bash
curl -s http://localhost:9001/ready
```

Returns 200 when LIVE, 503 otherwise.

Drain Listeners

```bash
# Graceful drain (stop accepting new connections, finish existing)
curl -s -X POST 'http://localhost:9001/drain_listeners?graceful&skip_exit'
```


## Network-Level Inspection

All commands in this section assume `ENVOY_PID` is set:

```bash
ENVOY_PID=$(docker exec contour-worker pgrep -f "envoy.*config")
```

```bash
docker exec contour-worker nsenter --net --target $ENVOY_PID <command>
```


## Contour Metrics

```bash
curl -s http://localhost:9002/metrics
```

Key metrics:
- `contour_dagrebuild_total` — number of DAG rebuilds
- `contour_dagrebuild_seconds` — DAG rebuild duration
- `contour_httpproxy` — HTTPProxy count by namespace/status
- `contour_dag_cache_object` — cached objects by kind

## Contour Debug Port and pprof

Contour debug port is exposed on http://localhost:6060/.
To enable debug port, patch the configmap and restart the deployment:

```bash
kubectl patch configmap contour -n projectcontour --type=merge -p '{"data":{"contour.yaml":"debug: true"}}'
kubectl patch deployment contour -n projectcontour --type=json -p '[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--debug-http-address=0.0.0.0"}]'
```

To visualize the DAG in DOT format:

```bash
curl localhost:6060/debug/dag
```

Use `go tool pprof` to capture profiles

```bash
# CPU profile (30 seconds), open in browser
go tool pprof -http=:8081 'http://localhost:6060/debug/pprof/profile?seconds=30'

# Heap profile
go tool pprof -http=:8081 'http://localhost:6060/debug/pprof/heap'

# Goroutine dump (summary)
curl http://localhost:6060/debug/pprof/goroutine?debug=1

# Full stack dump (all goroutines)
curl http://localhost:6060/debug/pprof/goroutine?debug=2
```

## container-resource-exporter Metrics

See [METRICS.md](/METRICS.md) for the complete metric list. Below is a subset of commonly used metrics. Exporter at `http://localhost:8080/metrics`.


To get a quick summary of metrics from the environment run

```
go run -C examples/contour/tools . stats
```
The output shows:

- Envoy: Server version/build info, connection and request counters (downstream/upstream), HTTP response code totals, TLS stats, latency percentiles, overload indicators, and per-cluster traffic breakdowns.
- Contour: Go runtime memory/goroutine stats, DAG rebuild counts and timing, and HTTPProxy resource validation summaries.
- Cgroup resources: Per-container memory (current/peak/limit) and CPU throttling time.
- Memory RSS (smaps): Per-container memory breakdown by category (binary, heap, metadata).
- Echoserver: Basic request count and connection state.


## HTTPProxy Experimentation

Contour HTTPProxy reference: https://projectcontour.io/docs/main/config/api/


## Contour Logs

```bash
kubectl -n projectcontour logs deployment/contour -f
```

## Envoy Logs

```bash
kubectl -n projectcontour logs daemonset/envoy -c envoy -f
```

Debug logging: see the Logging section under Envoy Admin Interface.

## Grafana Dashboard

Open http://localhost:3000/ -> Dashboards -> **Contour & Envoy Observability**.

Pick Contour or Envoy pod from the Pod dropdown.

Dashboard config: `examples/contour/configs/grafana-envoy-details.json`

## Restarting

```bash
kubectl -n projectcontour delete pod -l app=envoy --force
kubectl -n projectcontour delete pod -l app=contour --force
```

Clear all collected metrics:

```bash
kubectl delete pod -l app=prometheus --force
kubectl delete pod -l app=grafana --force
kubectl delete pod -l app=container-resource-exporter
```

## Naming Conventions

Understanding Contour's naming helps navigate config dumps:

| Type | Pattern | Example |
|------|---------|---------|
| Cluster name | `namespace/service/port/hash` | `default/echoserver/80/da39a3ee5e` |
| Cluster alt_stat_name | `namespace_service_port` | `default_echoserver_80` |
| EDS service_name | `namespace/service/portname` | `default/echoserver/http` |
| Listener name | fixed | `ingress_http`, `ingress_https` |
| Route config name | same as listener | `ingress_http` |

The hash in the cluster name is derived from the cluster config. Empty config = `da39a3ee5e` (SHA1 of empty string). Adding load balancer policy, health checks etc. changes the hash.

## Running Custom Envoy Binary

Replace the Envoy container with Ubuntu and manually run a custom-built Envoy binary:

```bash
# Patch the daemonset: replace image, keep pod alive, run as root
kubectl -n projectcontour patch daemonset envoy --type='json' -p='[
  {"op":"replace","path":"/spec/template/spec/securityContext/runAsNonRoot","value":false},
  {"op":"replace","path":"/spec/template/spec/containers/1/image","value":"ubuntu:24.04"},
  {"op":"replace","path":"/spec/template/spec/containers/1/command","value":["sleep","999999999"]},
  {"op":"replace","path":"/spec/template/spec/containers/1/args","value":[]},
  {"op":"remove","path":"/spec/template/spec/containers/1/readinessProbe"},
  {"op":"add","path":"/spec/template/spec/containers/1/securityContext","value":{"runAsUser":0}}
]'

# Wait for the new pod
kubectl -n projectcontour delete pod -l app=envoy --force
kubectl -n projectcontour wait --for=condition=Ready pod -l app=envoy --timeout=60s

# Copy the custom Envoy binary into the pod
ENVOY_POD=$(kubectl -n projectcontour get pod -l app=envoy -o jsonpath='{.items[0].metadata.name}')
kubectl -n projectcontour cp /path/to/envoy-static ${ENVOY_POD}:/usr/local/bin/envoy -c envoy

# Start Envoy manually with the same config Contour normally provides
kubectl -n projectcontour exec ${ENVOY_POD} -c envoy -- \
  /usr/local/bin/envoy -c /config/envoy.json \
  --service-cluster projectcontour \
  --service-node ${ENVOY_POD} \
  --log-level info
```

To restart with a new binary, kill the process, re-copy, and start again as above.

## Running Contour on Host

Run Contour from source on the host machine while Envoy stays in the cluster. This enables IDE debugging, fast recompilation, and profiling without container overhead.

### Step 1: Scale down in-cluster Contour

```bash
kubectl -n projectcontour scale deployment --replicas=0 contour
```

### Step 2: Create Service + EndpointSlice pointing to host

Envoy connects to Contour's xDS on port 8001. Create a Service and EndpointSlice that routes to the host's IP (the kind network gateway):

```bash
HOST_IP=$(docker network inspect kind | jq -r '.[0].IPAM.Config[] | select(.Gateway | test(":") | not) | .Gateway')

kubectl apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: contour
  namespace: projectcontour
spec:
  type: ClusterIP
  ports:
  - port: 8001
    targetPort: 8001
---
apiVersion: discovery.k8s.io/v1
kind: EndpointSlice
metadata:
  name: contour-dev
  namespace: projectcontour
  labels:
    kubernetes.io/service-name: contour
addressType: IPv4
ports:
- port: 8001
endpoints:
- addresses:
  - ${HOST_IP}
  conditions:
    ready: true
EOF
```

### Step 3: Extract xDS certificates

Contour uses mTLS for the xDS gRPC connection with Envoy:

```bash
kubectl -n projectcontour get secret contourcert -o jsonpath='{..ca\.crt}' | base64 -d > ca.crt
kubectl -n projectcontour get secret contourcert -o jsonpath='{..tls\.crt}' | base64 -d > tls.crt
kubectl -n projectcontour get secret contourcert -o jsonpath='{..tls\.key}' | base64 -d > tls.key
```

### Step 4: Run Contour on host

```bash
go run github.com/projectcontour/contour/cmd/contour serve \
  --xds-address=0.0.0.0 --xds-port=8001 \
  --envoy-service-http-port=8080 --envoy-service-https-port=8443 \
  --contour-cafile=ca.crt --contour-cert-file=tls.crt --contour-key-file=tls.key \
  --debug-http-address=0.0.0.0 --debug-http-port=6061
```

### Step 5: Restart Envoy to reconnect

```bash
kubectl -n projectcontour rollout restart daemonset envoy
```

Envoy will now connect to the Contour running on the host via the EndpointSlice. You can set breakpoints, use `pprof`, or run with `-race`.
