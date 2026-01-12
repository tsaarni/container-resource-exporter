# Container Resource Exporter

A Prometheus-compatible metrics exporter that monitors container resource usage of Kubernetes workloads.

## Overview

This exporter collects detailed resource usage statistics for containers by leveraging:
- cgroup v2 for container resource metrics
- `/proc/[pid]/smaps` for memory mapping statistics

See [documentation](METRICS.md) for a full list of supported metrics.
This exporter was created because other existing solutions did not provide all needed cgroup v2 and smaps metrics.


## Configuration

Provide configuration using a YAML file and specify it with the `-config` command line argument:

```bash
container-resource-exporter -config /path/to/config.yaml
```

### Configuration Options

The `config.yaml` file supports the following options:

| Field Name | Description | Default Value |
|---|---|---|
| `server.address` | Server listen address and port | `:8080` |
| `paths.cgroup` | Path to cgroup v2 filesystem | `/sys/fs/cgroup` |
| `paths.proc` | Path to proc filesystem | `/proc` |
| `paths.cri_socket` | Path to CRI socket for container discovery | Auto-detected from `/run/containerd/containerd.sock`, `/run/crio/crio.sock`, or `/run/cri-dockerd.sock` |
| `scrape_interval` | Interval for collecting metrics (Go duration format) | `1s` |
| `log_level` | Logging level (debug, info, warn, error) | `info` |
| `filters` | List of container filters to monitor | Required; at least one filter must be specified |
| `filters[].namespace` | Kubernetes namespace pattern (supports `*` wildcard) | — |
| `filters[].pod` | Pod name pattern (supports `*` wildcard) | — |
| `filters[].container` | Container name pattern (supports `*` wildcard) | — |
| `filters[].command` | Process command pattern (supports `*` wildcard) <sup>1</sup> | `*` (matches all commands) |

<sup>1</sup> The `command` filter is based on the process name from `/proc/[pid]/comm`, which is limited to the first 15 characters of the executable name.

For a complete example, see [`examples/config.yaml`](examples/config.yaml).

## Building

To build the project from source, ensure you have Go installed and run:

```bash
make
```

## Deployment

### Container Image

A pre-built container image is available at:
```
ghcr.io/tsaarni/container-resource-exporter:latest
```

### Kubernetes Deployment

The [`manifests/container-resource-exporter.yaml`](manifests/container-resource-exporter.yaml) file contains Kubernetes manifest for deploying the exporter.
Note that `container-resource-exporter` needs to run as root and following host paths need to be mounted into the container:

- `/sys/fs/cgroup` for cgroup v2 filesystem access.
- `/proc` for process information filesystem.
- CRI socket path e.g., `/run/containerd/containerd.sock` for container discovery.

To deploy with provided example manifest, run:

```bash
kubectl create configmap container-resource-exporter-config --from-file=examples/config.yaml
kubectl apply -f manifests/manifests/container-resource-exporter.yaml
```

To see that the metrics are being collected, port-forward the exporter service and access the metrics endpoint:

```bash
kubectl port-forward daemonset/container-resource-exporter 8080:8080
curl http://localhost:8080/metrics
```

The other manifests in [`manifests/`](manifests/) provide a simple example for full observability stack with Prometheus and Grafana, see [CONTRIBUTING.md](CONTRIBUTING.md) for example on how to use them in a local Kind cluster.

## Contributing

Please refer to [CONTRIBUTING.md](CONTRIBUTING.md).
