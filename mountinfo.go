package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// MountInfo represents a single entry from /proc/<pid>/mountinfo.
type MountInfo struct {
	MountPoint string
	FsType     string
	Source     string
	Root       string
}

// ParseMountInfo parses /proc/<pid>/mountinfo and returns mount entries.
// Format: https://www.kernel.org/doc/Documentation/filesystems/proc.txt
// Fields: mountID parentID major:minor root mountPoint options [optional fields...] - fsType source superOptions
func ParseMountInfo(r io.Reader) ([]MountInfo, error) {
	var mounts []MountInfo
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		// Find the separator "-" which precedes fstype, source, super_options
		sepIdx := -1
		for i, f := range fields {
			if f == "-" {
				sepIdx = i
				break
			}
		}
		if sepIdx < 0 || sepIdx+2 >= len(fields) {
			continue
		}
		if len(fields) < 5 {
			continue
		}
		mounts = append(mounts, MountInfo{
			Root:       fields[3],
			MountPoint: fields[4],
			FsType:     fields[sepIdx+1],
			Source:     fields[sepIdx+2],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading mountinfo: %w", err)
	}
	return mounts, nil
}
