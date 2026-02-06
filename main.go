// pattern: Imperative Shell
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/config"
	"devagent/internal/container"
	"devagent/internal/logging"
	"devagent/internal/tui"
)

func main() {
	configDir := flag.String("config-dir", "", "config directory (default: ~/.config/devagent)")
	flag.Parse()

	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "list":
			runListCommand(*configDir)
			return
		}
	}

	runTUI(*configDir)
}

// ContainerInfo represents container data for JSON output.
type ContainerInfo struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	State     string                `json:"state"`
	CreatedAt time.Time             `json:"created_at"`
	Mounts    []container.MountInfo `json:"mounts,omitempty"`
}

// SidecarInfo represents a sidecar container in JSON output.
type SidecarInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

// ProjectInfo represents a project with its devcontainer and optional sidecar.
type ProjectInfo struct {
	ProjectPath  string       `json:"project_path"`
	Template     string       `json:"template"`
	Devcontainer ContainerInfo `json:"devcontainer"`
	ProxySidecar *SidecarInfo `json:"proxy_sidecar,omitempty"`
}

// runListCommand outputs JSON data about all managed containers grouped by project.
func runListCommand(configDir string) {
	ctx := context.Background()

	cfg, err := loadConfig(configDir)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.ValidateRuntime(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	rt := container.NewRuntime(cfg.DetectedRuntime())

	containers, err := rt.ListContainers(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Separate devcontainers from sidecars
	// Sidecars have LabelSidecarOf label, devcontainers don't
	devcontainers := make([]container.Container, 0)
	sidecars := make(map[string]container.Container) // keyed by project hash (LabelSidecarOf value)

	for _, c := range containers {
		if parentRef, ok := c.Labels[container.LabelSidecarOf]; ok {
			sidecars[parentRef] = c
		} else {
			devcontainers = append(devcontainers, c)
		}
	}

	// Build project-grouped output
	output := make([]ProjectInfo, 0, len(devcontainers))
	for _, c := range devcontainers {
		mounts, _ := rt.GetMounts(ctx, c.ID)

		project := ProjectInfo{
			ProjectPath: c.ProjectPath,
			Template:    c.Template,
			Devcontainer: ContainerInfo{
				ID:        c.ID,
				Name:      c.Name,
				State:     string(c.State),
				CreatedAt: c.CreatedAt,
				Mounts:    mounts,
			},
		}

		// Find matching sidecar by project hash
		// Container names follow pattern: devagent-{hash}-app
		// Extract hash from name if possible
		projectHash := extractProjectHash(c.Name)
		if projectHash != "" {
			if sidecar, ok := sidecars[projectHash]; ok {
				project.ProxySidecar = &SidecarInfo{
					ID:    sidecar.ID,
					Name:  sidecar.Name,
					State: string(sidecar.State),
				}
			}
		}

		output = append(output, project)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}

// extractProjectHash extracts the project hash from a container name.
// Container names follow the pattern: devagent-{hash}-app or similar.
func extractProjectHash(name string) string {
	const prefix = "devagent-"
	if len(name) < len(prefix)+container.HashTruncLen+1 {
		return ""
	}
	if name[:len(prefix)] != prefix {
		return ""
	}
	// Extract the hash portion (12 chars after prefix)
	rest := name[len(prefix):]
	if len(rest) < container.HashTruncLen {
		return ""
	}
	return rest[:container.HashTruncLen]
}

// loadConfig loads the configuration from the specified directory or default location.
func loadConfig(configDir string) (config.Config, error) {
	if configDir != "" {
		return config.LoadFromDir(configDir)
	}
	return config.Load()
}

// runTUI launches the interactive TUI.
func runTUI(configDir string) {
	cfg, err := loadConfig(configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
	}

	if err := cfg.ValidateRuntime(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get home directory: %v\n", err)
		os.Exit(1)
	}
	dataDir := filepath.Join(home, ".config", "devagent")
	logPath := filepath.Join(dataDir, "orchestrator.log")

	logManager, err := logging.NewManager(logging.Config{
		FilePath:       logPath,
		MaxSizeMB:      10,
		MaxBackups:     3,
		MaxAgeDays:     7,
		ChannelBufSize: 1000,
		Level:          cfg.LogLevel,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logging: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = logManager.Close() }()

	appLogger := logManager.For("app")
	appLogger.Info("application starting")

	model := tui.NewModel(&cfg, logManager)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		appLogger.Error("application exited with error", "error", err)
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}

	appLogger.Info("application stopped")
}
