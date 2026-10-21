package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Variable label sets shared by the metric descriptors.
var (
	containerLabels  = []string{"namespace", "pod", "container"}
	processLabels    = []string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm"}
	smapsLabels      = []string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"}
	ioStatLabels     = []string{"namespace", "pod", "container", "major", "minor"}
	mountpointLabels = []string{"namespace", "pod", "container", "mountpoint", "fstype", "device"}
	fileLabels       = []string{"namespace", "pod", "container", "path"}
)

type Metric struct {
	desc             *prometheus.Desc
	valueType        prometheus.ValueType
	cgroupFile       string
	cgroupFileField  string
	conversionFactor float64
}

// Cgroup v2 metrics
// https://docs.kernel.org/admin-guide/cgroup-v2.html

var cgroupMetrics = []Metric{
	// Memory
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_current_bytes",
			"Total memory currently used by the cgroup and its descendants, in bytes (from memory.current).",
			containerLabels, nil,
		),
		valueType:  prometheus.GaugeValue,
		cgroupFile: "memory.current",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_peak_bytes",
			"Maximum memory usage recorded for the cgroup and its descendants since creation or last reset (from memory.peak).",
			containerLabels, nil,
		),
		valueType:  prometheus.GaugeValue,
		cgroupFile: "memory.peak",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_low_bytes",
			"Best-effort memory protection threshold below which memory is not reclaimed (from memory.low).",
			containerLabels, nil,
		),
		valueType:  prometheus.GaugeValue,
		cgroupFile: "memory.low",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_high_bytes",
			"Memory usage throttle limit above which processes are throttled and put under reclaim pressure (from memory.high).",
			containerLabels, nil,
		),
		valueType:  prometheus.GaugeValue,
		cgroupFile: "memory.high",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_max_bytes",
			"Hard memory usage limit for the cgroup; exceeding this may trigger OOM killer (from memory.max).",
			containerLabels, nil,
		),
		valueType:  prometheus.GaugeValue,
		cgroupFile: "memory.max",
	},
	// memory.stat fields
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_stat_anon_bytes",
			"Amount of memory used in anonymous mappings such as brk(), sbrk(), and mmap(MAP_ANONYMOUS) (from memory.stat:anon).",
			containerLabels, nil,
		),
		valueType:       prometheus.GaugeValue,
		cgroupFile:      "memory.stat",
		cgroupFileField: "anon",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_stat_file_bytes",
			"Amount of memory used to cache filesystem data, including tmpfs and shared memory (from memory.stat:file).",
			containerLabels, nil,
		),
		valueType:       prometheus.GaugeValue,
		cgroupFile:      "memory.stat",
		cgroupFileField: "file",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_stat_shmem_bytes",
			"Amount of cached filesystem data that is swap-backed, such as tmpfs, shm segments, and shared anonymous mmap()s (from memory.stat:shmem).",
			containerLabels, nil,
		),
		valueType:       prometheus.GaugeValue,
		cgroupFile:      "memory.stat",
		cgroupFileField: "shmem",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_stat_kernel_bytes",
			"Total kernel memory usage, including kernel_stack, pagetables, percpu, vmalloc, and slab (from memory.stat:kernel).",
			containerLabels, nil,
		),
		valueType:       prometheus.GaugeValue,
		cgroupFile:      "memory.stat",
		cgroupFileField: "kernel",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_stat_slab_bytes",
			"Amount of memory used for storing in-kernel data structures (from memory.stat:slab).",
			containerLabels, nil,
		),
		valueType:       prometheus.GaugeValue,
		cgroupFile:      "memory.stat",
		cgroupFileField: "slab",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_stat_slab_reclaimable_bytes",
			"Part of slab memory that might be reclaimed, such as dentries and inodes (from memory.stat:slab_reclaimable).",
			containerLabels, nil,
		),
		valueType:       prometheus.GaugeValue,
		cgroupFile:      "memory.stat",
		cgroupFileField: "slab_reclaimable",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_stat_slab_unreclaimable_bytes",
			"Part of slab memory that cannot be reclaimed on memory pressure (from memory.stat:slab_unreclaimable).",
			containerLabels, nil,
		),
		valueType:       prometheus.GaugeValue,
		cgroupFile:      "memory.stat",
		cgroupFileField: "slab_unreclaimable",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_stat_pagetables_bytes",
			"Amount of memory allocated for page tables (from memory.stat:pagetables).",
			containerLabels, nil,
		),
		valueType:       prometheus.GaugeValue,
		cgroupFile:      "memory.stat",
		cgroupFileField: "pagetables",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_stat_kernel_stack_bytes",
			"Amount of memory allocated to kernel stacks (from memory.stat:kernel_stack).",
			containerLabels, nil,
		),
		valueType:       prometheus.GaugeValue,
		cgroupFile:      "memory.stat",
		cgroupFileField: "kernel_stack",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_stat_active_anon_bytes",
			"Amount of active anonymous memory on the internal memory management lists (from memory.stat:active_anon).",
			containerLabels, nil,
		),
		valueType:       prometheus.GaugeValue,
		cgroupFile:      "memory.stat",
		cgroupFileField: "active_anon",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_stat_inactive_anon_bytes",
			"Amount of inactive anonymous memory on the internal memory management lists (from memory.stat:inactive_anon).",
			containerLabels, nil,
		),
		valueType:       prometheus.GaugeValue,
		cgroupFile:      "memory.stat",
		cgroupFileField: "inactive_anon",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_stat_active_file_bytes",
			"Amount of active file-backed memory on the internal memory management lists (from memory.stat:active_file).",
			containerLabels, nil,
		),
		valueType:       prometheus.GaugeValue,
		cgroupFile:      "memory.stat",
		cgroupFileField: "active_file",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_stat_inactive_file_bytes",
			"Amount of inactive file-backed memory on the internal memory management lists (from memory.stat:inactive_file).",
			containerLabels, nil,
		),
		valueType:       prometheus.GaugeValue,
		cgroupFile:      "memory.stat",
		cgroupFileField: "inactive_file",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_stat_unevictable_bytes",
			"Amount of unevictable memory (from memory.stat:unevictable).",
			containerLabels, nil,
		),
		valueType:       prometheus.GaugeValue,
		cgroupFile:      "memory.stat",
		cgroupFileField: "unevictable",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_stat_pgfault_total",
			"Total number of page faults incurred by the cgroup (from memory.stat:pgfault).",
			containerLabels, nil,
		),
		valueType:       prometheus.CounterValue,
		cgroupFile:      "memory.stat",
		cgroupFileField: "pgfault",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_memory_stat_pgmajfault_total",
			"Number of major page faults incurred by the cgroup (from memory.stat:pgmajfault).",
			containerLabels, nil,
		),
		valueType:       prometheus.CounterValue,
		cgroupFile:      "memory.stat",
		cgroupFileField: "pgmajfault",
	},
	// CPU
	{
		desc: prometheus.NewDesc(
			"cgroup_cpu_usage_seconds_total",
			"Total CPU time consumed by all processes in the cgroup, in seconds (from cpu.stat:usage_usec).",
			containerLabels, nil,
		),
		valueType:        prometheus.CounterValue,
		cgroupFile:       "cpu.stat",
		cgroupFileField:  "usage_usec",
		conversionFactor: 0.000001,
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_cpu_user_seconds_total",
			"Total user mode CPU time consumed by the cgroup, in seconds (from cpu.stat:user_usec).",
			containerLabels, nil,
		),
		valueType:        prometheus.CounterValue,
		cgroupFile:       "cpu.stat",
		cgroupFileField:  "user_usec",
		conversionFactor: 0.000001,
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_cpu_system_seconds_total",
			"Total system (kernel) mode CPU time consumed by the cgroup, in seconds (from cpu.stat:system_usec).",
			containerLabels, nil,
		),
		valueType:        prometheus.CounterValue,
		cgroupFile:       "cpu.stat",
		cgroupFileField:  "system_usec",
		conversionFactor: 0.000001,
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_cpu_nr_periods_total",
			"Number of enforcement intervals (periods) for CPU bandwidth (from cpu.stat:nr_periods).",
			containerLabels, nil,
		),
		valueType:       prometheus.CounterValue,
		cgroupFile:      "cpu.stat",
		cgroupFileField: "nr_periods",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_cpu_nr_throttled_total",
			"Number of periods in which the cgroup was throttled due to CPU quota (from cpu.stat:nr_throttled).",
			containerLabels, nil,
		),
		valueType:       prometheus.CounterValue,
		cgroupFile:      "cpu.stat",
		cgroupFileField: "nr_throttled",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_cpu_throttled_seconds_total",
			"Total time duration in seconds that the cgroup was throttled due to CPU quota (from cpu.stat:throttled_usec).",
			containerLabels, nil,
		),
		valueType:        prometheus.CounterValue,
		cgroupFile:       "cpu.stat",
		cgroupFileField:  "throttled_usec",
		conversionFactor: 0.000001,
	},
	// PIDs
	{
		desc: prometheus.NewDesc(
			"cgroup_pids_current",
			"Number of processes currently in the cgroup and its descendants (from pids.current).",
			containerLabels, nil,
		),
		valueType:  prometheus.GaugeValue,
		cgroupFile: "pids.current",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_pids_max",
			"Hard limit on the number of processes allowed in the cgroup (from pids.max).",
			containerLabels, nil,
		),
		valueType:  prometheus.GaugeValue,
		cgroupFile: "pids.max",
	},
	{
		desc: prometheus.NewDesc(
			"cgroup_pids_peak",
			"Maximum number of processes ever present in the cgroup and its descendants (from pids.peak).",
			containerLabels, nil,
		),
		valueType:  prometheus.GaugeValue,
		cgroupFile: "pids.peak",
	},
}

// Multi-value cgroup metrics (files with space-separated values like cpu.max)

type MultiValueMetric struct {
	descs            []*prometheus.Desc
	cgroupFile       string
	conversionFactor float64
}

var cgroupMultiValueMetrics = []MultiValueMetric{
	{
		cgroupFile: "cpu.max",
		descs: []*prometheus.Desc{
			prometheus.NewDesc(
				"cgroup_cpu_max_quota_microseconds",
				"CPU time in microseconds that the cgroup can consume per period; -1 means no limit (from cpu.max).",
				containerLabels, nil,
			),
			prometheus.NewDesc(
				"cgroup_cpu_max_period_microseconds",
				"Length of the CPU bandwidth period in microseconds (from cpu.max).",
				containerLabels, nil,
			),
		},
	},
}

// Smaps metrics - enhanced with container labels
// https://docs.kernel.org/filesystems/proc.html

var (
	ProcessSmapsSize = prometheus.NewDesc(
		"process_smaps_size_bytes",
		"Total size of the memory mapping in bytes (from Size).",
		smapsLabels, nil,
	)
	ProcessSmapsRss = prometheus.NewDesc(
		"process_smaps_rss_bytes",
		"Resident Set Size: amount of the mapping currently resident in RAM (bytes) (from Rss).",
		smapsLabels, nil,
	)
	ProcessSmapsPss = prometheus.NewDesc(
		"process_smaps_pss_bytes",
		"Proportional Set Size: mapping's share of RAM, divided by number of processes sharing each page (bytes) (from Pss).",
		smapsLabels, nil,
	)
	ProcessSmapsPssDirty = prometheus.NewDesc(
		"process_smaps_pss_dirty_bytes",
		"Proportional Set Size of dirty pages in the mapping (bytes) (from Pss_Dirty).",
		smapsLabels, nil,
	)
	ProcessSmapsSharedClean = prometheus.NewDesc(
		"process_smaps_shared_clean_bytes",
		"Amount of clean shared pages in the mapping (bytes) (from Shared_Clean).",
		smapsLabels, nil,
	)
	ProcessSmapsSharedDirty = prometheus.NewDesc(
		"process_smaps_shared_dirty_bytes",
		"Amount of dirty shared pages in the mapping (bytes) (from Shared_Dirty).",
		smapsLabels, nil,
	)
	ProcessSmapsPrivateClean = prometheus.NewDesc(
		"process_smaps_private_clean_bytes",
		"Amount of clean private pages in the mapping (bytes) (from Private_Clean).",
		smapsLabels, nil,
	)
	ProcessSmapsPrivateDirty = prometheus.NewDesc(
		"process_smaps_private_dirty_bytes",
		"Amount of dirty private pages in the mapping (bytes) (from Private_Dirty).",
		smapsLabels, nil,
	)
	ProcessSmapsReferenced = prometheus.NewDesc(
		"process_smaps_referenced_bytes",
		"Amount of memory in the mapping currently marked as referenced or accessed (bytes) (from Referenced).",
		smapsLabels, nil,
	)
	ProcessSmapsAnonymous = prometheus.NewDesc(
		"process_smaps_anonymous_bytes",
		"Amount of memory in the mapping that does not belong to any file (bytes) (from Anonymous).",
		smapsLabels, nil,
	)
	ProcessSmapsLazyFree = prometheus.NewDesc(
		"process_smaps_lazyfree_bytes",
		"Amount of memory in the mapping marked by madvise(MADV_FREE), to be freed under memory pressure (bytes) (from LazyFree).",
		smapsLabels, nil,
	)
	ProcessSmapsAnonHugePages = prometheus.NewDesc(
		"process_smaps_anon_hugepages_bytes",
		"Amount of memory in the mapping backed by transparent hugepages (bytes) (from AnonHugePages).",
		smapsLabels, nil,
	)
	ProcessSmapsShmemPmdMapped = prometheus.NewDesc(
		"process_smaps_shmem_pmdmapped_bytes",
		"Amount of shared (shmem/tmpfs) memory in the mapping backed by huge pages (bytes) (from ShmemPmdMapped).",
		smapsLabels, nil,
	)
	ProcessSmapsSharedHugetlb = prometheus.NewDesc(
		"process_smaps_shared_hugetlb_bytes",
		"Amount of memory in the mapping backed by hugetlbfs pages and shared (bytes) (from Shared_Hugetlb).",
		smapsLabels, nil,
	)
	ProcessSmapsPrivateHugetlb = prometheus.NewDesc(
		"process_smaps_private_hugetlb_bytes",
		"Amount of memory in the mapping backed by hugetlbfs pages and private (bytes) (from Private_Hugetlb).",
		smapsLabels, nil,
	)
	ProcessSmapsSwap = prometheus.NewDesc(
		"process_smaps_swap_bytes",
		"Amount of would-be-anonymous memory in the mapping that is swapped out (bytes) (from Swap).",
		smapsLabels, nil,
	)
	ProcessSmapsSwapPss = prometheus.NewDesc(
		"process_smaps_swap_pss_bytes",
		"Proportional share of swap space used by the mapping (bytes) (from SwapPss).",
		smapsLabels, nil,
	)
	ProcessSmapsKernelPageSize = prometheus.NewDesc(
		"process_smaps_kernel_page_size_bytes",
		"Kernel page size used for the mapping (bytes) (from KernelPageSize).",
		smapsLabels, nil,
	)
	ProcessSmapsMMUPageSize = prometheus.NewDesc(
		"process_smaps_mmu_page_size_bytes",
		"MMU page size used for the mapping (bytes) (from MMUPageSize).",
		smapsLabels, nil,
	)
	ProcessSmapsLocked = prometheus.NewDesc(
		"process_smaps_locked_bytes",
		"Amount of memory in the mapping that is locked in RAM (bytes) (from Locked).",
		smapsLabels, nil,
	)
)

// Disk (filesystem) metrics per container mountpoint

var (
	MountpointCapacityBytes = prometheus.NewDesc(
		"container_mountpoint_capacity_bytes",
		"Total capacity of the filesystem at the mountpoint in bytes (f_blocks * f_bsize).",
		mountpointLabels, nil,
	)
	MountpointAvailableBytes = prometheus.NewDesc(
		"container_mountpoint_available_bytes",
		"Available space on the filesystem at the mountpoint in bytes (f_bavail * f_bsize).",
		mountpointLabels, nil,
	)
	MountpointUsedBytes = prometheus.NewDesc(
		"container_mountpoint_used_bytes",
		"Used space on the filesystem at the mountpoint in bytes (capacity - available).",
		mountpointLabels, nil,
	)
)

// File size metrics for individually monitored files

var (
	FileSizeBytes = prometheus.NewDesc(
		"container_file_size_bytes",
		"Total apparent size of files matching the configured pattern in bytes (sum of st_size from stat).",
		fileLabels, nil,
	)
	FileDiskUsageBytes = prometheus.NewDesc(
		"container_file_disk_usage_bytes",
		"Total actual disk space used by files matching the configured pattern in bytes (sum of st_blocks * 512 from stat).",
		fileLabels, nil,
	)
)

// Cgroup v2 I/O metrics (from io.stat)

var (
	CgroupIoReadBytes = prometheus.NewDesc(
		"cgroup_io_read_bytes",
		"Total number of bytes read from block devices by the cgroup (from io.stat:rbytes).",
		ioStatLabels, nil,
	)
	CgroupIoWriteBytes = prometheus.NewDesc(
		"cgroup_io_write_bytes",
		"Total number of bytes written to block devices by the cgroup (from io.stat:wbytes).",
		ioStatLabels, nil,
	)
	CgroupIoReadOps = prometheus.NewDesc(
		"cgroup_io_read_ops",
		"Total number of read I/O operations issued to block devices by the cgroup (from io.stat:rios).",
		ioStatLabels, nil,
	)
	CgroupIoWriteOps = prometheus.NewDesc(
		"cgroup_io_write_ops",
		"Total number of write I/O operations issued to block devices by the cgroup (from io.stat:wios).",
		ioStatLabels, nil,
	)
	CgroupIoDiscardBytes = prometheus.NewDesc(
		"cgroup_io_discard_bytes",
		"Total number of bytes discarded (TRIM) on block devices by the cgroup (from io.stat:dbytes).",
		ioStatLabels, nil,
	)
	CgroupIoDiscardOps = prometheus.NewDesc(
		"cgroup_io_discard_ops",
		"Total number of discard I/O operations issued to block devices by the cgroup (from io.stat:dios).",
		ioStatLabels, nil,
	)
)

// Per-process I/O metrics (from /proc/[pid]/io)

var (
	ProcessIoRchar = prometheus.NewDesc(
		"process_io_rchar_bytes",
		"Total bytes read via read-like syscalls, including those served from page cache (from /proc/[pid]/io:rchar).",
		processLabels, nil,
	)
	ProcessIoWchar = prometheus.NewDesc(
		"process_io_wchar_bytes",
		"Total bytes written via write-like syscalls, including those that do not hit storage (from /proc/[pid]/io:wchar).",
		processLabels, nil,
	)
	ProcessIoSyscr = prometheus.NewDesc(
		"process_io_syscr",
		"Total number of read I/O syscalls (from /proc/[pid]/io:syscr).",
		processLabels, nil,
	)
	ProcessIoSyscw = prometheus.NewDesc(
		"process_io_syscw",
		"Total number of write I/O syscalls (from /proc/[pid]/io:syscw).",
		processLabels, nil,
	)
	ProcessIoReadBytes = prometheus.NewDesc(
		"process_io_read_bytes",
		"Total bytes that actually resulted in block device I/O for reads (from /proc/[pid]/io:read_bytes).",
		processLabels, nil,
	)
	ProcessIoWriteBytes = prometheus.NewDesc(
		"process_io_write_bytes",
		"Total bytes that actually resulted in block device I/O for writes (from /proc/[pid]/io:write_bytes).",
		processLabels, nil,
	)
)

// describeAll sends the descriptors of all metrics exported by the collector.
func describeAll(ch chan<- *prometheus.Desc) {
	// Cgroup metrics
	for _, m := range cgroupMetrics {
		ch <- m.desc
	}
	for _, m := range cgroupMultiValueMetrics {
		for _, d := range m.descs {
			ch <- d
		}
	}

	// Smaps metrics
	ch <- ProcessSmapsSize
	ch <- ProcessSmapsRss
	ch <- ProcessSmapsPss
	ch <- ProcessSmapsPssDirty
	ch <- ProcessSmapsSharedClean
	ch <- ProcessSmapsSharedDirty
	ch <- ProcessSmapsPrivateClean
	ch <- ProcessSmapsPrivateDirty
	ch <- ProcessSmapsReferenced
	ch <- ProcessSmapsAnonymous
	ch <- ProcessSmapsLazyFree
	ch <- ProcessSmapsAnonHugePages
	ch <- ProcessSmapsShmemPmdMapped
	ch <- ProcessSmapsSharedHugetlb
	ch <- ProcessSmapsPrivateHugetlb
	ch <- ProcessSmapsSwap
	ch <- ProcessSmapsSwapPss
	ch <- ProcessSmapsKernelPageSize
	ch <- ProcessSmapsMMUPageSize
	ch <- ProcessSmapsLocked

	// Disk metrics
	ch <- MountpointCapacityBytes
	ch <- MountpointAvailableBytes
	ch <- MountpointUsedBytes

	// File metrics
	ch <- FileSizeBytes
	ch <- FileDiskUsageBytes

	// Cgroup I/O metrics
	ch <- CgroupIoReadBytes
	ch <- CgroupIoWriteBytes
	ch <- CgroupIoReadOps
	ch <- CgroupIoWriteOps
	ch <- CgroupIoDiscardBytes
	ch <- CgroupIoDiscardOps

	// Process I/O metrics
	ch <- ProcessIoRchar
	ch <- ProcessIoWchar
	ch <- ProcessIoSyscr
	ch <- ProcessIoSyscw
	ch <- ProcessIoReadBytes
	ch <- ProcessIoWriteBytes
}
