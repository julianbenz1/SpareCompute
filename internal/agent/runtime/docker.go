package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/julianbenz1/SpareCompute/internal/common"
)

var safeMigrationID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type Docker struct {
	bindHost string
}

func NewDocker(bindHost string) *Docker {
	if strings.TrimSpace(bindHost) == "" {
		bindHost = "127.0.0.1"
	}
	return &Docker{bindHost: bindHost}
}

func (d *Docker) StartContainer(ctx context.Context, image string, name string, env map[string]string, internalPort int) (common.RuntimeStartResponse, error) {
	args := []string{"run", "-d", "--name", name}
	for k, v := range env {
		args = append(args, "-e", k+"="+v)
	}
	if internalPort > 0 {
		args = append(args, "-p", d.bindHost+"::"+strconv.Itoa(internalPort))
	}
	args = append(args, image)
	out, err := runOutput(ctx, "docker", args...)
	if err != nil {
		return common.RuntimeStartResponse{}, err
	}
	containerID := strings.TrimSpace(out)
	hostPort, err := d.inspectHostPort(ctx, name, internalPort)
	if err != nil {
		return common.RuntimeStartResponse{}, err
	}
	return common.RuntimeStartResponse{
		ContainerID:  containerID,
		ContainerRef: name,
		HostPort:     hostPort,
	}, nil
}

func (d *Docker) StopContainer(ctx context.Context, containerName string) error {
	_, err := runOutput(ctx, "docker", "rm", "-f", containerName)
	return err
}

func (d *Docker) CheckpointContainer(ctx context.Context, containerName, migrationID, sharedDir string) error {
	if !safeMigrationID.MatchString(migrationID) {
		return errors.New("invalid migration id")
	}
	checkpointDir := filepath.Join(sharedDir, migrationID)
	if err := os.MkdirAll(checkpointDir, 0o755); err != nil {
		return err
	}
	_, err := runOutput(ctx, "docker", "checkpoint", "create", "--checkpoint-dir", checkpointDir, containerName, "live")
	return err
}

func (d *Docker) RestoreContainer(ctx context.Context, image, containerName string, env map[string]string, internalPort int, migrationID, sharedDir string) (common.RuntimeStartResponse, error) {
	if !safeMigrationID.MatchString(migrationID) {
		return common.RuntimeStartResponse{}, errors.New("invalid migration id")
	}
	createArgs := []string{"create", "--name", containerName}
	for k, v := range env {
		createArgs = append(createArgs, "-e", k+"="+v)
	}
	if internalPort > 0 {
		createArgs = append(createArgs, "-p", d.bindHost+"::"+strconv.Itoa(internalPort))
	}
	createArgs = append(createArgs, image)
	if _, err := runOutput(ctx, "docker", createArgs...); err != nil {
		return common.RuntimeStartResponse{}, err
	}

	checkpointDir := filepath.Join(sharedDir, migrationID)
	if _, err := runOutput(ctx, "docker", "start", "--checkpoint", "live", "--checkpoint-dir", checkpointDir, containerName); err != nil {
		return common.RuntimeStartResponse{}, err
	}

	containerID, err := runOutput(ctx, "docker", "inspect", "--format", "{{.Id}}", containerName)
	if err != nil {
		return common.RuntimeStartResponse{}, err
	}
	hostPort, err := d.inspectHostPort(ctx, containerName, internalPort)
	if err != nil {
		return common.RuntimeStartResponse{}, err
	}

	return common.RuntimeStartResponse{
		ContainerID:  strings.TrimSpace(containerID),
		ContainerRef: containerName,
		HostPort:     hostPort,
	}, nil
}

func (d *Docker) inspectHostPort(ctx context.Context, containerName string, internalPort int) (int, error) {
	key := fmt.Sprintf("%d/tcp", internalPort)
	template := fmt.Sprintf("{{(index (index .NetworkSettings.Ports %q) 0).HostPort}}", key)
	out, err := runOutput(ctx, "docker", "inspect", "--format", template, containerName)
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, err
	}
	return port, nil
}

func runOutput(ctx context.Context, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s failed: %w: %s", command, err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
