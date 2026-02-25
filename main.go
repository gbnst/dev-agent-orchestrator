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
	"devagent/internal/events"
	"devagent/internal/instance"
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
		fmt.Fprintf(os.Stderr, "  cleanup   Remove stale lock/port files from a crashed instance\n")
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
		case "cleanup":
			runCleanupCommand(*configDir)
			return
		case "version":
			fmt.Println(version)
			return
		}
	}

	runTUI(*configDir)
}

// resolveDataDir returns the data directory for lock/port files.
// If configDir is specified, uses that; otherwise uses ~/.config/devagent.
func resolveDataDir(configDir string) string {
	if configDir != "" {
		return configDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "devagent")
	}
	return filepath.Join(home, ".config", "devagent")
}

// runListCommand outputs JSON data about all managed containers.
// If a TUI instance is running, delegates via HTTP for a consistent view.
// Otherwise, falls back to querying the container runtime directly.
func runListCommand(configDir string) {
	dataDir := resolveDataDir(configDir)
	baseURL, err := instance.Discover(dataDir)
	if err == nil {
		// Running instance found — delegate via HTTP
		client := instance.NewClient(baseURL)
		data, err := client.List()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		os.Stdout.Write(data)
		return
	}

	// No running instance — query container runtime directly
	runListStandalone(configDir)
}

// ProjectInfo represents a project with its devcontainer and optional sidecar.
type ProjectInfo struct {
	ProjectPath  string               `json:"project_path"`
	Template     string               `json:"template"`
	Devcontainer ProjectContainerInfo `json:"devcontainer"`
	ProxySidecar *ProjectSidecarInfo  `json:"proxy_sidecar,omitempty"`
}

// ProjectContainerInfo represents container data in JSON output.
type ProjectContainerInfo struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	State     string                `json:"state"`
	CreatedAt time.Time             `json:"created_at"`
	Mounts    []container.MountInfo `json:"mounts,omitempty"`
}

// ProjectSidecarInfo represents a sidecar container in JSON output.
type ProjectSidecarInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

// runListStandalone queries the container runtime directly without a running TUI instance.
func runListStandalone(configDir string) {
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
	manager := container.NewManager(container.ManagerOptions{
		Config:  &cfg,
		Runtime: rt,
	})

	if err := manager.Refresh(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	containers := manager.List()

	output := make([]ProjectInfo, 0, len(containers))
	for _, c := range containers {
		mounts, _ := rt.GetMounts(ctx, c.ID)

		project := ProjectInfo{
			ProjectPath: c.ProjectPath,
			Template:    c.Template,
			Devcontainer: ProjectContainerInfo{
				ID:        c.ID,
				Name:      c.Name,
				State:     string(c.State),
				CreatedAt: c.CreatedAt,
				Mounts:    mounts,
			},
		}

		sidecars := manager.GetSidecarsForProject(c.ProjectPath)
		for _, sidecar := range sidecars {
			if sidecar.Type == "proxy" {
				project.ProxySidecar = &ProjectSidecarInfo{
					ID:    sidecar.ID,
					Name:  sidecar.Name,
					State: string(sidecar.State),
				}
				break
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

// runCleanupCommand removes stale lock and port files from a crashed instance.
func runCleanupCommand(configDir string) {
	dataDir := resolveDataDir(configDir)

	// Try to acquire the lock to verify no instance is actually running
	fl, err := instance.Lock(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: a devagent instance appears to be running. Stop it first.\n")
		os.Exit(1)
	}
	// We got the lock — no instance is running. Clean up and release.
	instance.Cleanup(dataDir, fl)
	fmt.Println("Cleaned up stale lock and port files.")
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

	dataDir := resolveDataDir(configDir)

	// Acquire single-instance lock
	fl, err := instance.Lock(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer instance.Cleanup(dataDir, fl)

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

	// Start project discovery if scan paths configured and create scanner function for web server
	var scannerFn func(context.Context) []discovery.DiscoveredProject
	if len(cfg.ScanPaths) > 0 {
		scanner := discovery.NewScanner()
		resolvedPaths := cfg.ResolveScanPaths()
		projects := scanner.ScanAll(resolvedPaths)
		appLogger.Info("discovered projects", "count", len(projects), "scan_paths", resolvedPaths)
		model.SetDiscoveredProjects(projects)
		scannerFn = func(_ context.Context) []discovery.DiscoveredProject {
			return scanner.ScanAll(resolvedPaths)
		}
	}

	p := tea.NewProgram(model, tea.WithAltScreen())

	// Web server always starts (ephemeral port if not configured)
	webServer := web.New(
		web.Config{Bind: cfg.Web.Bind, Port: cfg.Web.Port},
		model.Manager(),
		func(msg any) { p.Send(msg) },
		logManager,
		scannerFn,
	)
	ln, err := webServer.Listen()
	if err != nil {
		appLogger.Error("web server listen error", "error", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Write port file for CLI discovery
	if err := instance.WritePort(dataDir, webServer.Addr()); err != nil {
		appLogger.Error("failed to write port file", "error", err)
	}

	webURL := fmt.Sprintf("http://%s", webServer.Addr())
	go func() {
		p.Send(events.WebListenURLMsg{URL: webURL})
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

	// Tailscale only when web port is explicitly configured
	if cfg.Web.Port > 0 && cfg.Tailscale.Enabled {
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
						p.Send(events.TailscaleURLMsg{URL: url})
						return
					}
					time.Sleep(1 * time.Second)
				}
				// Timed out, send fallback
				fallback, _ := tsnsrv.ReadServiceURL(stateDir, tc)
				appLogger.Warn("tailscale URL resolution timed out, using fallback", "url", fallback)
				p.Send(events.TailscaleURLMsg{URL: fallback})
			}()
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
