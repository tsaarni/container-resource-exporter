#!/bin/bash
set -e

cd "$(dirname "$0")"

CLUSTER_NAME=openbao

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
# Generate certificates and deploy services
#

echo ">>> Generating certificates..."
mkdir -p certs
certyaml -d certs configs/certs.yaml

echo ">>> Deploying OpenBao..."
kubectl apply -f manifests/openbao.yaml

echo ">>> Deploying container-resource-exporter..."
kubectl apply -f manifests/container-resource-exporter.yaml

echo ">>> Deploying Prometheus..."
kubectl apply -f manifests/prometheus.yaml

echo ">>> Deploying Grafana..."
kubectl apply -f manifests/grafana.yaml

echo ">>> Exposing services..."
kubectl apply -f manifests/exposure.yaml

##############################################################################
#
# Initialize and unseal OpenBao
#

echo ">>> Waiting for OpenBao pods to be running..."
kubectl wait --for=jsonpath='{.status.phase}'=Running pod/openbao-0 pod/openbao-1 pod/openbao-2 --timeout=60s

echo ">>> Initializing OpenBao..."
until kubectl exec openbao-0 -- bao operator init -key-shares=1 -key-threshold=1 -format=json > init.json 2>/dev/null; do
  sleep 2
done

UNSEAL_KEY=$(jq -r '.unseal_keys_b64[0]' init.json)

echo ">>> Unsealing OpenBao leader..."
kubectl exec openbao-0 -- bao operator unseal "$UNSEAL_KEY" > /dev/null

echo ">>> Waiting for followers to join..."
sleep 5

echo ">>> Unsealing OpenBao followers..."
kubectl exec openbao-1 -- bao operator unseal "$UNSEAL_KEY" > /dev/null
kubectl exec openbao-2 -- bao operator unseal "$UNSEAL_KEY" > /dev/null

BAO_TOKEN=$(jq -r '.root_token' init.json)

echo ">>> Configuring secrets engines and auth methods..."
# Enable KV v2 at kv/ (KV v1 remains at default secret/)
kubectl exec openbao-0 -- sh -c "BAO_TOKEN=$BAO_TOKEN bao secrets enable -version=2 -path=kv kv"
kubectl exec openbao-0 -- sh -c "BAO_TOKEN=$BAO_TOKEN bao write kv/config max_versions=3"

# Enable transit engine and create a default key
kubectl exec openbao-0 -- sh -c "BAO_TOKEN=$BAO_TOKEN bao secrets enable transit"
kubectl exec openbao-0 -- sh -c "BAO_TOKEN=$BAO_TOKEN bao write -f transit/keys/default"

# Enable cert auth and register the CA for client certificate login
kubectl exec openbao-0 -- sh -c "BAO_TOKEN=$BAO_TOKEN bao auth enable cert"
kubectl exec openbao-0 -- sh -c "BAO_TOKEN=$BAO_TOKEN bao write auth/cert/certs/default certificate=@/host/certs/ca.pem policies=default token_type=batch"

##############################################################################
#
# Done
#

cat <<'EOF'

Setup complete!

  Grafana:     http://localhost:3000/
  Prometheus:  http://localhost:9090/
  Exporter:    http://localhost:8080/metrics

  openbao-0:   https://localhost:8200
  openbao-1:   https://localhost:8201
  openbao-2:   https://localhost:8202

Use following settings:

  export BAO_ADDR=https://127.0.0.1:8200
  export UNSEAL_KEY=$(jq -r '.unseal_keys_b64[0]' init.json)
  export BAO_TOKEN=$(jq -r '.root_token' init.json)

To generate load:

  go run traffic.go
EOF
