#!/bin/bash
set -e

cd "$(dirname "$0")"

CLUSTER_NAME=openbao
VOLUME_SIZE=${VOLUME_SIZE:-256M}

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
# Create loop-backed ext4 volumes for OpenBao data
#

echo ">>> Setting up loop-backed ext4 volumes..."
KIND_NODE="${CLUSTER_NAME}-worker"
docker exec "$KIND_NODE" sh -c '
  mkdir -p /openbao-volumes
  for pod in openbao-0 openbao-1 openbao-2; do
    IMG=/openbao-volumes/${pod}.img
    MNT=/openbao-volumes/${pod}
    mkdir -p "$MNT"
    # Skip if already mounted
    if mountpoint -q "$MNT" 2>/dev/null; then
      continue
    fi
    # Create image if it does not exist
    if [ ! -f "$IMG" ]; then
      truncate -s '"$VOLUME_SIZE"' "$IMG"
      mkfs.ext4 -q "$IMG"
    fi
    mount -o loop "$IMG" "$MNT"
  done
'

##############################################################################
#
# Generate certificates and deploy services
#

echo ">>> Generating certificates..."
mkdir -p certs
go run github.com/tsaarni/certyaml/cmd/certyaml@latest -d certs configs/certs.yaml

echo ">>> Deploying OpenBao..."
if [ -n "$OPENBAO_IMAGE" ]; then
  echo "    Using custom image: $OPENBAO_IMAGE"
  sed "s|image: .*|image: $OPENBAO_IMAGE|" manifests/openbao.yaml | kubectl apply -f -
else
  kubectl apply -f manifests/openbao.yaml
fi

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

bao_exec() {
  kubectl exec openbao-0 -- sh -c "BAO_TOKEN=$BAO_TOKEN $*"
}

wait_for_pod() {
  kubectl wait --for=jsonpath='{.status.phase}'=Running pod/"$1" --timeout=300s
}

# Only init if not already initialized. Credentials are saved to init.json.
echo ">>> Initializing OpenBao..."
wait_for_pod openbao-0
if ! kubectl exec openbao-0 -- bao status -format=json 2>/dev/null | jq -e '.initialized' > /dev/null 2>&1; then
  until kubectl exec openbao-0 -- bao operator init -key-shares=1 -key-threshold=1 -format=json > init.json 2>/dev/null; do
    sleep 0.5
  done
fi
UNSEAL_KEY=$(jq -r '.unseal_keys_b64[0]' init.json)
BAO_TOKEN=$(jq -r '.root_token' init.json)

# Unseal each pod. Wait for it to join the cluster first, skip if already unsealed.
# After openbao-0, wait for leader election so followers can join Raft.
echo ">>> Unsealing OpenBao..."
for pod in openbao-0 openbao-1 openbao-2; do
  wait_for_pod "$pod"
  until kubectl exec "$pod" -- bao status -format=json 2>/dev/null | jq -e '.initialized' > /dev/null 2>&1; do sleep 0.5; done
  if kubectl exec "$pod" -- bao status -format=json 2>/dev/null | jq -e '.sealed' > /dev/null 2>&1; then
    kubectl exec "$pod" -- sh -c "bao operator unseal $UNSEAL_KEY < /dev/null" > /dev/null
  fi
  if [ "$pod" = "openbao-0" ]; then
    until kubectl exec openbao-0 -- bao status -format=json 2>/dev/null | jq -e '.leader_address != ""' > /dev/null 2>&1; do sleep 0.5; done
  fi
done

# Enable engines and auth. "|| true" makes enable commands idempotent (they
# fail if already enabled). The kv2/config write retries because KV v2 needs
# a moment to finish its internal upgrade after being enabled.
echo ">>> Configuring secrets engines and auth methods..."
bao_exec bao secrets enable -version=1 -path=kv1 kv 2>/dev/null || true
bao_exec bao secrets enable -version=2 -path=kv2 kv 2>/dev/null || true
until bao_exec bao write kv2/config max_versions=3 2>/dev/null; do sleep 0.5; done
bao_exec bao secrets enable transit 2>/dev/null || true
bao_exec bao write -f transit/keys/default 2>/dev/null || true
bao_exec bao auth enable cert 2>/dev/null || true
bao_exec bao write auth/cert/certs/batch certificate=@/host/certs/ca.pem policies=default token_type=batch
bao_exec bao write auth/cert/certs/service certificate=@/host/certs/ca.pem policies=default token_type=service token_ttl=5m

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
  export BAO_CACERT=$PWD/certs/ca.pem
  export UNSEAL_KEY=$(jq -r '.unseal_keys_b64[0]' init.json)
  export BAO_TOKEN=$(jq -r '.root_token' init.json)

To use toolbox, run:

  go run -C tools . --help
EOF
