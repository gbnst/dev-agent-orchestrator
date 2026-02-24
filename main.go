// pattern: Imperative Shell
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/config"
	"devagent/internal/container"
	"devagent/internal/discovery"
	"devagent/internal/logging"
	"devagent/internal/process"
	"devagent/internal/tsnsrv"
	"devagent/internal/tui"
	"devagent/internal/web"
)

var version = "dev"

func main() {
	configDir := flag.String("config-dir", "", "config directory (default: ~/.config/devagent)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: devagent [options] [command]\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  list      Output JSON data about all managed containers\n")
		fmt.Fprintf(os.Stderr, "  version   Print version and exit\n")
		fmt.Fprintf(os.Stderr, "  (none)    Launch interactive TUI\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "list":
			runListCommand(*configDir)
			return
		case "version":
			fmt.Println(version)
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
	ProjectPath  string        `json:"project_path"`
	Template     string        `json:"template"`
	Devcontainer ContainerInfo `json:"devcontainer"`
	ProxySidecar *SidecarInfo  `json:"proxy_sidecar,omitempty"`
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
	// Sidecars have LabelSidecarType label, devcontainers don't
	devcontainers := make([]container.Container, 0)
	sidecars := make(map[string]container.Container) // keyed by compose project name

	for _, c := range containers {
		if _, ok := c.Labels[container.LabelSidecarType]; ok {
			composeProject := c.Labels[container.LabelComposeProject]
			if composeProject != "" {
				sidecars[composeProject] = c
			}
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

		// Find matching sidecar by compose project label
		composeProject := c.Labels[container.LabelComposeProject]
		if composeProject != "" {
			if sidecar, ok := sidecars[composeProject]; ok {
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

	// Start project discovery if scan paths configured
	if len(cfg.ScanPaths) > 0 {
		scanner := discovery.NewScanner()
		resolvedPaths := cfg.ResolveScanPaths()
		projects := scanner.ScanAll(resolvedPaths)
		appLogger.Info("discovered projects", "count", len(projects), "scan_paths", resolvedPaths)
		model.SetDiscoveredProjects(projects)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())

	if cfg.Web.Port > 0 {
		webServer := web.New(
			web.Config{Bind: cfg.Web.Bind, Port: cfg.Web.Port},
			model.Manager(),
			p.Send,
			logManager,
		)
		ln, err := webServer.Listen()
		if err != nil {
			appLogger.Error("web server listen error", "error", err)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		webURL := fmt.Sprintf("http://%s", webServer.Addr())
		go func() {
			p.Send(tui.WebListenURLMsg{URL: webURL})
		}()

		go func() {
			if err := webServer.Serve(ln); err != nil && err != http.ErrServerClosed {
				appLogger.Error("web server error", "error", err)
			}
		}()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := webServer.Shutdown(ctx); err != nil {
				appLogger.Error("web server shutdown error", "error", err)
			}
		}()

		if cfg.Tailscale.Enabled {
			supervisor, err := startTsnsrv(&cfg, webServer.Addr(), logManager)
			if err != nil {
				appLogger.Warn("tsnsrv failed to start (continuing without tailscale)", "error", err)
			} else {
				defer supervisor.Stop()

				// Poll for tailscale FQDN in background
				stateDir := cfg.ResolveTokenPath(cfg.Tailscale.StateDir)
				tc := cfg.Tailscale
				go func() {
					for i := 0; i < 30; i++ {
						url, ok := tsnsrv.ReadServiceURL(stateDir, tc)
						if ok {
							appLogger.Info("tailscale URL resolved", "url", url)
							p.Send(tui.TailscaleURLMsg{URL: url})
							return
						}
						time.Sleep(1 * time.Second)
					}
					// Timed out, send fallback
					fallback, _ := tsnsrv.ReadServiceURL(stateDir, tc)
					appLogger.Warn("tailscale URL resolution timed out, using fallback", "url", fallback)
					p.Send(tui.TailscaleURLMsg{URL: fallback})
				}()
			}
		}
	}

	if _, err := p.Run(); err != nil {
		appLogger.Error("application exited with error", "error", err)
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}

	appLogger.Info("application stopped")
}

// startTsnsrv validates config, builds the process config, and starts the tsnsrv supervisor.
func startTsnsrv(cfg *config.Config, upstreamAddr string, logProvider logging.LoggerProvider) (*process.Supervisor, error) {
	logger := logProvider.For("tsnsrv")

	if err := cfg.Tailscale.Validate(cfg.ResolveTokenPath); err != nil {
		return nil, fmt.Errorf("tailscale config validation: %w", err)
	}

	pc, err := tsnsrv.BuildProcessConfig(cfg.Tailscale, upstreamAddr, cfg.ResolveTokenPath)
	if err != nil {
		return nil, fmt.Errorf("tsnsrv config: %w", err)
	}

	supervisor := process.NewSupervisor(pc, logger)
	if err := supervisor.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("tsnsrv start: %w", err)
	}

	logger.Info("tsnsrv started", "upstream", upstreamAddr, "name", cfg.Tailscale.Name)
	return supervisor, nil
}
