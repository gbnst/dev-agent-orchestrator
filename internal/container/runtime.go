// pattern: Imperative Shell

package container

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
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

// Exec runs a command inside a container as root.
func (r *Runtime) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	args := append([]string{"exec", id}, cmd...)
	return r.exec(ctx, r.executable, args...)
}

// ExecAs runs a command inside a container as the specified user.
func (r *Runtime) ExecAs(ctx context.Context, id string, user string, cmd []string) (string, error) {
	args := []string{"exec", "-u", user, id}
	args = append(args, cmd...)
	return r.exec(ctx, r.executable, args...)
}

// containerJSON represents the JSON output from docker/podman ps --format json.
// Supports both Docker (JSON lines) and Podman (JSON array) formats.
type containerJSON struct {
	// Docker uses "ID", Podman uses "Id"
	ID string `json:"ID"`
	Id string `json:"Id"`
	// Docker uses string, Podman uses array
	Names     interface{} `json:"Names"`
	State     string      `json:"State"`
	Labels    interface{} `json:"Labels"` // Docker: string, Podman: map
	CreatedAt string      `json:"CreatedAt"`
	Created   int64       `json:"Created"` // Podman uses unix timestamp
}

func (cj *containerJSON) getID() string {
	if cj.ID != "" {
		return cj.ID
	}
	return cj.Id
}

func (cj *containerJSON) getName() string {
	switch v := cj.Names.(type) {
	case string:
		return v
	case []interface{}:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

func (cj *containerJSON) getLabels() map[string]string {
	labels := make(map[string]string)
	switch v := cj.Labels.(type) {
	case string:
		// Docker format: comma-separated key=value
		return parseLabels(v)
	case map[string]interface{}:
		// Podman format: actual map
		for k, val := range v {
			if s, ok := val.(string); ok {
				labels[k] = s
			}
		}
	}
	return labels
}

func (cj *containerJSON) getCreatedAt() time.Time {
	if cj.Created > 0 {
		return time.Unix(cj.Created, 0)
	}
	t, _ := time.Parse(time.RFC3339, cj.CreatedAt)
	return t
}

// parseContainerList parses JSON output from docker/podman ps.
func (r *Runtime) parseContainerList(output string) ([]Container, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return []Container{}, nil
	}

	var containers []Container

	// Try parsing as JSON array first (Podman format)
	if strings.HasPrefix(output, "[") {
		var cjs []containerJSON
		if err := json.Unmarshal([]byte(output), &cjs); err == nil {
			for _, cj := range cjs {
				labels := cj.getLabels()
				containers = append(containers, Container{
					ID:          cj.getID(),
					Name:        cj.getName(),
					State:       mapState(cj.State),
					ProjectPath: labels[LabelProjectPath],
					Template:    labels[LabelTemplate],
					Agent:       labels[LabelAgent],
					RemoteUser:  getRemoteUser(labels),
					CreatedAt:   cj.getCreatedAt(),
					Labels:      labels,
				})
			}
			return containers, nil
		}
	}

	// Fall back to JSON lines (Docker format)
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

		labels := cj.getLabels()
		containers = append(containers, Container{
			ID:          cj.getID(),
			Name:        cj.getName(),
			State:       mapState(cj.State),
			ProjectPath: labels[LabelProjectPath],
			Template:    labels[LabelTemplate],
			Agent:       labels[LabelAgent],
			RemoteUser:  getRemoteUser(labels),
			CreatedAt:   cj.getCreatedAt(),
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

// getRemoteUser returns the remote user from labels, defaulting to DefaultRemoteUser.
func getRemoteUser(labels map[string]string) string {
	if user, ok := labels[LabelRemoteUser]; ok && user != "" {
		return user
	}
	return DefaultRemoteUser
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
