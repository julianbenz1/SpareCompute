package runtime

import (
	"context"
	"os/exec"
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
		args = append(args, "-p", "127.0.0.1::"+itoa(internalPort))
	}
	args = append(args, image)
	return exec.CommandContext(ctx, "docker", args...).Run()
}

func (d *Docker) StopContainer(ctx context.Context, containerName string) error {
	return exec.CommandContext(ctx, "docker", "rm", "-f", containerName).Run()
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	buf := [20]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return sign + string(buf[i:])
}

