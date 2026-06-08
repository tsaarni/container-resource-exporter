#!/bin/bash
set -e

cd "$(dirname "$0")"

CLUSTER_NAME=contour

##############################################################################
#
# Create Kind cluster
#

echo ">>> Creating kind cluster..."
if ! kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  kind create cluster --name "$CLUSTER_NAME" --config configs/kind.yaml
else
  echo "Cluster '$CLUSTER_NAME' already exists, skipping creation."
fi

##############################################################################
#
# Deploy Contour and Envoy
#

echo ">>> Deploying Contour and Envoy into 'projectcontour' namespace..."
kubectl apply -f https://projectcontour.io/quickstart/contour.yaml

# Scale Contour to one replica for simpler monitoring
echo ">>> Scaling Contour to one replica..."
kubectl scale deployment/contour -n projectcontour --replicas=1

echo ">>> Adding resource limits to Contour..."
kubectl patch deployment contour -n projectcontour --patch '{"spec": {"template": {"spec": {"containers": [{"name": "contour", "resources": {"limits": {"cpu": "100m", "memory": "128Mi"}, "requests": {"cpu": "100m", "memory": "128Mi"}}}]}}}}'

echo ">>> Adding resource limits to Envoy..."
kubectl patch daemonset envoy -n projectcontour --patch '{"spec": {"template": {"spec": {"initContainers": [{"name": "envoy-initconfig", "resources": {"limits": {"cpu": "200m", "memory": "100Mi"}, "requests": {"cpu": "25m", "memory": "50Mi"}}}], "containers": [{"name": "envoy", "resources": {"limits": {"cpu": "500m", "memory": "512Mi"}, "requests": {"cpu": "500m", "memory": "512Mi"}}}, {"name": "shutdown-manager", "resources": {"limits": {"cpu": "200m", "memory": "100Mi"}, "requests": {"cpu": "25m", "memory": "50Mi"}}}]}}}}'

# NOTE: An alternative would be `kubectl debug --custom` with an ephemeral container,
# but ephemeral containers don't survive pod restarts so a sidecar is used instead.
echo ">>> Adding socat sidecar for Envoy admin access..."
kubectl patch daemonset envoy -n projectcontour --type=strategic --patch '
{
  "spec": {
    "template": {
      "spec": {
        "containers": [{
          "name": "admin-proxy",
          "image": "alpine/socat:1.8.0.1",
          "args": ["TCP-LISTEN:9001,fork,reuseaddr", "UNIX-CONNECT:/admin/admin.sock"],
          "ports": [{"containerPort": 9001, "name": "admin", "protocol": "TCP"}],
          "volumeMounts": [{"name": "envoy-admin", "mountPath": "/admin"}],
          "resources": {"limits": {"cpu": "50m", "memory": "32Mi"}, "requests": {"cpu": "10m", "memory": "16Mi"}}
        }]
      }
    }
  }
}'

##############################################################################
#
# Deploy the observability stack
#

echo ">>> Deploying container-resource-exporter..."
kubectl apply -f manifests/container-resource-exporter.yaml

echo ">>> Deploying Prometheus..."
kubectl apply -f manifests/prometheus.yaml

echo ">>> Deploying Grafana..."
kubectl apply -f manifests/grafana.yaml

##############################################################################
#
# Deploy Example Workload
#

echo ">>> Generating certificates..."
mkdir -p certs
go run github.com/tsaarni/certyaml/cmd/certyaml@latest -d certs configs/certs.yaml

echo ">>> Generating JWKS..."
go run -C traffic . jwks generate

echo ">>> Deploying JWKS server..."
kubectl apply -f manifests/jwks-server.yaml

echo ">>> Deploying echoserver workload..."
kubectl apply -f manifests/echoserver.yaml

echo ">>> Exposing services..."
kubectl apply -f manifests/exposure.yaml

##############################################################################
#
# Wait for all pods to be ready
#

echo ">>> Waiting for all pods to be ready..."
kubectl rollout status -n projectcontour deployment/contour daemonset/envoy

##############################################################################
#
# Done
#

cat <<'EOF'

Setup complete!

  Grafana:          http://localhost:3000/
  Prometheus:       http://localhost:9090/
  Exporter:         http://localhost:8080/metrics
  Envoy Admin:      http://localhost:9001/
  Contour Metrics:  http://localhost:9002/metrics
  Contour Debug:    http://localhost:6060/debug/pprof/

  Echoserver (HTTP):        http://echoserver.127.0.0.1.nip.io/
  Echoserver (HTTPS):       https://echoserver-tls.127.0.0.1.nip.io/
  Echoserver (cert auth):   https://echoserver-cert-auth.127.0.0.1.nip.io/
  Echoserver (JWT):         https://echoserver-jwt.127.0.0.1.nip.io/

EOF
