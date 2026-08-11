![Container Resource Exporter logo light](./examples/cre-logo-light.png#gh-light-mode-only)
![Container Resource Exporter logo dark](./examples/cre-logo-dark.png#gh-dark-mode-only)


A Prometheus-compatible metrics exporter that monitors container resource usage of Kubernetes workloads.

This exporter collects metrics by leveraging:
- cgroup v2 for CPU, memory, PID limits and usage, and block I/O statistics
- `/proc/[pid]/smaps` for per-process memory mapping statistics
- `/proc/[pid]/io` for per-process I/O statistics
- `/proc/[pid]/mountinfo` and `statfs` for filesystem disk usage
- `/proc/[pid]/root` and `stat` for file sizes and disk usage

See [documentation](METRICS.md) for a full list of supported metrics.
This exporter was created because other existing solutions did not provide all of these metrics.


## Demo

For complete examples of using the exporter with Prometheus and Grafana, see:
- [`examples/contour`](examples/contour) for monitoring Envoy proxy resource usage.
- [`examples/openbao`](examples/openbao) for monitoring OpenBao, including its database and storage size.

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
| `filters[].collect.mountpoints` | Optional list of disk mountpoints inside the container to monitor | — |
| `filters[].collect.files` | Optional list of file paths inside the container to monitor (supports wildcards like `*` and `**`) | — |

<sup>1</sup> The `command` filter is based on the process name from `/proc/[pid]/comm`, which is limited to the first 15 characters of the executable name.

For a complete example, see [`examples/example-config.yaml`](examples/example-config.yaml).

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
kubectl create configmap container-resource-exporter-config --from-file=examples/example-config.yaml
kubectl apply -f manifests/container-resource-exporter.yaml
```

## Contributing

Please refer to [CONTRIBUTING.md](CONTRIBUTING.md).
