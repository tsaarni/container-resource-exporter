#!/bin/bash
set -e

# Change directory to the script's directory so relative paths work
cd "$(dirname "$0")"

##############################################################################
#
# Deploy Contour and Envoy
#

kubectl apply -f https://projectcontour.io/quickstart/contour.yaml

# Wait for Contour deployment to be available before scaling and patching
kubectl rollout status deployment/contour -n projectcontour

# Scale Contour to one replica for simpler monitoring
kubectl scale deployment/contour -n projectcontour --replicas=1

# Add resource limits to Contour (for demonstration purposes)
kubectl patch deployment contour -n projectcontour --patch '{"spec": {"template": {"spec": {"containers": [{"name": "contour", "resources": {"limits": {"cpu": "100m", "memory": "128Mi"}, "requests": {"cpu": "100m", "memory": "128Mi"}}}]}}}}'

# Add resource limits to Envoy (for demonstration purposes)
kubectl patch daemonset envoy -n projectcontour --patch '{"spec": {"template": {"spec": {"initContainers": [{"name": "envoy-initconfig", "resources": {"limits": {"cpu": "200m", "memory": "100Mi"}, "requests": {"cpu": "25m", "memory": "50Mi"}}}], "containers": [{"name": "envoy", "resources": {"limits": {"cpu": "500m", "memory": "512Mi"}, "requests": {"cpu": "500m", "memory": "512Mi"}}}, {"name": "shutdown-manager", "resources": {"limits": {"cpu": "200m", "memory": "100Mi"}, "requests": {"cpu": "25m", "memory": "50Mi"}}}]}}}}'

##############################################################################
#
# Create the necessary `ConfigMaps` for the `container-resource-exporter` and Grafana dashboards
#

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

##############################################################################
#
# Deploy the observability stack
#

# Deploy container-resource-exporter, Prometheus, Grafana, etc.
kubectl apply -f ../../manifests/container-resource-exporter.yaml
kubectl apply -f manifests/prometheus.yaml
kubectl apply -f manifests/grafana.yaml

# Expose echoserver and container-resource-exporter for access from host
kubectl apply -f manifests/exposure.yaml

##############################################################################
#
# Deploy Example Workload
#

# Deploy an echoserver backend:
kubectl apply -f https://raw.githubusercontent.com/tsaarni/echoserver/refs/heads/main/manifests/echoserver.yaml
