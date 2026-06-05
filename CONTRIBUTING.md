# Contributing

This guide is for those who wish to contribute to the project.

## Prerequisites

- Go installed on your system.
- Docker and kubectl available.
- Kubernetes cluster with cluster-admin access, this example uses [Kind](https://kind.sigs.k8s.io/) for local testing.

## Development

Build the project

```bash
make
```


Run linters:

```bash
make lint
```

Create Kubernetes cluster:

```bash
make kind-create
```

Deploy full observability stack, see the [examples/contour/README.md](examples/contour/README.md) for further information.


```bash
make kind-setup
```

After making code changes:

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

Delete the cluster when done:

```bash
make kind-delete
```

