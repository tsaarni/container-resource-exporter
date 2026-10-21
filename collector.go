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
	"github.com/prometheus/client_golang/prometheus"
)

// discoveryTimeout bounds container discovery so that a hung CRI call does not block scrapes indefinitely.
const discoveryTimeout = 10 * time.Second

// Collector implements prometheus.Collector. Metrics are read on each scrape and
// sent as const metrics, so a scrape always sees one complete and current collection.
type Collector struct {
	config   *Config
	discover func(context.Context) ([]Container, error)
}

func NewCollector(config *Config, discover func(context.Context) ([]Container, error)) *Collector {
	return &Collector{
		config:   config,
		discover: discover,
	}
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	describeAll(ch)
}

// Collect implements prometheus.Collector.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), discoveryTimeout)
	defer cancel()

	containers, err := c.discover(ctx)
	if err != nil {
		// Fail the scrape rather than expose an incomplete result.
		slog.Error("Failed to discover containers", "error", err)
		ch <- prometheus.NewInvalidMetric(prometheus.NewInvalidDesc(err), err)
		return
	}

	if len(containers) == 0 {
		slog.Warn("No containers found matching filters")
		return
	}

	for _, container := range containers {
		// Collect cgroup metrics (including io.stat)
		c.collectCgroupMetrics(ch, container)

		// Collect smaps metrics
		c.collectSmapsMetrics(ch, container)

		// Collect per-process I/O metrics
		c.collectProcIoMetrics(ch, container)

		// Collect disk metrics
		c.collectDiskMetrics(ch, container)

		// Collect file metrics
		c.collectFileMetrics(ch, container)
	}

	slog.Debug("Metric collection complete", "containers", len(containers))
}

// gauge sends a gauge const metric.
func gauge(ch chan<- prometheus.Metric, desc *prometheus.Desc, value float64, labels ...string) {
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, labels...)
}

func (c *Collector) collectCgroupMetrics(ch chan<- prometheus.Metric, container Container) {
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

		ch <- prometheus.MustNewConstMetric(metric.desc, metric.valueType, finalValue, container.Namespace, container.Pod, container.Container)
	}

	for _, metric := range cgroupMultiValueMetrics {
		values, err := cgroup.ReadSpaceSeparatedIntegers(metric.cgroupFile)
		if err != nil {
			slog.Debug("Failed to read cgroup multi-value metric", "file", metric.cgroupFile, "error", err)
			continue
		}
		if len(values) != len(metric.descs) {
			slog.Debug("Unexpected number of values in cgroup file", "file", metric.cgroupFile, "expected", len(metric.descs), "got", len(values))
		}
		for i, desc := range metric.descs {
			if i >= len(values) {
				break
			}
			finalValue := float64(values[i])
			if metric.conversionFactor != 0 {
				finalValue *= metric.conversionFactor
			}
			gauge(ch, desc, finalValue, container.Namespace, container.Pod, container.Container)
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

		gauge(ch, CgroupIoReadBytes, float64(stat.Rbytes), labels...)
		gauge(ch, CgroupIoWriteBytes, float64(stat.Wbytes), labels...)
		gauge(ch, CgroupIoReadOps, float64(stat.Rios), labels...)
		gauge(ch, CgroupIoWriteOps, float64(stat.Wios), labels...)
		gauge(ch, CgroupIoDiscardBytes, float64(stat.Dbytes), labels...)
		gauge(ch, CgroupIoDiscardOps, float64(stat.Dios), labels...)
	}

	slog.Debug("Collected io.stat metrics", "namespace", container.Namespace, "pod", container.Pod, "container", container.Container, "devices", len(stats))
}

func (c *Collector) readCgroupMetric(cgroup *CGroup, metric Metric) (int, error) {
	if metric.cgroupFileField == "" {
		return cgroup.ReadInteger(metric.cgroupFile)
	}
	return cgroup.ReadIntegerField(metric.cgroupFile, metric.cgroupFileField)
}

func (c *Collector) collectSmapsMetrics(ch chan<- prometheus.Metric, container Container) {
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
			c.setSmapsMetrics(ch, container, proc, m)
		}

		slog.Debug("Collected smaps metrics", "namespace", container.Namespace, "pod", container.Pod, "container", container.Container, "pid", proc.PID, "ns_pid", proc.NSPID, "comm", proc.Comm, "mappings", len(mappings))
	}
}

func (c *Collector) setSmapsMetrics(ch chan<- prometheus.Metric, container Container, proc ProcessInfo, m *SmapsMapping) {
	labels := []string{container.Namespace, container.Pod, container.Container, strconv.Itoa(proc.PID), strconv.Itoa(proc.NSPID), proc.Comm, m.Path}

	gauge(ch, ProcessSmapsSize, float64(m.SizeBytes), labels...)
	gauge(ch, ProcessSmapsRss, float64(m.RssBytes), labels...)
	gauge(ch, ProcessSmapsPss, float64(m.PssBytes), labels...)
	gauge(ch, ProcessSmapsPssDirty, float64(m.PssDirtyBytes), labels...)
	gauge(ch, ProcessSmapsSharedClean, float64(m.SharedCleanBytes), labels...)
	gauge(ch, ProcessSmapsSharedDirty, float64(m.SharedDirtyBytes), labels...)
	gauge(ch, ProcessSmapsPrivateClean, float64(m.PrivateCleanBytes), labels...)
	gauge(ch, ProcessSmapsPrivateDirty, float64(m.PrivateDirtyBytes), labels...)
	gauge(ch, ProcessSmapsReferenced, float64(m.ReferencedBytes), labels...)
	gauge(ch, ProcessSmapsAnonymous, float64(m.AnonymousBytes), labels...)
	gauge(ch, ProcessSmapsLazyFree, float64(m.LazyFreeBytes), labels...)
	gauge(ch, ProcessSmapsAnonHugePages, float64(m.AnonHugePagesBytes), labels...)
	gauge(ch, ProcessSmapsShmemPmdMapped, float64(m.ShmemPmdMappedBytes), labels...)
	gauge(ch, ProcessSmapsSharedHugetlb, float64(m.SharedHugetlbBytes), labels...)
	gauge(ch, ProcessSmapsPrivateHugetlb, float64(m.PrivateHugetlbBytes), labels...)
	gauge(ch, ProcessSmapsSwap, float64(m.SwapBytes), labels...)
	gauge(ch, ProcessSmapsSwapPss, float64(m.SwapPssBytes), labels...)
	gauge(ch, ProcessSmapsKernelPageSize, float64(m.KernelPageSizeBytes), labels...)
	gauge(ch, ProcessSmapsMMUPageSize, float64(m.MMUPageSizeBytes), labels...)
	gauge(ch, ProcessSmapsLocked, float64(m.LockedBytes), labels...)
}

func (c *Collector) collectProcIoMetrics(ch chan<- prometheus.Metric, container Container) {
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

		gauge(ch, ProcessIoRchar, float64(procIO.Rchar), labels...)
		gauge(ch, ProcessIoWchar, float64(procIO.Wchar), labels...)
		gauge(ch, ProcessIoSyscr, float64(procIO.Syscr), labels...)
		gauge(ch, ProcessIoSyscw, float64(procIO.Syscw), labels...)
		gauge(ch, ProcessIoReadBytes, float64(procIO.ReadBytes), labels...)
		gauge(ch, ProcessIoWriteBytes, float64(procIO.WriteBytes), labels...)

		slog.Debug("Collected proc io metrics", "namespace", container.Namespace, "pod", container.Pod, "container", container.Container, "pid", proc.PID)
	}
}

func (c *Collector) collectDiskMetrics(ch chan<- prometheus.Metric, container Container) {
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
		gauge(ch, MountpointCapacityBytes, capacity, labels...)
		gauge(ch, MountpointAvailableBytes, available, labels...)
		gauge(ch, MountpointUsedBytes, used, labels...)
	}

	slog.Debug("Collected disk metrics", "namespace", container.Namespace, "pod", container.Pod, "container", container.Container)
}

func (c *Collector) collectFileMetrics(ch chan<- prometheus.Metric, container Container) {
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
		gauge(ch, FileSizeBytes, float64(totalSize), labels...)
		gauge(ch, FileDiskUsageBytes, float64(totalDiskUsage), labels...)
	}

	slog.Debug("Collected file metrics", "namespace", container.Namespace, "pod", container.Pod, "container", container.Container)
}
