package main

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// SmapsMapping describes a memory mapping entry parsed from smaps.
// The AddrRange field is omitted
// Aggregation is performed by (perms, path)
type SmapsMapping struct {
	// Header fields
	AddrRange string
	Perms     string
	Offset    string
	Dev       string
	Inode     string
	Path      string

	// Key-Value fields (all values in bytes)
	SizeBytes           int64
	RssBytes            int64
	PssBytes            int64
	PssDirtyBytes       int64
	SharedCleanBytes    int64
	SharedDirtyBytes    int64
	PrivateCleanBytes   int64
	PrivateDirtyBytes   int64
	ReferencedBytes     int64
	AnonymousBytes      int64
	LazyFreeBytes       int64
	AnonHugePagesBytes  int64
	ShmemPmdMappedBytes int64
	SharedHugetlbBytes  int64
	PrivateHugetlbBytes int64
	SwapBytes           int64
	SwapPssBytes        int64
	KernelPageSizeBytes int64
	MMUPageSizeBytes    int64
	LockedBytes         int64
}

var (
	// AddrRange                 Perms Offset  Dev    Inode                     Path
	// 7d4337f0f000-7d4337f10000 rw-p 0002d000 00:2bc 42926480                  /usr/lib/x86_64-linux-gnu/ld-2.31.so
	headerRe = regexp.MustCompile(`^([0-9a-fA-F]+-[0-9a-fA-F]+) ([rwxps-]{4}) ([0-9a-fA-F]+) ([0-9a-fA-F:]+) (\d+)(?:\s+(.*))?$`)

	// Key: Value kB
	// Size:                  4 kB
	kvRe = regexp.MustCompile(`^([A-Za-z_]+):\s+(\d+) kB`)
)

// ParseSmaps parses the contents of a /proc/[pid]/smaps file.
func ParseSmaps(r io.Reader) ([]*SmapsMapping, error) {
	aggregatedSmaps := make(map[string]*SmapsMapping)

	scanner := bufio.NewScanner(r)
	var mapping *SmapsMapping
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		if matches := headerRe.FindStringSubmatch(line); matches != nil {
			// Start a new mapping

			perms := "----"
			if matches[2] != "" {
				perms = matches[2]
			}
			path := "[anon]"
			if matches[6] != "" {
				path = matches[6]
			}

			tmp := &SmapsMapping{
				AddrRange: matches[1],
				Perms:     perms,
				Offset:    matches[3],
				Dev:       matches[4],
				Inode:     matches[5],
				Path:      strings.TrimSpace(path),
			}

			// Use path as the aggregation key.
			key := tmp.Path

			if existing, found := aggregatedSmaps[key]; found {
				mapping = existing
			} else {
				aggregatedSmaps[key] = tmp
				mapping = tmp
			}

			continue
		}

		if kv := kvRe.FindStringSubmatch(line); kv != nil {
			key := kv[1]
			val, _ := strconv.ParseInt(kv[2], 10, 64)
			valBytes := val * 1024
			switch key {
			case "Size":
				mapping.SizeBytes += valBytes
			case "KernelPageSize":
				mapping.KernelPageSizeBytes += valBytes
			case "MMUPageSize":
				mapping.MMUPageSizeBytes += valBytes
			case "Rss":
				mapping.RssBytes += valBytes
			case "Pss":
				mapping.PssBytes += valBytes
			case "Pss_Dirty":
				mapping.PssDirtyBytes += valBytes
			case "Shared_Clean":
				mapping.SharedCleanBytes += valBytes
			case "Shared_Dirty":
				mapping.SharedDirtyBytes += valBytes
			case "Private_Clean":
				mapping.PrivateCleanBytes += valBytes
			case "Private_Dirty":
				mapping.PrivateDirtyBytes += valBytes
			case "Referenced":
				mapping.ReferencedBytes += valBytes
			case "Anonymous":
				mapping.AnonymousBytes += valBytes
			case "LazyFree":
				mapping.LazyFreeBytes += valBytes
			case "AnonHugePages":
				mapping.AnonHugePagesBytes += valBytes
			case "ShmemPmdMapped":
				mapping.ShmemPmdMappedBytes += valBytes
			case "Shared_Hugetlb":
				mapping.SharedHugetlbBytes += valBytes
			case "Private_Hugetlb":
				mapping.PrivateHugetlbBytes += valBytes
			case "Swap":
				mapping.SwapBytes += valBytes
			case "SwapPss":
				mapping.SwapPssBytes += valBytes
			case "Locked":
				mapping.LockedBytes += valBytes
			}

		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan error: %w", err)
	}

	// Convert map to slice
	result := make([]*SmapsMapping, 0, len(aggregatedSmaps))
	for _, m := range aggregatedSmaps {
		result = append(result, m)
	}

	return result, nil
}
