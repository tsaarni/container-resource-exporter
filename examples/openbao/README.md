# OpenBao Monitoring Example

This example demonstrates how to monitor [OpenBao](https://openbao.org/) using `container-resource-exporter`.

![Grafana Screenshot](grafana-screenshot.png)

## Prerequisites

- [Kind](https://kind.sigs.k8s.io/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Go](https://go.dev/)

The environment will use `go run` to execute [`certyaml`](https://github.com/tsaarni/certyaml), [`raft-inspector`](https://github.com/tsaarni/raft-inspector/) and the traffic generator in the [`traffic`](traffic) directory.

## Deployment

Run the [setup.sh](setup.sh) script to create the Kind cluster and deploy OpenBao with the full observability stack (Prometheus, Grafana, and the exporter). The script also initializes and unseals the cluster automatically.

```bash
./setup.sh
```

## Host Port Mappings

The following services are exposed on the host after setup:

- **Grafana**: [http://localhost:3000/](http://localhost:3000/)
- **Prometheus**: [http://localhost:9090/](http://localhost:9090/)
- **container-resource-exporter**: [http://localhost:8080/metrics](http://localhost:8080/metrics)
- **OpenBao (openbao-0)**: `https://localhost:8200`
- **OpenBao (openbao-1)**: `https://localhost:8201`
- **OpenBao (openbao-2)**: `https://localhost:8202`

## Generating Load

To generate load on OpenBao, run the traffic generator, for example:

```bash
BAO_TOKEN=$(jq -r '.root_token' init.json)
go run -C traffic . kv-write
```

Then open Grafana at http://localhost:3000/ -> Dashboards -> **OpenBao Resource Monitoring** and pick a pod from the dropdown.

Use [`RUNBOOK.md`](RUNBOOK.md) for inspiration, or as instructions for LLM agents, on how to explore the collected metrics and data.

## Cleanup

```bash
kind delete cluster --name openbao
```
