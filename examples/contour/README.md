# Contour and Envoy Monitoring Example

This example demonstrates how to monitor Contour and Envoy using `container-resource-exporter`.

## Prerequisites

- [Kind](https://kind.sigs.k8s.io/) installed.
- [kubectl](https://kubernetes.io/docs/tasks/tools/) installed.

## Deployment Steps

### 1. Create a Kind Cluster

Use the provided cluster configuration:

```bash
kind create cluster --config configs/kind.yaml --name container-resource-exporter
```

### 2. Deploy Contour and Envoy

```bash
kubectl apply -f https://projectcontour.io/quickstart/contour.yaml

# Scale Contour to one replica for simpler monitoring
kubectl scale deployment/contour -n projectcontour --replicas=1

# Add resource limits to Contour
kubectl patch deployment contour -n projectcontour --patch '{"spec": {"template": {"spec": {"containers": [{"name": "contour", "resources": {"limits": {"cpu": "100m", "memory": "128Mi"}, "requests": {"cpu": "100m", "memory": "128Mi"}}}]}}}}'

# Add resource limits to Envoy
kubectl patch daemonset envoy -n projectcontour --patch '{"spec": {"template": {"spec": {"initContainers": [{"name": "envoy-initconfig", "resources": {"limits": {"cpu": "200m", "memory": "100Mi"}, "requests": {"cpu": "25m", "memory": "50Mi"}}}], "containers": [{"name": "envoy", "resources": {"limits": {"cpu": "500m", "memory": "512Mi"}, "requests": {"cpu": "500m", "memory": "512Mi"}}}, {"name": "shutdown-manager", "resources": {"limits": {"cpu": "200m", "memory": "100Mi"}, "requests": {"cpu": "25m", "memory": "50Mi"}}}]}}}}'
```

### 3. Configure and Deploy the Observability Stack

Create the necessary `ConfigMaps` for the `container-resource-exporter` and Grafana dashboards, then deploy the observability stack:

```bash
# Upload configuration for container-resource-exporter
kubectl create configmap container-resource-exporter-config \
    --from-file=config.yaml=configs/exporter.yaml \
    --dry-run=client -o yaml | kubectl apply -f -

# Upload configuration for Prometheus
kubectl create configmap prometheus-config \
    --from-file=configs/prometheus.yml \
    --dry-run=client -o yaml | kubectl apply -f -

# Upload Grafana dashboards.
kubectl create configmap grafana-dashboards \
    --from-file=configs/grafana-envoy-details.json \
    --dry-run=client -o yaml | kubectl apply -f -

# Deploy container-resource-exporter, Prometheus, Grafana, etc.
kubectl apply -f ../../manifests/container-resource-exporter.yaml
kubectl apply -f manifests/prometheus.yaml
kubectl apply -f manifests/grafana.yaml

# Expose echoserver and container-resource-exporter for access from host
kubectl apply -f manifests/exposure.yaml
```

### 4. Deploy Example Workload

Deploy an echoserver backend:

```bash
kubectl apply -f https://raw.githubusercontent.com/tsaarni/echoserver/refs/heads/main/manifests/echoserver.yaml
```

## Accessing the Dashboards

Once all pods are running, you can access the following services:

- **Grafana**: [http://localhost:3000/](http://localhost:3000/)
  - Login with `admin`/`admin`.
  - The "Envoy Resource Details" dashboard provides deep insights into Envoy's memory usage.
- **Echoserver**: [http://echoserver.127.0.0.1.nip.io/](http://echoserver.127.0.0.1.nip.io/)
- **container-resource-exporter**: [http://localhost:8080/metrics](http://localhost:8080/metrics)

## Generating Load

To see the metrics in action, generate some traffic:

```bash
go run github.com/tsaarni/echoclient/cmd/echoclient@latest get -url http://echoserver.127.0.0.1.nip.io/ -concurrency 100 -duration 60s
```
