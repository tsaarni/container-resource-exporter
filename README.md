# Contariner Resource Exporter

Exports Linux cgroup and `/proc/[pid]/smaps` memory metrics for containers in a format compatible with Prometheus scraping.

This exporter is designed for Kubernetes environments and collects detailed memory mapping statistics for processes running inside containers.
It uses the containerd runtime to discover container processes.


TODO: documentation.


## Contributing

Please refer to [CONTRIBUTING.md](CONTRIBUTING.md).
