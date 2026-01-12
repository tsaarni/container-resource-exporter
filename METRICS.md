# Supported Metrics

This document describes all metrics exported by `container-resource-exporter`.

## Cgroup v2 Metrics

These metrics are based on Linux cgroup v2 and are available for each Kubernetes namespace, pod, and container.

The `(from ...)` in descriptions tells the source for the metric within the Linux cgroup v2 filesystem:
- Single file: `(from memory.current)` - metric is read directly from the cgroup v2 `memory.current` file.
- Field from file: `(from memory.stat:anon)` - metric is read from the `anon` field in the cgroup v2 `memory.stat` file.

### Memory Metrics

Labels: `namespace`, `pod`, `container`

| Metric Name | Type | Description |
|---|---|---|
| `cgroup_memory_current_bytes` | Gauge | Total memory currently used by the cgroup and its descendants, in bytes (from `memory.current`). |
| `cgroup_memory_peak_bytes` | Gauge | Maximum memory usage recorded for the cgroup and its descendants since creation or last reset (from `memory.peak`). |
| `cgroup_memory_low_bytes` | Gauge | Best-effort memory protection threshold below which memory is not reclaimed (from `memory.low`). |
| `cgroup_memory_high_bytes` | Gauge | Memory usage throttle limit above which processes are throttled and put under reclaim pressure (from `memory.high`). |
| `cgroup_memory_max_bytes` | Gauge | Hard memory usage limit for the cgroup; exceeding this may trigger OOM killer (from `memory.max`). |
| `cgroup_memory_stat_anon_bytes` | Gauge | Amount of memory used in anonymous mappings such as brk(), sbrk(), and mmap(MAP_ANONYMOUS) (from `memory.stat:anon`). |
| `cgroup_memory_stat_file_bytes` | Gauge | Amount of memory used to cache filesystem data, including tmpfs and shared memory (from `memory.stat:file`). |
| `cgroup_memory_stat_shmem_bytes` | Gauge | Amount of cached filesystem data that is swap-backed, such as tmpfs, shm segments, and shared anonymous mmap()s (from `memory.stat:shmem`). |
| `cgroup_memory_stat_kernel_bytes` | Gauge | Total kernel memory usage, including kernel_stack, pagetables, percpu, vmalloc, and slab (from `memory.stat:kernel`). |
| `cgroup_memory_stat_slab_bytes` | Gauge | Amount of memory used for storing in-kernel data structures (from `memory.stat:slab`). |
| `cgroup_memory_stat_slab_reclaimable_bytes` | Gauge | Part of slab memory that might be reclaimed, such as dentries and inodes (from `memory.stat:slab_reclaimable`). |
| `cgroup_memory_stat_slab_unreclaimable_bytes` | Gauge | Part of slab memory that cannot be reclaimed on memory pressure (from `memory.stat:slab_unreclaimable`). |
| `cgroup_memory_stat_pagetables_bytes` | Gauge | Amount of memory allocated for page tables (from `memory.stat:pagetables`). |
| `cgroup_memory_stat_kernel_stack_bytes` | Gauge | Amount of memory allocated to kernel stacks (from `memory.stat:kernel_stack`). |
| `cgroup_memory_stat_active_anon_bytes` | Gauge | Amount of active anonymous memory on the internal memory management lists (from `memory.stat:active_anon`). |
| `cgroup_memory_stat_inactive_anon_bytes` | Gauge | Amount of inactive anonymous memory on the internal memory management lists (from `memory.stat:inactive_anon`). |
| `cgroup_memory_stat_active_file_bytes` | Gauge | Amount of active file-backed memory on the internal memory management lists (from `memory.stat:active_file`). |
| `cgroup_memory_stat_inactive_file_bytes` | Gauge | Amount of inactive file-backed memory on the internal memory management lists (from `memory.stat:inactive_file`). |
| `cgroup_memory_stat_unevictable_bytes` | Gauge | Amount of unevictable memory (from `memory.stat:unevictable`). |
| `cgroup_memory_stat_pgfault_total` | Counter | Total number of page faults incurred by the cgroup (from `memory.stat:pgfault`). |
| `cgroup_memory_stat_pgmajfault_total` | Counter | Number of major page faults incurred by the cgroup (from `memory.stat:pgmajfault`). |

### CPU Metrics

Labels: `namespace`, `pod`, `container`

| Metric Name | Type | Description |
|---|---|---|
| `cgroup_cpu_usage_usec` | Counter | Total CPU time consumed by all processes in the cgroup, in microseconds (from `cpu.stat:usage_usec`). |
| `cgroup_cpu_user_usec` | Counter | Total user mode CPU time consumed by the cgroup, in microseconds (from `cpu.stat:user_usec`). |
| `cgroup_cpu_system_usec` | Counter | Total system (kernel) mode CPU time consumed by the cgroup, in microseconds (from `cpu.stat:system_usec`). |
| `cgroup_cpu_nr_periods_total` | Counter | Number of enforcement intervals (periods) for CPU bandwidth (from `cpu.stat:nr_periods`). |
| `cgroup_cpu_nr_throttled_total` | Counter | Number of periods in which the cgroup was throttled due to CPU quota (from `cpu.stat:nr_throttled`). |
| `cgroup_cpu_throttled_usec_total` | Counter | Total time duration in microseconds that the cgroup was throttled due to CPU quota (from `cpu.stat:throttled_usec`). |

### PID Metrics

Labels: `namespace`, `pod`, `container`

| Metric Name | Type | Description |
|---|---|---|
| `cgroup_pids_current` | Gauge | Number of processes currently in the cgroup and its descendants (from `pids.current`). |
| `cgroup_pids_max` | Gauge | Hard limit on the number of processes allowed in the cgroup (from `pids.max`). |
| `cgroup_pids_peak` | Gauge | Maximum number of processes ever present in the cgroup and its descendants (from `pids.peak`). |

## Smaps Metrics

These metrics provide detailed per-process memory mapping information for all containers being monitored.
Smaps metrics are read from the Linux `/proc/<pid>/smaps` file.

The `(from ...)` in descriptions tells the source of the metric within the `smaps` file.

The `path` label in smaps metrics refers to the file path associated with each memory mapping in a process.
It can contain:
- A real file path (e.g., `/usr/lib/x86_64-linux-gnu/libc.so.6`) - the file backing the memory mapping
- `[anon]` - for anonymous memory mappings (heap, stack, mmap with MAP_ANONYMOUS)

This label allows you to break down memory usage by the files or memory types being used. For example, you can see metrics showing how much memory a shared library is consuming or how much memory is allocated to the heap.

Labels: `namespace`, `pod`, `container`, `host_pid`, `ns_pid`, `comm`, `path`

| Metric Name | Type | Description |
|---|---|---|
| `process_smaps_size_bytes` | Gauge | Total size of the memory mapping in bytes (from `Size`). |
| `process_smaps_rss_bytes` | Gauge | Resident Set Size: amount of the mapping currently resident in RAM in bytes (from `Rss`). |
| `process_smaps_pss_bytes` | Gauge | Proportional Set Size: mapping's share of RAM, divided by number of processes sharing each page in bytes (from `Pss`). |
| `process_smaps_pss_dirty_bytes` | Gauge | Proportional Set Size of dirty pages in the mapping in bytes (from `Pss_Dirty`). |
| `process_smaps_shared_clean_bytes` | Gauge | Amount of clean shared pages in the mapping in bytes (from `Shared_Clean`). |
| `process_smaps_shared_dirty_bytes` | Gauge | Amount of dirty shared pages in the mapping in bytes (from `Shared_Dirty`). |
| `process_smaps_private_clean_bytes` | Gauge | Amount of clean private pages in the mapping in bytes (from `Private_Clean`). |
| `process_smaps_private_dirty_bytes` | Gauge | Amount of dirty private pages in the mapping in bytes (from `Private_Dirty`). |
| `process_smaps_referenced_bytes` | Gauge | Amount of memory in the mapping currently marked as referenced or accessed in bytes (from `Referenced`). |
| `process_smaps_anonymous_bytes` | Gauge | Amount of memory in the mapping that does not belong to any file in bytes (from `Anonymous`). |
| `process_smaps_lazyfree_bytes` | Gauge | Amount of memory in the mapping marked by madvise(MADV_FREE), to be freed under memory pressure in bytes (from `LazyFree`). |
| `process_smaps_anon_hugepages_bytes` | Gauge | Amount of memory in the mapping backed by transparent hugepages in bytes (from `AnonHugePages`). |
| `process_smaps_shmem_pmdmapped_bytes` | Gauge | Amount of shared (shmem/tmpfs) memory in the mapping backed by huge pages in bytes (from `ShmemPmdMapped`). |
| `process_smaps_shared_hugetlb_bytes` | Gauge | Amount of memory in the mapping backed by hugetlbfs pages and shared in bytes (from `Shared_Hugetlb`). |
| `process_smaps_private_hugetlb_bytes` | Gauge | Amount of memory in the mapping backed by hugetlbfs pages and private in bytes (from `Private_Hugetlb`). |
| `process_smaps_swap_bytes` | Gauge | Amount of would-be-anonymous memory in the mapping that is swapped out in bytes (from `Swap`). |
| `process_smaps_swap_pss_bytes` | Gauge | Proportional share of swap space used by the mapping in bytes (from `SwapPss`). |
| `process_smaps_kernel_page_size_bytes` | Gauge | Kernel page size used for the mapping in bytes (from `KernelPageSize`). |
| `process_smaps_mmu_page_size_bytes` | Gauge | MMU page size used for the mapping in bytes (from `MMUPageSize`). |
| `process_smaps_locked_bytes` | Gauge | Amount of memory in the mapping that is locked in RAM in bytes (from `Locked`). |

## References

- [Linux cgroup v2 documentation](https://docs.kernel.org/admin-guide/cgroup-v2.html)
- [Linux /proc filesystem documentation](https://docs.kernel.org/filesystems/proc.html)
