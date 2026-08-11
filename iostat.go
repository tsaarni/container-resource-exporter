package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// IoStat represents I/O statistics for a single block device from cgroup v2 io.stat.
type IoStat struct {
	Major  int
	Minor  int
	Rbytes int64
	Wbytes int64
	Rios   int64
	Wios   int64
	Dbytes int64
	Dios   int64
}

// ReadIoStat reads and parses the io.stat file from the cgroup directory.
// Format: "major:minor rbytes=N wbytes=N rios=N wios=N dbytes=N dios=N"
// Each line represents one block device.
func (c *CGroup) ReadIoStat() ([]IoStat, error) {
	data, err := os.ReadFile(filepath.Join(c.path, "io.stat"))
	if err != nil {
		return nil, fmt.Errorf("error reading io.stat: %w", err)
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil, nil
	}

	lines := strings.Split(content, "\n")
	stats := make([]IoStat, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		stat, err := parseIoStatLine(line)
		if err != nil {
			return nil, fmt.Errorf("error parsing io.stat line %q: %w", line, err)
		}
		stats = append(stats, stat)
	}

	return stats, nil
}

// parseIoStatLine parses a single line from io.stat.
// Example: "259:0 rbytes=1234 wbytes=5678 rios=10 wios=20 dbytes=0 dios=0"
func parseIoStatLine(line string) (IoStat, error) {
	fields := strings.Fields(line)
	if len(fields) < 1 {
		return IoStat{}, fmt.Errorf("empty line")
	}

	// Parse "major:minor"
	devParts := strings.SplitN(fields[0], ":", 2)
	if len(devParts) != 2 {
		return IoStat{}, fmt.Errorf("invalid device format: %s", fields[0])
	}

	major, err := strconv.Atoi(devParts[0])
	if err != nil {
		return IoStat{}, fmt.Errorf("invalid major number: %w", err)
	}
	minor, err := strconv.Atoi(devParts[1])
	if err != nil {
		return IoStat{}, fmt.Errorf("invalid minor number: %w", err)
	}

	stat := IoStat{Major: major, Minor: minor}

	// Parse key=value pairs
	for _, field := range fields[1:] {
		kv := strings.SplitN(field, "=", 2)
		if len(kv) != 2 {
			slog.Debug("Skipping malformed io.stat field", "field", field)
			continue
		}

		val, err := strconv.ParseInt(kv[1], 10, 64)
		if err != nil {
			slog.Debug("Failed to parse io.stat value", "key", kv[0], "value", kv[1], "error", err)
			continue
		}

		switch kv[0] {
		case "rbytes":
			stat.Rbytes = val
		case "wbytes":
			stat.Wbytes = val
		case "rios":
			stat.Rios = val
		case "wios":
			stat.Wios = val
		case "dbytes":
			stat.Dbytes = val
		case "dios":
			stat.Dios = val
		}
	}

	return stat, nil
}
