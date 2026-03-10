// pattern: Imperative Shell
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	flag "github.com/spf13/pflag"

	"devagent/internal/cli"
	"devagent/internal/config"
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
	// Stop parsing flags after the first non-flag arg (the subcommand),
	// so that --help after a subcommand is handled by the subcommand.
	flag.CommandLine.SetInterspersed(false)

	configDir := flag.StringP("config-dir", "c", "", "config directory (default: ~/.config/devagent)")
	agentHelp := flag.Bool("agent-help", false, "print agent orchestration guide")

	// Override flag.Usage before Parse so --help uses the CLI app's help
	flag.Usage = func() {
		app := cli.BuildApp(version, *configDir)
		app.PrintHelp(os.Stderr)
		flag.PrintDefaults()
	}

	flag.Parse()

	app := cli.BuildApp(version, *configDir)

	if *agentHelp {
		app.PrintAgentHelp(os.Stdout)
		return
	}

	if app.Execute(flag.Args()) {
		runTUI(*configDir)
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

	dataDir := cli.ResolveDataDir(configDir)

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
