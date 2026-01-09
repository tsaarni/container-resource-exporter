package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type KubernetesClient struct {
	criClient        runtimeapi.RuntimeServiceClient
	containerdClient *containerd.Client
	config           *Config
}

type Container struct {
	ID        string
	Namespace string
	Pod       string
	Container string
	PIDs      []ProcessInfo
}

type ProcessInfo struct {
	PID  int
	Comm string
}

func NewKubernetesClient(config *Config) (*KubernetesClient, error) {
	slog.Debug("Connecting to containerd", "socket", config.Paths.ContainerdSocket)

	conn, err := grpc.NewClient(
		fmt.Sprintf("unix://%s", config.Paths.ContainerdSocket),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to CRI socket: %w", err)
	}

	containerdClient, err := containerd.New(config.Paths.ContainerdSocket)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}

	return &KubernetesClient{
		criClient:        runtimeapi.NewRuntimeServiceClient(conn),
		containerdClient: containerdClient,
		config:           config,
	}, nil
}

func (k *KubernetesClient) DiscoverContainers(ctx context.Context) ([]Container, error) {
	slog.Debug("Discovering containers")

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

		// List containers in this pod
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

			// Apply filter
			if !k.config.MatchesContainer(namespace, podName, containerName) {
				slog.Debug("Container filtered out", "namespace", namespace, "pod", podName, "container", containerName)
				continue
			}

			// Get PIDs for this container
			pids, err := k.getProcessesForContainer(ctx, c.Id, namespace, podName, containerName)
			if err != nil {
				slog.Warn("Failed to get processes for container", "container", containerName, "error", err)
				pids = []ProcessInfo{} // Continue with empty PID list
			}

			containers = append(containers, Container{
				ID:        c.Id,
				Namespace: namespace,
				Pod:       podName,
				Container: containerName,
				PIDs:      pids,
			})

			slog.Debug("Discovered container", "namespace", namespace, "pod", podName, "container", containerName, "pids", len(pids))
		}
	}

	slog.Info("Container discovery complete", "containers", len(containers))
	return containers, nil
}

func (k *KubernetesClient) getProcessesForContainer(ctx context.Context, containerID, namespace, pod, container string) ([]ProcessInfo, error) {
	// Get init PID from containerd
	initPID, err := k.getInitPID(containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get init PID: %w", err)
	}

	// Get PID namespace
	pidNS, err := k.getPIDNamespace(initPID)
	if err != nil {
		return nil, fmt.Errorf("failed to get PID namespace: %w", err)
	}

	// Find all PIDs in this namespace that match process filters
	pids := k.findMatchingPIDs(pidNS, namespace, pod, container)

	slog.Debug("Found matching processes", "namespace", namespace, "pod", pod, "container", container, "count", len(pids))
	return pids, nil
}

func (k *KubernetesClient) getInitPID(containerID string) (string, error) {
	ctx := namespaces.WithNamespace(context.Background(), "k8s.io")
	container, err := k.containerdClient.LoadContainer(ctx, containerID)
	if err != nil {
		return "", err
	}
	task, err := container.Task(ctx, nil)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", task.Pid()), nil
}

func (k *KubernetesClient) getPIDNamespace(pid string) (string, error) {
	nsPath := filepath.Join(k.config.Paths.Proc, pid, "ns", "pid")
	link, err := os.Readlink(nsPath)
	if err != nil {
		return "", err
	}
	var nsid int
	n, err := fmt.Sscanf(link, "pid:[%d]", &nsid)
	if n == 1 && err == nil {
		return fmt.Sprintf("%d", nsid), nil
	}
	return "", fmt.Errorf("unexpected ns link format: %s", link)
}

func (k *KubernetesClient) findMatchingPIDs(pidNamespace, namespace, pod, container string) []ProcessInfo {
	var processes []ProcessInfo

	entries, err := os.ReadDir(k.config.Paths.Proc)
	if err != nil {
		return processes
	}

	for _, entry := range entries {
		pid := entry.Name()
		pidInt, err := strconv.Atoi(pid)
		if err != nil {
			continue
		}
		if !entry.IsDir() {
			continue
		}

		// Check if PID is in the target namespace
		if !k.pidInNamespace(pid, pidNamespace) {
			continue
		}

		// Get command name
		comm, err := k.getComm(pid)
		if err != nil {
			continue
		}

		// Apply process filter
		if !k.config.MatchesProcess(namespace, pod, container, comm) {
			continue
		}

		processes = append(processes, ProcessInfo{
			PID:  pidInt,
			Comm: comm,
		})
	}

	return processes
}

func (k *KubernetesClient) pidInNamespace(pid, targetNS string) bool {
	nsPath := filepath.Join(k.config.Paths.Proc, pid, "ns", "pid")
	link, err := os.Readlink(nsPath)
	if err != nil {
		return false
	}
	var pidnsInt int
	n, err := fmt.Sscanf(link, "pid:[%d]", &pidnsInt)
	if n == 1 && err == nil {
		return fmt.Sprintf("%d", pidnsInt) == targetNS
	}
	return false
}

func (k *KubernetesClient) getComm(pid string) (string, error) {
	commPath := filepath.Join(k.config.Paths.Proc, pid, "comm")
	data, err := os.ReadFile(commPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
