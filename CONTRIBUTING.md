# Contributing

This guide is for those who wish to contribute to the project.

## Prerequisites

- Go installed on your system.
- Docker and kubectl available.
- A working Kubernetes cluster (e.g. Kind) with cluster-admin access.

## Development Setup

### Build the Project

```bash
make
```

### Create a Test Kubernetes Cluster

```bash
make kind-create
```

The [kind-cluster-config.yaml](examples/kind/kind-cluster-config.yaml) file configures the test cluster to:
- Bind to the local address `127.0.0.136` for accessing services from your host machine.
- Expose the following ports:
  - **80**: HTTP traffic for Contour/Envoy
  - **443**: HTTPS traffic for Contour/Envoy
  - **3000**: Grafana dashboard
  - **8080**: Metrics endpoint for `container-resource-exporter`

This configuration allows you to access services using DNS names with [nip.io](https://nip.io/) (e.g., `http://grafana.127.0.0.136.nip.io:3000/`).

To delete the cluster when done:

```bash
make kind-delete
```

## Deploying to the Cluster

**Step 1:** Create configuration `ConfigMaps`

```bash
kubectl create configmap container-resource-exporter-config --from-file=config.yaml=examples/contour/contour-and-envoy-metrics.yaml --dry-run=client -o yaml | kubectl apply -f -
kubectl create configmap grafana-dashboards --from-file=examples/contour/grafana-dashboard-envoy.json --dry-run=client -o yaml | kubectl apply -f -
```

**Step 2:** Deploy the applications

This deploys the `container-resource-exporter`, Prometheus, and Grafana:

```bash
kubectl apply -f manifests/
```

**Step 3:** Build and load container image

After making code changes, rebuild, load the new image to the Kind cluster, and restart the exporter pods to pick up the changes:

```bash
make container
make kind-load
kubectl delete pod -l app=container-resource-exporter
```


## Testing with Example Workload

**Step 1:** Deploy Contour and Envoy

```bash
kubectl apply -f https://projectcontour.io/quickstart/contour.yaml

# Scale Contour to one replica for simplicity of monitoring.
kubectl scale deployment/contour -n projectcontour --replicas=1
```

**Step 2:** Deploy echoserver backend service and create HTTPProxy to route traffic to it.

```bash
kubectl apply -f https://raw.githubusercontent.com/tsaarni/echoserver/refs/heads/main/manifests/echoserver.yaml
kubectl apply -f examples/contour/echoserver-httproxy.yaml
```

**Step 3:** Access dashboards and metrics

- Grafana Dashboard: http://grafana.127.0.0.136.nip.io:3000/
- Metrics Endpoint: http://exporter.127.0.0.136.nip.io:8080/metrics
- Echoserver Service: http://echoserver.127.0.0.136.nip.io/


**Step 4:** Generate load

Generate traffic to observe metric changes:

```bash
go run github.com/tsaarni/echoclient/cmd/echoclient@latest get -url http://echoserver.127.0.0.136.nip.io/ -concurrency 100 -duration 30s
```

**Step 5:** Reset data (optional)

After making code or configuration changes, you may want to reset Prometheus data and restart the relevant pods.

```bash
kubectl delete pod -l app=prometheus
kubectl delete pod -l app=envoy -n projectcontour
kubectl delete pod -l app=container-resource-exporter
```
