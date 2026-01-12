package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type KubernetesClient struct {
	criClient runtimeapi.RuntimeServiceClient
	config    *Config
}

type Container struct {
	ID        string
	Namespace string
	Pod       string
	Container string
	PIDs      []ProcessInfo
}

type ProcessInfo struct {
	PID   int
	NSPID int
	Comm  string
}

func NewKubernetesClient(config *Config) (*KubernetesClient, error) {
	slog.Debug("Connecting to CRI socket", "socket", config.Paths.CRISocket)

	conn, err := grpc.NewClient(
		fmt.Sprintf("unix://%s", config.Paths.CRISocket),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to CRI socket: %w", err)
	}

	return &KubernetesClient{
		criClient: runtimeapi.NewRuntimeServiceClient(conn),
		config:    config,
	}, nil
}

func (k *KubernetesClient) DiscoverContainers(ctx context.Context) ([]Container, error) {
	slog.Debug("Discovering containers")

	// List all pods.
	resp, err := k.criClient.ListPodSandbox(ctx, &runtimeapi.ListPodSandboxRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pod sandboxes: %w", err)
	}

	var containers []Container

	for _, pod := range resp.Items {
		if pod.State != runtimeapi.PodSandboxState_SANDBOX_READY {
			continue
		}

		namespace := pod.Metadata.Namespace
		podName := pod.Metadata.Name

		// List containers in this pod.
		containerResp, err := k.criClient.ListContainers(ctx, &runtimeapi.ListContainersRequest{
			Filter: &runtimeapi.ContainerFilter{
				PodSandboxId: pod.Id,
			},
		})
		if err != nil {
			slog.Warn("Failed to list containers for pod", "pod", podName, "error", err)
			continue
		}

		for _, c := range containerResp.Containers {
			if c.State != runtimeapi.ContainerState_CONTAINER_RUNNING {
				continue
			}

			containerName := c.Metadata.Name

			// Apply filter.
			if !k.config.MatchesContainer(namespace, podName, containerName) {
				slog.Debug("Container filtered out", "namespace", namespace, "pod", podName, "container", containerName)
				continue
			}

			containers = append(containers, Container{
				ID:        c.Id,
				Namespace: namespace,
				Pod:       podName,
				Container: containerName,
			})
		}
	}

	// Scan /proc and populate PIDs for all containers.
	k.populateContainerProcesses(containers)

	slog.Info("Container discovery complete", "containers", len(containers))
	return containers, nil
}

// populateContainerProcesses scans /proc once and populates the PIDs field for all containers that match the configured filters.
func (k *KubernetesClient) populateContainerProcesses(containers []Container) {
	entries, err := os.ReadDir(k.config.Paths.Proc)
	if err != nil {
		slog.Warn("Failed to read /proc", "error", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid := entry.Name()
		pidInt, err := strconv.Atoi(pid)
		if err != nil {
			continue // Not a PID directory.
		}

		// Get cgroup and command for this PID.
		cgroup, err := k.readCgroup(pid)
		if err != nil {
			continue
		}

		comm, err := k.getComm(pid)
		if err != nil {
			continue
		}

		nsPID, err := k.getNamespacePID(pid)
		if err != nil {
			continue
		}

		// Check if this process belongs to any of our containers.
		for i := range containers {
			if strings.Contains(cgroup, containers[i].ID) && k.config.MatchesProcess(containers[i].Namespace, containers[i].Pod, containers[i].Container, comm) {
				containers[i].PIDs = append(containers[i].PIDs, ProcessInfo{PID: pidInt, NSPID: nsPID, Comm: comm})
				break // Process belongs to only one container.
			}
		}
	}

	// Log discovered processes for each container.
	for _, container := range containers {
		slog.Debug("Discovered container", "namespace", container.Namespace, "pod", container.Pod, "container", container.Container, "pids", len(container.PIDs))
	}
}

func (k *KubernetesClient) readCgroup(pid string) (string, error) {
	cgroupPath := filepath.Join(k.config.Paths.Proc, pid, "cgroup")
	data, err := os.ReadFile(cgroupPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (k *KubernetesClient) getComm(pid string) (string, error) {
	commPath := filepath.Join(k.config.Paths.Proc, pid, "comm")
	data, err := os.ReadFile(commPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (k *KubernetesClient) getNamespacePID(pid string) (int, error) {
	statusPath := filepath.Join(k.config.Paths.Proc, pid, "status")
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return 0, err
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "NSpid:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				break
			}
			nsPID, err := strconv.Atoi(fields[len(fields)-1])
			if err != nil {
				return 0, err
			}
			return nsPID, nil
		}
	}

	return 0, fmt.Errorf("NSpid not found for pid %s", pid)
}
