# Runbook for OpenBao Example Environment

Operational reference for working with the example environment.
See [README.md](README.md) for initial setup and deployment instructions.

## Credentials

Root token and unseal keys are written to `init.json` by `setup.sh`. Set these once per shell session:

```bash
export BAO_TOKEN=$(jq -r '.root_token' examples/openbao/init.json)
export BAO_UNSEAL_KEY=$(jq -r '.unseal_keys_b64[0]' examples/openbao/init.json)
```

## Running bao Commands

The cluster has 3 pods: `openbao-0`, `openbao-1`, `openbao-2`. `BAO_ADDR` and `BAO_CACERT` are pre-configured in the pod environment. Only `BAO_TOKEN` needs to be supplied.

```bash
# Cluster status
kubectl exec openbao-0 -- sh -c "BAO_TOKEN=$BAO_TOKEN bao status"

# Cluster health
kubectl exec openbao-0 -- sh -c "BAO_TOKEN=$BAO_TOKEN bao read sys/health"

# Runtime metrics (heap, goroutines, GC)
kubectl exec openbao-0 -- sh -c "BAO_TOKEN=$BAO_TOKEN bao read sys/metrics"

# Go runtime stats as JSON (alloc_bytes, heap_objects, num_goroutines, gc)
kubectl exec openbao-0 -- sh -c "BAO_TOKEN=$BAO_TOKEN bao read -format=json sys/metrics" | \
  jq '.data.Gauges[] | select(.Name | test("runtime")) | {(.Name): .Value}'

# List secrets
kubectl exec openbao-0 -- sh -c "BAO_TOKEN=$BAO_TOKEN bao kv list secret/"

# Count secrets
kubectl exec openbao-0 -- sh -c "BAO_TOKEN=$BAO_TOKEN bao kv list secret/ | wc -l"

# Read a secret
kubectl exec openbao-0 -- sh -c "BAO_TOKEN=$BAO_TOKEN bao kv get secret/key-1"
```

## Raft Leadership

```bash
# Check leader/follower status
kubectl exec openbao-0 -- sh -c "BAO_TOKEN=$BAO_TOKEN bao operator raft list-peers"

# Force leader to step down (triggers re-election)
kubectl exec openbao-0 -- sh -c "BAO_TOKEN=$BAO_TOKEN bao operator step-down"
```

## Raft Snapshots

Must be run on the leader pod (see Raft Leadership above to identify it):

```bash
# Save snapshot
kubectl exec openbao-1 -- sh -c "BAO_TOKEN=$BAO_TOKEN bao operator raft snapshot save /tmp/raft-snapshot.bao"

# Restore snapshot
kubectl exec openbao-1 -- sh -c "BAO_TOKEN=$BAO_TOKEN bao operator raft snapshot restore /tmp/raft-snapshot.bao"
```

## Raft Storage Inspection (raft-inspector)

OpenBao uses two BoltDB databases: `raft.db` stores the raft log (write-ahead log of operations), `vault.db` stores the FSM state (actual secret data and metadata). Log compaction and snapshots periodically truncate raft.db.

Note: after traffic stops, snapshots, GC, and log compaction may still run. Wait 10-30 seconds before collecting final measurements.

Offline inspection of `raft.db` and `vault.db` directly from the pod's data directory. Works while the server is running. See https://github.com/tsaarni/raft-inspector and https://github.com/tsaarni/raft-inspector/blob/main/raft-inspector.md for full documentation.

```bash
# Copy data directory from a pod
kubectl cp openbao-0:/openbao/data /tmp/openbao-data-0

# Combined health overview: log indices, cluster membership, BoltDB free pages, space efficiency
go run github.com/tsaarni/raft-inspector@latest status /tmp/openbao-data-0

# Raft log statistics: op distribution, hot keys, size
go run github.com/tsaarni/raft-inspector@latest log /tmp/openbao-data-0 --stats \
    --unseal-key-file examples/openbao/init.json

# FSM state: key counts per path segment, largest keys
go run github.com/tsaarni/raft-inspector@latest fsm /tmp/openbao-data-0 \
    --unseal-key-file examples/openbao/init.json
```

## Prometheus Metrics (from host)

Requires `telemetry` block in `configs/openbao.hcl` (already configured).

```bash
CA=examples/openbao/certs/ca.pem

curl -s --cacert $CA "https://localhost:8200/v1/sys/metrics?format=prometheus" \
  -H "X-Vault-Token: $BAO_TOKEN" | grep heap
```

## container-resource-exporter Metrics (from host)

See [METRICS.md](/METRICS.md) for all collected metrics. The exporter is at `http://localhost:8080/metrics`.

```bash
# Quick check: Go heap RSS per pod
curl -s http://localhost:8080/metrics | grep 'process_smaps_rss_bytes.*Go: heap"'
```

## Combined Stats Summary

Prints memory (smaps RSS/PSS), disk usage, cgroup memory, CPU throttling, and OpenBao Go runtime stats for all pods:

```bash
go run -C examples/openbao/traffic . stats
```

## Traffic Generator

All subcommands support `--rps` (default 200), `--concurrency` (default 10), and `--count` (default 1000, 0=infinite).

```bash
# Write 1000 secrets of 10 KB each (~10 MB stored data).
# Adjust --count and --size to control how much data is written.
go run -C examples/openbao/traffic . kv-write

# Read random secrets from a pool of 500 keys at 200 RPS.
# Adjust --pool to control how many distinct keys are read.
go run -C examples/openbao/traffic . kv-read

# Delete secrets written by kv-write (keys named key-1..key-N).
go run -C examples/openbao/traffic . bulk-delete

# Encrypt+decrypt 10 KB payloads via transit engine (no disk I/O).
# Adjust --size for different payload sizes.
go run -C examples/openbao/traffic . transit

# Login using TLS client certificate (batch tokens, no storage).
go run -C examples/openbao/traffic . cert-login

# Login with service tokens (causes storage writes per login).
go run -C examples/openbao/traffic . cert-login --token-type service

# Mixed workload on KV v2: read/write/delete/transit/login.
# Default mix: read=70,write=10,delete=10,transit=5,login=5.
# Adjust --mix, --duration (e.g. 5m), --size, --pool.
go run -C examples/openbao/traffic . mix

# Show all subcommands and global flags
go run -C examples/openbao/traffic . --help
```

**Token types**: `cert-login` and `mix` support `--token-type batch` (default) and `--token-type service`.
Batch tokens are self-contained encrypted blobs with no storage cost — purely computational.
Service tokens are heavyweight: each login causes multiple storage writes (token tracked on disk), making them useful for exercising disk I/O under load.
Service tokens are automatically revoked after their TTL expires (system default is 32 days, configured to 5 minutes in `setup.sh`).
List active service tokens with `bao list auth/token/accessors` (batch tokens are not tracked and won't appear).

**KV v2 versioning**: The `kv/` engine keeps up to `max_versions` (3) versions per key. Writing to an existing key adds a new version; when the limit is exceeded the oldest version is permanently deleted. The default without our override is 10 versions. The `mix` command's delete operation uses `DELETE /kv/metadata/:path` which fully removes the key and all its versions from storage.

## Grafana Dashboard

The Grafana dashboard is defined in `examples/openbao/configs/grafana-openbao-details.json`.
It is loaded into Grafana automatically via the ConfigMap in `examples/openbao/configs/grafana-dashboards.yaml`.

Open at: http://localhost:3000/ -> Dashboards -> **OpenBao Resource Monitoring**

## OpenBao Configuration

The config file is at `examples/openbao/configs/openbao.hcl` (mounted into pods via hostPath at `/host`).

## Restarting

Data is stored on tmpfs (`emptyDir` with `medium: Memory`, see `manifests/`), so deleting pods destroys all data. To restart without losing data, kill the process instead — the container restarts but the pod (and its tmpfs) survives. You will need to unseal afterwards.

To change configuration (e.g. TTL, policies, auth roles), just update the settings on the running cluster — no restart needed.

```bash
# Restart without losing data (repeat for each pod)
kubectl exec openbao-0 -- sh -c "kill 1"
sleep 3
kubectl exec openbao-0 -- sh -c "bao operator unseal $BAO_UNSEAL_KEY"
```

## Wipe All Data

Deletes all pods, destroying all stored data (tokens, secrets, leases, raft state). Use only when you want to start from a completely empty state.

```bash
kubectl delete pod -l app=openbao --force
kubectl wait --for=jsonpath='{.status.phase}'=Running pod/openbao-0 pod/openbao-1 pod/openbao-2 --timeout=60s
examples/openbao/setup.sh
```

The setup script initializes, unseals, and enables secret engines and auth methods needed by the traffic generator — see `setup.sh` for details.

## Resetting Metrics and Dashboard

Restarts Prometheus, Grafana, and the exporter to clear all collected metrics and dashboard state:

```bash
kubectl delete pod -l app=prometheus --force
kubectl delete pod -l app=grafana --force
kubectl delete pod -l app=container-resource-exporter
```
