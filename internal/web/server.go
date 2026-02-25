// pattern: Imperative Shell

package web

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"devagent/internal/container"
	"devagent/internal/discovery"
	"devagent/internal/logging"
	"devagent/internal/worktree"
)

// worktreeOps abstracts worktree package functions for testability.
type worktreeOps interface {
	ValidateName(name string) error
	Create(projectPath, name string) (string, error)
	Destroy(projectPath, name string) error
	WorktreeDir(projectPath, name string) string
}

// realWorktreeOps delegates to the worktree package functions.
type realWorktreeOps struct{}

func (realWorktreeOps) ValidateName(name string) error {
	return worktree.ValidateName(name)
}

func (realWorktreeOps) Create(projectPath, name string) (string, error) {
	return worktree.Create(projectPath, name)
}

func (realWorktreeOps) Destroy(projectPath, name string) error {
	return worktree.Destroy(projectPath, name)
}

func (realWorktreeOps) WorktreeDir(projectPath, name string) string {
	return worktree.WorktreeDir(projectPath, name)
}

// Server is the web server that serves the API and SPA.
type Server struct {
	httpServer  *http.Server
	manager     *container.Manager
	notifyTUI   func(any)
	logger      *logging.ScopedLogger
	addr        string
	listener    net.Listener
	events      *eventBroker
	scanner     func(context.Context) []discovery.DiscoveredProject
	worktreeOps worktreeOps
}

// Config holds web server configuration.
type Config struct {
	Bind string
	Port int
}

// New creates a web server.
// notifyTUI is called after mutations to keep the TUI in sync via p.Send().
// logProvider must implement logging.LoggerProvider (both *logging.Manager and
// *logging.TestLogManager satisfy this interface).
// scanner is an optional function for project discovery; if nil, the /api/projects endpoint
// will return only unmatched containers.
func New(cfg Config, manager *container.Manager, notifyTUI func(any), logProvider logging.LoggerProvider, scanner func(context.Context) []discovery.DiscoveredProject) *Server {
	logger := logProvider.For("web")
	addr := fmt.Sprintf("%s:%d", cfg.Bind, cfg.Port)

	mux := http.NewServeMux()

	events := newEventBroker()
	if manager != nil {
		manager.SetOnChange(events.Notify)
	}

	s := &Server{
		httpServer: &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		},
		manager:     manager,
		notifyTUI:   notifyTUI,
		logger:      logger,
		addr:        addr,
		events:      events,
		scanner:     scanner,
		worktreeOps: realWorktreeOps{},
	}

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("GET /api/projects", s.handleGetProjects)
	mux.HandleFunc("GET /api/containers", s.handleListContainers)
	mux.HandleFunc("GET /api/containers/{id}", s.handleGetContainer)
	mux.HandleFunc("GET /api/containers/{id}/sessions", s.handleListSessions)
	mux.HandleFunc("POST /api/containers/{id}/sessions", s.handleCreateSession)
	mux.HandleFunc("DELETE /api/containers/{id}/sessions/{name}", s.handleDestroySession)
	mux.HandleFunc("GET /api/containers/{id}/sessions/{name}/terminal", s.HandleTerminal)
	mux.HandleFunc("POST /api/containers/{id}/start", s.handleStartContainer)
	mux.HandleFunc("POST /api/containers/{id}/stop", s.handleStopContainer)
	mux.HandleFunc("DELETE /api/containers/{id}", s.handleDestroyContainer)
	mux.HandleFunc("POST /api/projects/{encodedPath}/worktrees", s.handleCreateWorktree)
	mux.HandleFunc("POST /api/projects/{encodedPath}/worktrees/{name}/start", s.handleStartWorktreeContainer)
	mux.HandleFunc("DELETE /api/projects/{encodedPath}/worktrees/{name}", s.handleDeleteWorktree)
	mux.HandleFunc("GET /api/host/sessions", s.handleListHostSessions)
	mux.HandleFunc("POST /api/host/sessions", s.handleCreateHostSession)
	mux.HandleFunc("DELETE /api/host/sessions/{name}", s.handleDestroyHostSession)
	mux.HandleFunc("GET /api/host/sessions/{name}/terminal", s.HandleHostTerminal)
	mux.Handle("/", s.spaHandler())

	return s
}

// spaHandler serves the embedded frontend dist directory.
// Unknown paths fall back to index.html to support client-side routing.
func (s *Server) spaHandler() http.Handler {
	dist, err := fs.Sub(frontendDist, "frontend/dist")
	if err != nil {
		s.logger.Error("failed to create sub filesystem", "error", err)
		return http.NotFoundHandler()
	}
	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		_, err := fs.Stat(dist, path)
		if os.IsNotExist(err) {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}

// Listen binds the server to its configured address and returns the listener.
// Call Serve() after Listen() to start accepting connections.
// This two-step approach allows callers to obtain the actual bound address
// (useful for ephemeral port 0 in tests) before the server blocks on Serve().
func (s *Server) Listen() (net.Listener, error) {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return nil, fmt.Errorf("web server listen: %w", err)
	}
	s.listener = ln
	return ln, nil
}

// Serve accepts connections on the listener. Blocks until the server stops.
// Must call Listen() first.
func (s *Server) Serve(ln net.Listener) error {
	s.logger.Info("web server started", "addr", ln.Addr().String())
	return s.httpServer.Serve(ln)
}

// Start is a convenience that calls Listen() then Serve(). Blocks until the server stops.
func (s *Server) Start() error {
	ln, err := s.Listen()
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

// Addr returns the address the server is listening on.
// Only valid after Listen() or Start() has been called.
func (s *Server) Addr() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.addr
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("web server shutting down")
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// SetWorktreeOpsForTest replaces the worktreeOps implementation. Test-only.
func (s *Server) SetWorktreeOpsForTest(ops worktreeOps) {
	s.worktreeOps = ops
}
