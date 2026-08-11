package main

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
)

// ProcIO represents I/O statistics for a process from /proc/[pid]/io.
type ProcIO struct {
	Rchar      int64
	Wchar      int64
	Syscr      int64
	Syscw      int64
	ReadBytes  int64
	WriteBytes int64
}

// ParseProcIO parses the contents of a /proc/[pid]/io file.
// Format:
//
//	rchar: 12345
//	wchar: 67890
//	syscr: 100
//	syscw: 200
//	read_bytes: 4096
//	write_bytes: 8192
//	cancelled_write_bytes: 0
func ParseProcIO(r io.Reader) (*ProcIO, error) {
	procIO := &ProcIO{}
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			slog.Debug("Failed to parse proc io value", "key", key, "value", strings.TrimSpace(parts[1]), "error", err)
			continue
		}

		switch key {
		case "rchar":
			procIO.Rchar = val
		case "wchar":
			procIO.Wchar = val
		case "syscr":
			procIO.Syscr = val
		case "syscw":
			procIO.Syscw = val
		case "read_bytes":
			procIO.ReadBytes = val
		case "write_bytes":
			procIO.WriteBytes = val
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan error: %w", err)
	}

	return procIO, nil
}
