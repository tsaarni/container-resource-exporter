# Contributing

This guide is for those who wish to contribute to the project.

## Prerequisites

- Go installed on your system.
- Docker and kubectl available.
- Kubernetes cluster with cluster-admin access, this example uses [Kind](https://kind.sigs.k8s.io/) for local testing.

## Development Setup

### Build the Project

```bash
make
```

### Linting

Run linters to ensure code quality:

```bash
make lint
```

### Create a Test Kubernetes Cluster

```bash
make kind-create
```

To delete the cluster when done:

```bash
make kind-delete
```

### Setup Observability Stack in Kind Cluster

```bash
make kind-setup
```

### Make Code Changes and Test

Re-build the container image, load it into the Kind cluster, and restart the exporter pods:

```bash
make container
make kind-load
```

`make kind-load` automatically loads the image into the Kind cluster and performs a restart of the DaemonSet.

After making code or configuration changes, you may want to reset Prometheus data and restart all pods at the same time to ensure a clean state:

```bash
kubectl delete pod -l app=container-resource-exporter
kubectl delete pod -l app=envoy -n projectcontour
kubectl delete pod -l app=contour -n projectcontour
kubectl delete pod -l app=prometheus
kubectl delete pod -l app=grafana
```

See the [examples/contour/README.md](examples/contour/README.md) for further information.

## Pull Requests

- Create a descriptive branch name for your changes.
- Ensure `make lint` passes before submitting.
- Add unit tests for new features or bug fixes whenever possible.
- Provide a clear description of the changes in your pull request.
