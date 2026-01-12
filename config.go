package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server         ServerConfig      `yaml:"server"`
	Paths          PathsConfig       `yaml:"paths"`
	ScrapeInterval string            `yaml:"scrape_interval"`
	LogLevel       string            `yaml:"log_level"`
	Filters        []ContainerFilter `yaml:"filters"`
}

type ServerConfig struct {
	Address string `yaml:"address"`
}

type PathsConfig struct {
	Cgroup    string `yaml:"cgroup"`
	Proc      string `yaml:"proc"`
	CRISocket string `yaml:"cri_socket"`
}

type ContainerFilter struct {
	Namespace string `yaml:"namespace"`
	Pod       string `yaml:"pod"`
	Container string `yaml:"container"`
	Command   string `yaml:"command"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	config.applyDefaults()

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// applyDefaults sets default values for optional fields that were not specified in the config.
func (c *Config) applyDefaults() {
	if c.Server.Address == "" {
		c.Server.Address = ":8080"
	}

	if c.Paths.Cgroup == "" {
		c.Paths.Cgroup = "/sys/fs/cgroup"
	}

	if c.Paths.Proc == "" {
		c.Paths.Proc = "/proc"
	}

	if c.Paths.CRISocket == "" {
		// Auto-detect CRI socket from common locations
		c.Paths.CRISocket = detectCRISocket()
	}

	if c.ScrapeInterval == "" {
		c.ScrapeInterval = "1s"
	}

	if c.LogLevel == "" {
		c.LogLevel = "info"
	}

	// Add default wildcard command filter to each container filter if not specified.
	for i := range c.Filters {
		if c.Filters[i].Command == "" {
			c.Filters[i].Command = "*"
		}
	}
}

func (c *Config) Validate() error {
	if c.Server.Address == "" {
		return fmt.Errorf("server.address is required")
	}

	if c.Paths.Cgroup == "" {
		return fmt.Errorf("paths.cgroup is required")
	}

	if c.Paths.Proc == "" {
		return fmt.Errorf("paths.proc is required")
	}

	if c.Paths.CRISocket == "" {
		return fmt.Errorf("paths.cri_socket was not auto-detected and is required to be specified")
	}

	if _, err := time.ParseDuration(c.ScrapeInterval); err != nil {
		return fmt.Errorf("invalid scrape_interval: %w", err)
	}

	if len(c.Filters) == 0 {
		return fmt.Errorf("at least one container filter is required")
	}

	// Validate that paths exist.
	for _, path := range []struct {
		name string
		path string
	}{
		{"cgroup path", c.Paths.Cgroup},
		{"proc path", c.Paths.Proc},
		{"CRI socket", c.Paths.CRISocket},
	} {
		if path.path == "" {
			continue
		}
		if _, err := os.Stat(path.path); os.IsNotExist(err) {
			return fmt.Errorf("%s does not exist: %s", path.name, path.path)
		}
	}

	return nil
}

// GetScrapeInterval parses and returns the scrape interval as time.Duration.
func (c *Config) GetScrapeInterval() time.Duration {
	d, _ := time.ParseDuration(c.ScrapeInterval)
	return d
}

// MatchesContainer checks if a container matches any of the configured filters.
func (c *Config) MatchesContainer(namespace, pod, container string) bool {
	for _, filter := range c.Filters {
		if matchPattern(filter.Namespace, namespace) &&
			matchPattern(filter.Pod, pod) &&
			matchPattern(filter.Container, container) {
			return true
		}
	}
	return false
}

// MatchesProcess checks if a process command matches the command filter for the given container.
func (c *Config) MatchesProcess(namespace, pod, container, command string) bool {
	for _, filter := range c.Filters {
		if matchPattern(filter.Namespace, namespace) &&
			matchPattern(filter.Pod, pod) &&
			matchPattern(filter.Container, container) {
			// Found matching container filter, check command pattern.
			return matchPattern(filter.Command, command)
		}
	}
	return false
}

// matchPattern matches a pattern against a value, supporting "*" wildcard.
func matchPattern(pattern, value string) bool {
	if pattern == "*" {
		return true
	}

	matched, _ := filepath.Match(pattern, value)
	return matched
}

// detectCRISocket attempts to find the CRI socket from common locations.
func detectCRISocket() string {
	// Try common CRI socket paths in order of prevalence
	commonPaths := []string{
		"/run/containerd/containerd.sock", // containerd
		"/run/crio/crio.sock",             // CRI-O
		"/run/cri-dockerd.sock",           // cri-dockerd
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	slog.Warn("Failed to auto-detect CRI socket from common locations")
	return ""
}
