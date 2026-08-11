package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/bmatcuk/doublestar/v4"
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

	// Reset all metrics to remove stale series from disappeared containers/processes.
	resetAllMetrics()

	for _, container := range containers {
		// Collect cgroup metrics (including io.stat)
		c.collectCgroupMetrics(container)

		// Collect smaps metrics
		c.collectSmapsMetrics(container)

		// Collect per-process I/O metrics
		c.collectProcIoMetrics(container)

		// Collect disk metrics
		c.collectDiskMetrics(container)

		// Collect file metrics
		c.collectFileMetrics(container)
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

		finalValue := float64(value)
		if metric.conversionFactor != 0 {
			finalValue *= metric.conversionFactor
		}

		metric.gauge.WithLabelValues(container.Namespace, container.Pod, container.Container).Set(finalValue)
	}

	for _, metric := range cgroupMultiValueMetrics {
		values, err := cgroup.ReadSpaceSeparatedIntegers(metric.cgroupFile)
		if err != nil {
			slog.Debug("Failed to read cgroup multi-value metric", "file", metric.cgroupFile, "error", err)
			continue
		}
		if len(values) != len(metric.gauges) {
			slog.Debug("Unexpected number of values in cgroup file", "file", metric.cgroupFile, "expected", len(metric.gauges), "got", len(values))
		}
		for i, gauge := range metric.gauges {
			if i >= len(values) {
				break
			}
			finalValue := float64(values[i])
			if metric.conversionFactor != 0 {
				finalValue *= metric.conversionFactor
			}
			gauge.WithLabelValues(container.Namespace, container.Pod, container.Container).Set(finalValue)
		}
	}

	slog.Debug("Collected cgroup metrics", "namespace", container.Namespace, "pod", container.Pod, "container", container.Container)

	// Collect io.stat metrics using the same cgroup
	stats, err := cgroup.ReadIoStat()
	if err != nil {
		slog.Debug("Failed to read io.stat", "container", container.Container, "error", err)
		return
	}

	for _, stat := range stats {
		major := strconv.Itoa(stat.Major)
		minor := strconv.Itoa(stat.Minor)
		labels := []string{container.Namespace, container.Pod, container.Container, major, minor}

		CgroupIoReadBytes.WithLabelValues(labels...).Set(float64(stat.Rbytes))
		CgroupIoWriteBytes.WithLabelValues(labels...).Set(float64(stat.Wbytes))
		CgroupIoReadOps.WithLabelValues(labels...).Set(float64(stat.Rios))
		CgroupIoWriteOps.WithLabelValues(labels...).Set(float64(stat.Wios))
		CgroupIoDiscardBytes.WithLabelValues(labels...).Set(float64(stat.Dbytes))
		CgroupIoDiscardOps.WithLabelValues(labels...).Set(float64(stat.Dios))
	}

	slog.Debug("Collected io.stat metrics", "namespace", container.Namespace, "pod", container.Pod, "container", container.Container, "devices", len(stats))
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
		if closeErr := f.Close(); closeErr != nil {
			slog.Warn("Failed to close smaps", "pid", proc.PID, "error", closeErr)
		}
		if err != nil {
			slog.Warn("Failed to parse smaps", "pid", proc.PID, "error", err)
			continue
		}

		for _, m := range mappings {
			c.setSmapsMetrics(container, proc, m)
		}

		slog.Debug("Collected smaps metrics", "namespace", container.Namespace, "pod", container.Pod, "container", container.Container, "pid", proc.PID, "ns_pid", proc.NSPID, "comm", proc.Comm, "mappings", len(mappings))
	}
}

func (c *Collector) setSmapsMetrics(container Container, proc ProcessInfo, m *SmapsMapping) {
	labels := []string{container.Namespace, container.Pod, container.Container, strconv.Itoa(proc.PID), strconv.Itoa(proc.NSPID), proc.Comm, m.Path}

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

func (c *Collector) collectProcIoMetrics(container Container) {
	if len(container.PIDs) == 0 {
		slog.Debug("No PIDs to collect proc io for", "container", container.Container)
		return
	}

	for _, proc := range container.PIDs {
		ioPath := filepath.Join(c.config.Paths.Proc, strconv.Itoa(proc.PID), "io")
		f, err := os.Open(ioPath)
		if err != nil {
			slog.Debug("Failed to open proc io", "pid", proc.PID, "error", err)
			continue
		}

		procIO, err := ParseProcIO(f)
		if closeErr := f.Close(); closeErr != nil {
			slog.Warn("Failed to close proc io", "pid", proc.PID, "error", closeErr)
		}
		if err != nil {
			slog.Warn("Failed to parse proc io", "pid", proc.PID, "error", err)
			continue
		}

		labels := []string{container.Namespace, container.Pod, container.Container, strconv.Itoa(proc.PID), strconv.Itoa(proc.NSPID), proc.Comm}

		ProcessIoRchar.WithLabelValues(labels...).Set(float64(procIO.Rchar))
		ProcessIoWchar.WithLabelValues(labels...).Set(float64(procIO.Wchar))
		ProcessIoSyscr.WithLabelValues(labels...).Set(float64(procIO.Syscr))
		ProcessIoSyscw.WithLabelValues(labels...).Set(float64(procIO.Syscw))
		ProcessIoReadBytes.WithLabelValues(labels...).Set(float64(procIO.ReadBytes))
		ProcessIoWriteBytes.WithLabelValues(labels...).Set(float64(procIO.WriteBytes))

		slog.Debug("Collected proc io metrics", "namespace", container.Namespace, "pod", container.Pod, "container", container.Container, "pid", proc.PID)
	}
}

func (c *Collector) collectDiskMetrics(container Container) {
	if len(container.MountpointPaths) == 0 || len(container.PIDs) == 0 {
		return
	}

	// Use first PID — all processes in a container share the same mount namespace.
	pid := container.PIDs[0].PID
	mountinfoPath := filepath.Join(c.config.Paths.Proc, strconv.Itoa(pid), "mountinfo")
	f, err := os.Open(mountinfoPath)
	if err != nil {
		slog.Debug("Failed to open mountinfo", "pid", pid, "error", err)
		return
	}
	defer func() { _ = f.Close() }()

	mounts, err := ParseMountInfo(f)
	if err != nil {
		slog.Warn("Failed to parse mountinfo", "pid", pid, "error", err)
		return
	}

	// Index mounts by mountpoint for quick lookup.
	mountMap := make(map[string]MountInfo, len(mounts))
	for _, mount := range mounts {
		mountMap[mount.MountPoint] = mount
	}

	for _, mountpoint := range container.MountpointPaths {
		mount, ok := mountMap[mountpoint]
		if !ok {
			slog.Debug("Mountpoint not found in mountinfo", "mountpoint", mountpoint)
			continue
		}

		// statfs via /proc/<pid>/root/<mountpoint> to access the container's mount namespace.
		statPath := filepath.Join(c.config.Paths.Proc, strconv.Itoa(pid), "root", mount.MountPoint)
		var stat syscall.Statfs_t
		if err := syscall.Statfs(statPath, &stat); err != nil {
			slog.Debug("Failed to statfs", "path", statPath, "error", err)
			continue
		}

		bsize := uint64(stat.Bsize)
		capacity := float64(stat.Blocks * bsize)
		available := float64(stat.Bavail * bsize)
		used := capacity - available

		labels := []string{container.Namespace, container.Pod, container.Container, mount.MountPoint, mount.FsType, mount.Source}
		MountpointCapacityBytes.WithLabelValues(labels...).Set(capacity)
		MountpointAvailableBytes.WithLabelValues(labels...).Set(available)
		MountpointUsedBytes.WithLabelValues(labels...).Set(used)
	}

	slog.Debug("Collected disk metrics", "namespace", container.Namespace, "pod", container.Pod, "container", container.Container)
}

func (c *Collector) collectFileMetrics(container Container) {
	if len(container.FileMetricPaths) == 0 || len(container.PIDs) == 0 {
		return
	}

	pid := container.PIDs[0].PID
	rootPath := filepath.Join(c.config.Paths.Proc, strconv.Itoa(pid), "root")

	for _, pattern := range container.FileMetricPaths {
		matches, err := doublestar.Glob(os.DirFS(rootPath), pattern[1:]) // Strip leading "/" for fs.FS
		if err != nil {
			slog.Debug("Invalid file glob pattern", "pattern", pattern, "error", err)
			continue
		}

		var totalSize, totalDiskUsage int64
		for _, match := range matches {
			fullPath := filepath.Join(rootPath, match)
			info, err := os.Lstat(fullPath)
			if err != nil {
				slog.Debug("Failed to stat file", "path", fullPath, "error", err)
				continue
			}
			if info.IsDir() {
				continue
			}

			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				continue
			}

			totalSize += info.Size()
			totalDiskUsage += stat.Blocks * 512
		}

		labels := []string{container.Namespace, container.Pod, container.Container, pattern}
		FileSizeBytes.WithLabelValues(labels...).Set(float64(totalSize))
		FileDiskUsageBytes.WithLabelValues(labels...).Set(float64(totalDiskUsage))
	}

	slog.Debug("Collected file metrics", "namespace", container.Namespace, "pod", container.Pod, "container", container.Container)
}
