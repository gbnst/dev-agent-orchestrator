package container

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

// CommandExecutor is a function that executes a command and returns its output.
type CommandExecutor func(ctx context.Context, name string, args ...string) (string, error)

// Runtime wraps Docker or Podman CLI operations.
type Runtime struct {
	executable string
	exec       CommandExecutor
}

// NewRuntime creates a new Runtime with the specified executable (docker or podman).
func NewRuntime(executable string) *Runtime {
	return &Runtime{
		executable: executable,
		exec:       defaultExecutor,
	}
}

// NewRuntimeWithExecutor creates a new Runtime with a custom executor for testing.
func NewRuntimeWithExecutor(executable string, exec CommandExecutor) *Runtime {
	return &Runtime{
		executable: executable,
		exec:       exec,
	}
}

// defaultExecutor runs commands using os/exec.
func defaultExecutor(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return stdout.String(), nil
}

// ListContainers returns all devagent-managed containers.
func (r *Runtime) ListContainers(ctx context.Context) ([]Container, error) {
	output, err := r.exec(ctx, r.executable, "ps", "-a", "--filter", "label=devagent.managed=true", "--format", "json")
	if err != nil {
		return nil, err
	}

	return r.parseContainerList(output)
}

// StartContainer starts a container by ID.
func (r *Runtime) StartContainer(ctx context.Context, id string) error {
	_, err := r.exec(ctx, r.executable, "start", id)
	return err
}

// StopContainer stops a container by ID.
func (r *Runtime) StopContainer(ctx context.Context, id string) error {
	_, err := r.exec(ctx, r.executable, "stop", id)
	return err
}

// RemoveContainer removes a container by ID.
func (r *Runtime) RemoveContainer(ctx context.Context, id string) error {
	_, err := r.exec(ctx, r.executable, "rm", id)
	return err
}

// Exec runs a command inside a container.
func (r *Runtime) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	args := append([]string{"exec", id}, cmd...)
	return r.exec(ctx, r.executable, args...)
}

// containerJSON represents the JSON output from docker/podman ps --format json.
type containerJSON struct {
	ID        string `json:"ID"`
	Names     string `json:"Names"`
	State     string `json:"State"`
	Labels    string `json:"Labels"`
	CreatedAt string `json:"CreatedAt"`
}

// parseContainerList parses JSON lines output from docker ps.
func (r *Runtime) parseContainerList(output string) ([]Container, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return []Container{}, nil
	}

	var containers []Container
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var cj containerJSON
		if err := json.Unmarshal([]byte(line), &cj); err != nil {
			continue // Skip malformed lines
		}

		labels := parseLabels(cj.Labels)
		createdAt, _ := time.Parse(time.RFC3339, cj.CreatedAt)

		containers = append(containers, Container{
			ID:          cj.ID,
			Name:        cj.Names,
			State:       mapState(cj.State),
			ProjectPath: labels[LabelProjectPath],
			Template:    labels[LabelTemplate],
			Agent:       labels[LabelAgent],
			CreatedAt:   createdAt,
			Labels:      labels,
		})
	}

	return containers, nil
}

// mapState converts Docker/Podman state strings to ContainerState.
func mapState(state string) ContainerState {
	switch strings.ToLower(state) {
	case "running":
		return StateRunning
	case "created":
		return StateCreated
	case "exited", "paused", "dead", "removing":
		return StateStopped
	default:
		return StateStopped
	}
}

// parseLabels parses a comma-separated key=value label string.
func parseLabels(labelStr string) map[string]string {
	labels := make(map[string]string)
	if labelStr == "" {
		return labels
	}

	pairs := strings.Split(labelStr, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			labels[kv[0]] = kv[1]
		}
	}

	return labels
}
