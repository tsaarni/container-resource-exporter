package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metric struct {
	gauge           *prometheus.GaugeVec
	counter         *prometheus.CounterVec
	cgroupFile      string
	cgroupFileField string
}

// Cgroup v2 metrics
// https://docs.kernel.org/admin-guide/cgroup-v2.html

var cgroupMetrics = []Metric{
	// Memory
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_memory_current_bytes",
				Help: "Total memory currently used by the cgroup and its descendants, in bytes (from memory.current).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile: "memory.current",
	},
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_memory_peak_bytes",
				Help: "Maximum memory usage recorded for the cgroup and its descendants since creation or last reset (from memory.peak).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile: "memory.peak",
	},
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_memory_low_bytes",
				Help: "Best-effort memory protection threshold below which memory is not reclaimed (from memory.low).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile: "memory.low",
	},
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_memory_high_bytes",
				Help: "Memory usage throttle limit above which processes are throttled and put under reclaim pressure (from memory.high).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile: "memory.high",
	},
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_memory_max_bytes",
				Help: "Hard memory usage limit for the cgroup; exceeding this may trigger OOM killer (from memory.max).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile: "memory.max",
	},
	// memory.stat fields
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_memory_stat_anon_bytes",
				Help: "Amount of memory used in anonymous mappings such as brk(), sbrk(), and mmap(MAP_ANONYMOUS) (from memory.stat:anon).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "memory.stat",
		cgroupFileField: "anon",
	},
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_memory_stat_file_bytes",
				Help: "Amount of memory used to cache filesystem data, including tmpfs and shared memory (from memory.stat:file).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "memory.stat",
		cgroupFileField: "file",
	},
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_memory_stat_shmem_bytes",
				Help: "Amount of cached filesystem data that is swap-backed, such as tmpfs, shm segments, and shared anonymous mmap()s (from memory.stat:shmem).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "memory.stat",
		cgroupFileField: "shmem",
	},
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_memory_stat_kernel_bytes",
				Help: "Total kernel memory usage, including kernel_stack, pagetables, percpu, vmalloc, and slab (from memory.stat:kernel).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "memory.stat",
		cgroupFileField: "kernel",
	},
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_memory_stat_slab_bytes",
				Help: "Amount of memory used for storing in-kernel data structures (from memory.stat:slab).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "memory.stat",
		cgroupFileField: "slab",
	},
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_memory_stat_slab_reclaimable_bytes",
				Help: "Part of slab memory that might be reclaimed, such as dentries and inodes (from memory.stat:slab_reclaimable).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "memory.stat",
		cgroupFileField: "slab_reclaimable",
	},
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_memory_stat_slab_unreclaimable_bytes",
				Help: "Part of slab memory that cannot be reclaimed on memory pressure (from memory.stat:slab_unreclaimable).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "memory.stat",
		cgroupFileField: "slab_unreclaimable",
	},
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_memory_stat_pagetables_bytes",
				Help: "Amount of memory allocated for page tables (from memory.stat:pagetables).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "memory.stat",
		cgroupFileField: "pagetables",
	},
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_memory_stat_kernel_stack_bytes",
				Help: "Amount of memory allocated to kernel stacks (from memory.stat:kernel_stack).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "memory.stat",
		cgroupFileField: "kernel_stack",
	},
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_memory_stat_active_anon_bytes",
				Help: "Amount of active anonymous memory on the internal memory management lists (from memory.stat:active_anon).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "memory.stat",
		cgroupFileField: "active_anon",
	},
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_memory_stat_inactive_anon_bytes",
				Help: "Amount of inactive anonymous memory on the internal memory management lists (from memory.stat:inactive_anon).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "memory.stat",
		cgroupFileField: "inactive_anon",
	},
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_memory_stat_active_file_bytes",
				Help: "Amount of active file-backed memory on the internal memory management lists (from memory.stat:active_file).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "memory.stat",
		cgroupFileField: "active_file",
	},
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_memory_stat_inactive_file_bytes",
				Help: "Amount of inactive file-backed memory on the internal memory management lists (from memory.stat:inactive_file).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "memory.stat",
		cgroupFileField: "inactive_file",
	},
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_memory_stat_unevictable_bytes",
				Help: "Amount of unevictable memory (from memory.stat:unevictable).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "memory.stat",
		cgroupFileField: "unevictable",
	},
	{
		counter: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cgroup_memory_stat_pgfault_total",
				Help: "Total number of page faults incurred by the cgroup (from memory.stat:pgfault).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "memory.stat",
		cgroupFileField: "pgfault",
	},
	{
		counter: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cgroup_memory_stat_pgmajfault_total",
				Help: "Number of major page faults incurred by the cgroup (from memory.stat:pgmajfault).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "memory.stat",
		cgroupFileField: "pgmajfault",
	},
	// CPU
	{
		counter: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cgroup_cpu_usage_usec",
				Help: "Total CPU time consumed by all processes in the cgroup, in microseconds (from cpu.stat:usage_usec).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "cpu.stat",
		cgroupFileField: "usage_usec",
	},
	{
		counter: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cgroup_cpu_user_usec",
				Help: "Total user mode CPU time consumed by the cgroup, in microseconds (from cpu.stat:user_usec).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "cpu.stat",
		cgroupFileField: "user_usec",
	},
	{
		counter: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cgroup_cpu_system_usec",
				Help: "Total system (kernel) mode CPU time consumed by the cgroup, in microseconds (from cpu.stat:system_usec).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "cpu.stat",
		cgroupFileField: "system_usec",
	},
	{
		counter: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cgroup_cpu_nr_periods_total",
				Help: "Number of enforcement intervals (periods) for CPU bandwidth (from cpu.stat:nr_periods).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "cpu.stat",
		cgroupFileField: "nr_periods",
	},
	{
		counter: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cgroup_cpu_nr_throttled_total",
				Help: "Number of periods in which the cgroup was throttled due to CPU quota (from cpu.stat:nr_throttled).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "cpu.stat",
		cgroupFileField: "nr_throttled",
	},
	{
		counter: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cgroup_cpu_throttled_usec_total",
				Help: "Total time duration in microseconds that the cgroup was throttled due to CPU quota (from cpu.stat:throttled_usec).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile:      "cpu.stat",
		cgroupFileField: "throttled_usec",
	},
	// PIDs
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_pids_current",
				Help: "Number of processes currently in the cgroup and its descendants (from pids.current).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile: "pids.current",
	},
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_pids_max",
				Help: "Hard limit on the number of processes allowed in the cgroup (from pids.max).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile: "pids.max",
	},
	{
		gauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cgroup_pids_peak",
				Help: "Maximum number of processes ever present in the cgroup and its descendants (from pids.peak).",
			},
			[]string{"namespace", "pod", "container"},
		),
		cgroupFile: "pids.peak",
	},
}

// Smaps metrics - enhanced with container labels
// https://docs.kernel.org/filesystems/proc.html

var (
	ProcessSmapsSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_size_bytes",
			Help: "Total size of the memory mapping in bytes (from Size).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
	ProcessSmapsRss = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_rss_bytes",
			Help: "Resident Set Size: amount of the mapping currently resident in RAM (bytes) (from Rss).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
	ProcessSmapsPss = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_pss_bytes",
			Help: "Proportional Set Size: mapping's share of RAM, divided by number of processes sharing each page (bytes) (from Pss).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
	ProcessSmapsPssDirty = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_pss_dirty_bytes",
			Help: "Proportional Set Size of dirty pages in the mapping (bytes) (from Pss_Dirty).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
	ProcessSmapsSharedClean = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_shared_clean_bytes",
			Help: "Amount of clean shared pages in the mapping (bytes) (from Shared_Clean).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
	ProcessSmapsSharedDirty = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_shared_dirty_bytes",
			Help: "Amount of dirty shared pages in the mapping (bytes) (from Shared_Dirty).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
	ProcessSmapsPrivateClean = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_private_clean_bytes",
			Help: "Amount of clean private pages in the mapping (bytes) (from Private_Clean).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
	ProcessSmapsPrivateDirty = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_private_dirty_bytes",
			Help: "Amount of dirty private pages in the mapping (bytes) (from Private_Dirty).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
	ProcessSmapsReferenced = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_referenced_bytes",
			Help: "Amount of memory in the mapping currently marked as referenced or accessed (bytes) (from Referenced).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
	ProcessSmapsAnonymous = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_anonymous_bytes",
			Help: "Amount of memory in the mapping that does not belong to any file (bytes) (from Anonymous).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
	ProcessSmapsLazyFree = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_lazyfree_bytes",
			Help: "Amount of memory in the mapping marked by madvise(MADV_FREE), to be freed under memory pressure (bytes) (from LazyFree).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
	ProcessSmapsAnonHugePages = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_anon_hugepages_bytes",
			Help: "Amount of memory in the mapping backed by transparent hugepages (bytes) (from AnonHugePages).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
	ProcessSmapsShmemPmdMapped = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_shmem_pmdmapped_bytes",
			Help: "Amount of shared (shmem/tmpfs) memory in the mapping backed by huge pages (bytes) (from ShmemPmdMapped).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
	ProcessSmapsSharedHugetlb = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_shared_hugetlb_bytes",
			Help: "Amount of memory in the mapping backed by hugetlbfs pages and shared (bytes) (from Shared_Hugetlb).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
	ProcessSmapsPrivateHugetlb = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_private_hugetlb_bytes",
			Help: "Amount of memory in the mapping backed by hugetlbfs pages and private (bytes) (from Private_Hugetlb).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
	ProcessSmapsSwap = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_swap_bytes",
			Help: "Amount of would-be-anonymous memory in the mapping that is swapped out (bytes) (from Swap).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
	ProcessSmapsSwapPss = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_swap_pss_bytes",
			Help: "Proportional share of swap space used by the mapping (bytes) (from SwapPss).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
	ProcessSmapsKernelPageSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_kernel_page_size_bytes",
			Help: "Kernel page size used for the mapping (bytes) (from KernelPageSize).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
	ProcessSmapsMMUPageSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_mmu_page_size_bytes",
			Help: "MMU page size used for the mapping (bytes) (from MMUPageSize).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
	ProcessSmapsLocked = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_smaps_locked_bytes",
			Help: "Amount of memory in the mapping that is locked in RAM (bytes) (from Locked).",
		},
		[]string{"namespace", "pod", "container", "host_pid", "ns_pid", "comm", "path"},
	)
)
