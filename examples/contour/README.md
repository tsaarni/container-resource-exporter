# Contour and Envoy Monitoring Example

This example demonstrates how to monitor Contour and Envoy using `container-resource-exporter`.

## Prerequisites

- [Kind](https://kind.sigs.k8s.io/) installed.
- [kubectl](https://kubernetes.io/docs/tasks/tools/) installed.

## Deployment Steps

Create a Kind Cluster

```bash
kind create cluster --config configs/kind.yaml --name container-resource-exporter
```

Run the [setup.sh](setup.sh) script to deploy Contour, Envoy, and the observability stack (Prometheus, Grafana, and the exporter).


```bash
./setup.sh
```

## Accessing the Dashboards

Once all pods are running, you can access the following services:

- **Grafana**: [http://localhost:3000/](http://localhost:3000/)
  - Click "Dashboards" > "Contour & Envoy Observability" to view the dashboard.
  - Pick Contour or Envoy pod from the Pod dropdown to see metrics for that specific pod.
- **Echoserver**: [http://echoserver.127.0.0.1.nip.io/](http://echoserver.127.0.0.1.nip.io/)
- **container-resource-exporter**: [http://localhost:8080/metrics](http://localhost:8080/metrics)

## Generating Load

To see the metrics in action, generate some traffic:

```bash
go run github.com/tsaarni/echoclient/cmd/echoclient@latest get -url http://echoserver.127.0.0.1.nip.io/ -concurrency 100 -duration 60s
```
