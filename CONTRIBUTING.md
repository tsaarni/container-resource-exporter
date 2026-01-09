# Contributing

This guide is for those who wish to contribute to the project.

## Development

To build the project, ensure you have Go installed and run:

```bash
make
```

Set up Kubernetes cluster for testing:

```bash
make kind-create
```

Create `ConfigMaps` for configuration files:

```bash
kubectl create configmap container-resource-exporter-config --from-file=config.yaml=examples/contour/contour-and-envoy-metrics.yaml --dry-run=client -o yaml | kubectl apply -f -
kubectl create configmap grafana-dashboards --from-file=examples/contour/grafana-dashboard-envoy.json --dry-run=client -o yaml | kubectl apply -f -
```

Deploy the manifests to the cluster

```bash
kubectl apply -f manifests/
```

This will deploy `container-resource-exporter`, Prometheus, and Grafana:


Re-build, load the container image into the kind cluster and restart the daemonset:

```bash
make container
make kind-load
```

### Example Workload

Deploy Contour and Envoy to generate some resource usage data:

```bash
kubectl apply -f https://projectcontour.io/quickstart/contour.yaml

# Scale Contour to one replica for simplicity.
kubectl scale deployment/contour -n projectcontour --replicas=1

kubectl apply -f https://raw.githubusercontent.com/tsaarni/echoserver/refs/heads/main/manifests/echoserver.yaml

# Create HTTPProxy to route traffic to echoserver
cat <<EOF | kubectl apply -f -
apiVersion: projectcontour.io/v1
kind: HTTPProxy
metadata:
  name: echoserver
spec:
  virtualhost:
    fqdn: echoserver.127.0.0.136.nip.io
  routes:
    - services:
        - name: echoserver
          port: 80
EOF
```



Access Grafana at `http://grafana.127.0.0.136.nip.io:3000/`




```bash
# Restart `container-resource-exporter` deployment to pick up new configuration:
kubectl delete pod -l app=container-resource-exporter
# Restart prometheus deployment to reset its data:
kubectl delete pod -l app=prometheus
# Restart Envoy pods
kubectl delete pod -l app=envoy -n projectcontour
```

```bash
http http://exporter.127.0.0.136.nip.io:8080/metrics
```


```bash
go run github.com/tsaarni/echoclient/cmd/echoclient@latest get -url http://127.0.0.136.nip.io/ -concurrency 100 -duration 30s
```
