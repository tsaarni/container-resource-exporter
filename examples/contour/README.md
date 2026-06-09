# Contour and Envoy Monitoring Example

This example demonstrates how to monitor Contour and Envoy using `container-resource-exporter`.

![Grafana Screenshot](grafana-screenshot.png)

## Prerequisites

- [Kind](https://kind.sigs.k8s.io/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Go](https://go.dev/)

The environment will use `go run` to execute [`certyaml`](https://github.com/tsaarni/certyaml), and the tools in the [`tools`](tools) directory.

## Deployment

Run the [setup.sh](setup.sh) script to create Kind cluster, deploy Contour, Envoy, and the observability stack (Prometheus, Grafana, and the exporter).

```bash
./setup.sh
```

## Host Port Mappings

The following services are exposed on the host after setup:

- **Grafana**: [http://localhost:3000/](http://localhost:3000/)
  - Click "Dashboards" > "Contour & Envoy Observability" to view the dashboard.
  - Pick Contour or Envoy pod from the Pod dropdown to see metrics for that specific pod.
- **Prometheus**: [http://localhost:9090/](http://localhost:9090/)
- **container-resource-exporter**: [http://localhost:8080/metrics](http://localhost:8080/metrics)
- **Envoy Admin**: [http://localhost:9001/](http://localhost:9001/)
- **Contour Metrics**: [http://localhost:9002/metrics](http://localhost:9002/metrics)
- **Contour Debug**: [http://localhost:6060/debug/pprof/](http://localhost:6060/debug/pprof/)

The [`echoserver`](https://github.com/tsaarni/echoserver) workload is exposed via four Envoy virtual hosts with different authentication modes.

## Interacting with Contour and Envoy

Send requests to the echoserver via Envoy

```
curl http://echoserver.127.0.0.1.nip.io/
curl --cacert certs/external-ca.pem https://echoserver-tls.127.0.0.1.nip.io/
curl --cacert certs/external-ca.pem --cert certs/external-client.pem --key certs/external-client-key.pem https://echoserver-cert-auth.127.0.0.1.nip.io/
curl --cacert certs/external-ca.pem -H "Authorization: Bearer $(go run -C tools . jwt sign)" https://echoserver-jwt.127.0.0.1.nip.io/
```

The [`tools`](tools) directory contains additional tools to interact with Contour and Envoy, such as a traffic generator that simulates load.
For example, to generate 100 requests for 10 seconds, run:

```bash
go run -C tools . http --rps 100 --duration 10s
```


To see all the sub-commands available in the tools, run:

```bash
go run -C tools . --help
```

If you are using an LLM agent, point it to [`AGENTS.md`](AGENTS.md) to use the environment as an interactive playground to test and learn about Contour and Envoy.

## Accessing Metrics in Testing

You can use `container-resource-exporter` metrics in CI/CD pipelines to detect resource usage regressions when deploying new software versions.

1. Reset the peak memory metric by restarting the pod
2. Run your test that exercises the system under test
3. Capture peak memory, total CPU usage and throttling metrics
4. Compare against baseline values from previous versions to detect major changes

In the demo deployment, the following endpoint exposes the metrics:

- **container-resource-exporter**: [http://localhost:8080/metrics](http://localhost:8080/metrics) (see [METRICS.md](/METRICS.md))

Example Python script:

```python
from urllib.request import urlopen
import re

# Get metrics from exporter
metrics = urlopen('http://localhost:8080/metrics').read().decode()

# Extract peak memory (since container start)
peak_mem = float(re.search(r'cgroup_memory_peak_bytes{container="envoy"[^}]*} (\S+)', metrics).group(1))

# Extract memory limit
mem_limit = float(re.search(r'cgroup_memory_max_bytes{container="envoy"[^}]*} (\S+)', metrics).group(1))

# Extract CPU and throttling metrics (since container start)
# Note: For test-specific measurements, capture before/after and compute delta.
cpu_total = float(re.search(r'cgroup_cpu_usage_seconds_total{container="envoy"[^}]*} (\S+)', metrics).group(1))
nr_periods = float(re.search(r'cgroup_cpu_nr_periods_total{container="envoy"[^}]*} (\S+)', metrics).group(1))
nr_throttled = float(re.search(r'cgroup_cpu_nr_throttled_total{container="envoy"[^}]*} (\S+)', metrics).group(1))

# Calculate derived values
peak_mem_mb = f"{peak_mem / 1024**2:.1f} MB"
mem_limit_str = f"{mem_limit/1024**2:.1f} MB" if mem_limit > 0 and mem_limit < 2**63 else "unlimited"
mem_usage_str = f"{(peak_mem / mem_limit) * 100:.1f}% of limit" if mem_limit > 0 and mem_limit < 2**63 else "N/A (no limit set)"
cpu_total_str = f"{cpu_total:.2f} seconds"
throttle_str = f"{100 * nr_throttled / nr_periods if nr_periods > 0 else 0:.1f}%"

print(f"""Peak Memory: {peak_mem_mb}
Memory Limit: {mem_limit_str}
Memory Usage: {mem_usage_str}
Total CPU Time: {cpu_total_str}
Throttling %: {throttle_str}""")
```

Output example:

```
Peak Memory: 33.5 MB
Memory Limit: 512.0 MB
Memory Usage: 6.5% of limit
Total CPU Time: 137.29 seconds
Throttling %: 57.1%
```

## Cleanup

```bash
kind delete cluster --name contour
```
