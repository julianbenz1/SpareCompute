package runtime

import (
	"context"
	"os/exec"
	"strconv"
)

type Docker struct{}

func NewDocker() *Docker {
	return &Docker{}
}

func (d *Docker) StartContainer(ctx context.Context, image string, name string, env map[string]string, internalPort int) error {
	args := []string{"run", "-d", "--name", name}
	for k, v := range env {
		args = append(args, "-e", k+"="+v)
	}
	if internalPort > 0 {
		args = append(args, "--expose", strconv.Itoa(internalPort))
	}
	args = append(args, image)
	return exec.CommandContext(ctx, "docker", args...).Run()
}

func (d *Docker) StopContainer(ctx context.Context, containerName string) error {
	return exec.CommandContext(ctx, "docker", "rm", "-f", containerName).Run()
}
