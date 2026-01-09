package main

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type CGroup struct {
	path string
}

// FindCgroup searches cgroupv2 directories recursively under the given host root path
// for a directory name that contains the specified container ID and ends with ".scope".
//
// cgroupv2RootPath: The path of the cgroup v2 filesystem.
// id: The ID to search for in the cgroup directory names.
func FindCgroup(cgroupv2RootPath, id string) (*CGroup, error) {
	searchPath := cgroupv2RootPath
	slog.Debug("Searching for cgroup sandbox", "id", id, "searchPath", searchPath)

	// Recursively search for the cgroup path matching the container ID.
	var directories []string
	err := filepath.WalkDir(searchPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && strings.Contains(d.Name(), id) && strings.HasSuffix(d.Name(), ".scope") {
			directories = append(directories, path)
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error searching cgroup path: %w", err)
	}

	if len(directories) == 0 {
		slog.Debug("No cgroup directories found", "id", id, "searchPath", searchPath)
		return nil, fmt.Errorf("cgroup path not found for id: %s", id)
	}

	// TODO: Maybe this approach is too simple? Just pick the first one if multiple are found.
	found := directories[0]
	if len(directories) > 1 {
		slog.Warn("Multiple cgroup directories found, using the first one", "id", id, "searchPath", searchPath, "found", directories)
	}

	if found == "" {
		slog.Debug("Cgroup path not found", "id", id, "searchPath", searchPath)
		return nil, fmt.Errorf("cgroup path not found for id: %s", id)
	}

	slog.Debug("Cgroup path found", "path", found)
	return &CGroup{path: found}, nil
}

// ReadInteger reads the content of the specified file within the cgroup directory.
func (c *CGroup) ReadInteger(fileName string) (int, error) {
	slog.Debug("Reading cgroup file", "path", filepath.Join(c.path, fileName))
	rawData, err := os.ReadFile(filepath.Join(c.path, fileName))
	if err != nil {
		return 0, fmt.Errorf("error reading cgroup file: %w", err)
	}

	data := strings.TrimSpace(string(rawData))
	slog.Debug("Cgroup file data", "file", fileName, "data", data)

	if data == "max" {
		return -1, nil // Indicate no limit with -1
	}

	value, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, fmt.Errorf("error converting cgroup file data to int: %w", err)
	}

	return value, nil
}

// ReadIntegerField reads a specific field from a cgroup file that contains key-value pairs.
func (c *CGroup) ReadIntegerField(fileName, field string) (int, error) {
	slog.Debug("Reading cgroup file field", "path", filepath.Join(c.path, fileName), "field", field)
	rawData, err := os.ReadFile(filepath.Join(c.path, fileName))
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(rawData), "\n")

	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[0] == field {
			slog.Debug("Cgroup file field data", "file", fileName, "field", field, "data", parts[1])
			return strconv.Atoi(parts[1])
		}
	}

	slog.Debug("Cgroup file field not found", "file", fileName, "field", field)
	return 0, fmt.Errorf("field %s not found in file %s", field, fileName)
}
