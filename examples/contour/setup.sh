#!/bin/bash
set -e

# Change directory to the script's directory so relative paths work
cd "$(dirname "$0")"

##############################################################################
#
# Deploy Contour and Envoy
#

echo ">>> Deploying Contour and Envoy into 'projectcontour' namespace..."
kubectl apply -f https://projectcontour.io/quickstart/contour.yaml

# Wait for Contour deployment to be available before scaling and patching
echo ">>> Waiting for Contour deployment to be ready..."
kubectl rollout status deployment/contour -n projectcontour

# Scale Contour to one replica for simpler monitoring
echo ">>> Scaling Contour to one replica..."
kubectl scale deployment/contour -n projectcontour --replicas=1

echo ">>> Adding resource limits to Contour..."
# Add resource limits to Contour (for demonstration purposes)
kubectl patch deployment contour -n projectcontour --patch '{"spec": {"template": {"spec": {"containers": [{"name": "contour", "resources": {"limits": {"cpu": "100m", "memory": "128Mi"}, "requests": {"cpu": "100m", "memory": "128Mi"}}}]}}}}'

# Add resource limits to Envoy (for demonstration purposes)
echo ">>> Adding resource limits to Envoy..."
kubectl patch daemonset envoy -n projectcontour --patch '{"spec": {"template": {"spec": {"initContainers": [{"name": "envoy-initconfig", "resources": {"limits": {"cpu": "200m", "memory": "100Mi"}, "requests": {"cpu": "25m", "memory": "50Mi"}}}], "containers": [{"name": "envoy", "resources": {"limits": {"cpu": "500m", "memory": "512Mi"}, "requests": {"cpu": "500m", "memory": "512Mi"}}}, {"name": "shutdown-manager", "resources": {"limits": {"cpu": "200m", "memory": "100Mi"}, "requests": {"cpu": "25m", "memory": "50Mi"}}}]}}}}'

##############################################################################
#
# Create the necessary `ConfigMaps` for the `container-resource-exporter` and Grafana dashboards
#

# Upload configuration for container-resource-exporter
echo ">>> Creating ConfigMap for 'default/container-resource-exporter'..."
kubectl create configmap container-resource-exporter-config \
    --from-file=config.yaml=configs/exporter.yaml \
    --dry-run=client -o yaml | kubectl apply -f -

# Upload configuration for Prometheus
echo ">>> Creating ConfigMap for 'default/prometheus-config'..."
kubectl create configmap prometheus-config \
    --from-file=configs/prometheus.yml \
    --dry-run=client -o yaml | kubectl apply -f -

# Upload Grafana dashboards.
echo ">>> Creating ConfigMap for 'default/grafana-dashboards'..."
kubectl create configmap grafana-dashboards \
    --from-file=configs/grafana-envoy-details.json \
    --dry-run=client -o yaml | kubectl apply -f -

##############################################################################
#
# Deploy the observability stack
#

# Deploy container-resource-exporter, Prometheus, Grafana, etc.
echo ">>> Deploying container-resource-exporter to 'default' namespace..."
kubectl apply -f ../../manifests/container-resource-exporter.yaml
echo ">>> Deploying Prometheus to 'default' namespace..."
kubectl apply -f manifests/prometheus.yaml
echo ">>> Deploying Grafana to 'default' namespace..."
kubectl apply -f manifests/grafana.yaml

# Expose echoserver and container-resource-exporter for access from host
echo ">>> Exposing services..."
kubectl apply -f manifests/exposure.yaml

##############################################################################
#
# Deploy Example Workload
#

# Deploy an echoserver backend:
echo ">>> Deploying echoserver workload to 'default' namespace..."
kubectl apply -f https://raw.githubusercontent.com/tsaarni/echoserver/refs/heads/main/manifests/echoserver.yaml
