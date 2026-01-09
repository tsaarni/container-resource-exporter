package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Collector struct {
	kubeClient *KubernetesClient
	config     *Config
}

func NewCollector(config *Config, kubeClient *KubernetesClient) *Collector {
	return &Collector{
		kubeClient: kubeClient,
		config:     config,
	}
}

func (c *Collector) Start(ctx context.Context) {
	ticker := time.NewTicker(c.config.GetScrapeInterval())
	defer ticker.Stop()

	slog.Info("Starting metric collection", "interval", c.config.ScrapeInterval)

	// Collect immediately on start
	c.collect(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping metric collection")
			return
		case <-ticker.C:
			c.collect(ctx)
		}
	}
}

func (c *Collector) collect(ctx context.Context) {
	containers, err := c.kubeClient.DiscoverContainers(ctx)
	if err != nil {
		slog.Error("Failed to discover containers", "error", err)
		return
	}

	if len(containers) == 0 {
		slog.Warn("No containers found matching filters")
		return
	}

	for _, container := range containers {
		// Collect cgroup metrics
		c.collectCgroupMetrics(container)

		// Collect smaps metrics
		c.collectSmapsMetrics(container)
	}

	slog.Debug("Metric collection cycle complete", "containers", len(containers))
}

func (c *Collector) collectCgroupMetrics(container Container) {
	cgroup, err := FindCgroup(c.config.Paths.Cgroup, container.ID)
	if err != nil {
		slog.Warn("Failed to find cgroup", "container", container.Container, "error", err)
		return
	}

	for _, metric := range cgroupMetrics {
		value, err := c.readCgroupMetric(cgroup, metric)
		if err != nil {
			slog.Debug("Failed to read cgroup metric", "file", metric.cgroupFile, "field", metric.cgroupFileField, "error", err)
			continue
		}

		if metric.gauge != nil {
			metric.gauge.WithLabelValues(container.Namespace, container.Pod, container.Container).Set(float64(value))
		} else if metric.counter != nil {
			metric.counter.WithLabelValues(container.Namespace, container.Pod, container.Container).Add(float64(value))
		}
	}

	slog.Debug("Collected cgroup metrics", "namespace", container.Namespace, "pod", container.Pod, "container", container.Container)
}

func (c *Collector) readCgroupMetric(cgroup *CGroup, metric Metric) (int, error) {
	if metric.cgroupFileField == "" {
		return cgroup.ReadInteger(metric.cgroupFile)
	}
	return cgroup.ReadIntegerField(metric.cgroupFile, metric.cgroupFileField)
}

func (c *Collector) collectSmapsMetrics(container Container) {
	if len(container.PIDs) == 0 {
		slog.Debug("No PIDs to collect smaps for", "container", container.Container)
		return
	}

	for _, proc := range container.PIDs {
		smapsPath := filepath.Join(c.config.Paths.Proc, strconv.Itoa(proc.PID), "smaps")
		f, err := os.Open(smapsPath)
		if err != nil {
			slog.Debug("Failed to open smaps", "pid", proc.PID, "error", err)
			continue
		}

		mappings, err := ParseSmaps(f)
		f.Close()
		if err != nil {
			slog.Warn("Failed to parse smaps", "pid", proc.PID, "error", err)
			continue
		}

		for _, m := range mappings {
			c.setSmapsMetrics(container, proc.Comm, m)
		}

		slog.Debug("Collected smaps metrics", "namespace", container.Namespace, "pod", container.Pod, "container", container.Container, "pid", proc.PID, "comm", proc.Comm, "mappings", len(mappings))
	}
}

func (c *Collector) setSmapsMetrics(container Container, comm string, m *SmapsMapping) {
	labels := []string{container.Namespace, container.Pod, container.Container, comm, m.Path}

	ProcessSmapsSize.WithLabelValues(labels...).Set(float64(m.SizeBytes))
	ProcessSmapsRss.WithLabelValues(labels...).Set(float64(m.RssBytes))
	ProcessSmapsPss.WithLabelValues(labels...).Set(float64(m.PssBytes))
	ProcessSmapsPssDirty.WithLabelValues(labels...).Set(float64(m.PssDirtyBytes))
	ProcessSmapsSharedClean.WithLabelValues(labels...).Set(float64(m.SharedCleanBytes))
	ProcessSmapsSharedDirty.WithLabelValues(labels...).Set(float64(m.SharedDirtyBytes))
	ProcessSmapsPrivateClean.WithLabelValues(labels...).Set(float64(m.PrivateCleanBytes))
	ProcessSmapsPrivateDirty.WithLabelValues(labels...).Set(float64(m.PrivateDirtyBytes))
	ProcessSmapsReferenced.WithLabelValues(labels...).Set(float64(m.ReferencedBytes))
	ProcessSmapsAnonymous.WithLabelValues(labels...).Set(float64(m.AnonymousBytes))
	ProcessSmapsLazyFree.WithLabelValues(labels...).Set(float64(m.LazyFreeBytes))
	ProcessSmapsAnonHugePages.WithLabelValues(labels...).Set(float64(m.AnonHugePagesBytes))
	ProcessSmapsShmemPmdMapped.WithLabelValues(labels...).Set(float64(m.ShmemPmdMappedBytes))
	ProcessSmapsSharedHugetlb.WithLabelValues(labels...).Set(float64(m.SharedHugetlbBytes))
	ProcessSmapsPrivateHugetlb.WithLabelValues(labels...).Set(float64(m.PrivateHugetlbBytes))
	ProcessSmapsSwap.WithLabelValues(labels...).Set(float64(m.SwapBytes))
	ProcessSmapsSwapPss.WithLabelValues(labels...).Set(float64(m.SwapPssBytes))
	ProcessSmapsKernelPageSize.WithLabelValues(labels...).Set(float64(m.KernelPageSizeBytes))
	ProcessSmapsMMUPageSize.WithLabelValues(labels...).Set(float64(m.MMUPageSizeBytes))
	ProcessSmapsLocked.WithLabelValues(labels...).Set(float64(m.LockedBytes))
}
