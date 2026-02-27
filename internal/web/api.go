// pattern: Imperative Shell

package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"devagent/internal/container"
	"devagent/internal/discovery"
	"devagent/internal/events"
	"devagent/internal/tmux"
	"devagent/internal/worktree"
)

// ContainerResponse is the JSON representation of a container.
type ContainerResponse struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	State       string            `json:"state"`
	Template    string            `json:"template"`
	ProjectPath string            `json:"project_path"`
	RemoteUser  string            `json:"remote_user"`
	CreatedAt   time.Time         `json:"created_at"`
	Sessions    []SessionResponse `json:"sessions"`
}

// SessionResponse is the JSON representation of a tmux session.
type SessionResponse struct {
	Name     string `json:"name"`
	Windows  int    `json:"windows"`
	Attached bool   `json:"attached"`
}

// ProjectResponse is the JSON representation of a discovered project.
type ProjectResponse struct {
	Name        string             `json:"name"`
	Path        string             `json:"path"`
	EncodedPath string             `json:"encoded_path"`
	HasMakefile bool               `json:"has_makefile"`
	Worktrees   []WorktreeResponse `json:"worktrees"`
}

// WorktreeResponse is the JSON representation of a git worktree within a project.
type WorktreeResponse struct {
	Name      string             `json:"name"`
	Path      string             `json:"path"`
	IsMain    bool               `json:"is_main"`
	Container *ContainerResponse `json:"container"`
}

// ProjectsListResponse wraps the projects list with unmatched containers.
// Unmatched containers are those not belonging to any discovered project.
type ProjectsListResponse struct {
	Projects  []ProjectResponse   `json:"projects"`
	Unmatched []ContainerResponse `json:"unmatched"`
}

// buildContainerResponse converts a container to a ContainerResponse, populating
// sessions if the container is running.
func (s *Server) buildContainerResponse(ctx context.Context, c *container.Container) ContainerResponse {
	resp := ContainerResponse{
		ID:          c.ID,
		Name:        c.Name,
		State:       string(c.State),
		Template:    c.Template,
		ProjectPath: c.ProjectPath,
		RemoteUser:  c.RemoteUser,
		CreatedAt:   c.CreatedAt,
		Sessions:    []SessionResponse{},
	}

	if c.IsRunning() {
		sessions, err := s.manager.ListSessions(ctx, c.ID)
		if err == nil {
			for _, sess := range sessions {
				resp.Sessions = append(resp.Sessions, SessionResponse{
					Name:     sess.Name,
					Windows:  sess.Windows,
					Attached: sess.Attached,
				})
			}
		}
	}

	return resp
}

// handleListContainers handles GET /api/containers.
// Returns JSON array of all managed containers. Populates sessions for running containers.
func (s *Server) handleListContainers(w http.ResponseWriter, r *http.Request) {
	containers := s.manager.List()
	result := make([]ContainerResponse, 0, len(containers))

	for _, c := range containers {
		result = append(result, s.buildContainerResponse(r.Context(), c))
	}

	writeJSON(w, http.StatusOK, result)
}

// handleGetContainer handles GET /api/containers/{id}.
// Returns single container JSON including sessions. Returns 404 for unknown IDs.
func (s *Server) handleGetContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, ok := s.manager.GetByNameOrID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}

	writeJSON(w, http.StatusOK, s.buildContainerResponse(r.Context(), c))
}

// handleListSessions handles GET /api/containers/{id}/sessions.
// Returns sessions for a container. Returns 404 for unknown container IDs.
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, ok := s.manager.GetByNameOrID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}

	sessions, err := s.manager.ListSessions(r.Context(), c.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}

	result := make([]SessionResponse, 0, len(sessions))
	for _, sess := range sessions {
		result = append(result, SessionResponse{
			Name:     sess.Name,
			Windows:  sess.Windows,
			Attached: sess.Attached,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateSessionRequest is the JSON body for creating a tmux session.
type CreateSessionRequest struct {
	Name string `json:"name"`
}

// CreateWorktreeRequest is the JSON body for creating a git worktree.
type CreateWorktreeRequest struct {
	Name    string `json:"name"`
	NoStart bool   `json:"no_start"`
}

// decodeProjectPath decodes a base64-URL-encoded project path from the URL.
func decodeProjectPath(encoded string) (string, error) {
	decoded, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("invalid path encoding: %w", err)
	}
	return string(decoded), nil
}

// handleCreateSession handles POST /api/containers/{id}/sessions.
// Creates a tmux session in the named container. Returns 201 on success.
// Returns 400 if container is not running or name is empty, 404 if container not found,
// 409 if session name already exists, 500 on internal error.
func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	c, ok := s.manager.GetByNameOrID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}

	if !c.IsRunning() {
		writeError(w, http.StatusBadRequest, "container is not running")
		return
	}

	sessions, err := s.manager.ListSessions(r.Context(), c.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}
	for _, sess := range sessions {
		if sess.Name == req.Name {
			writeError(w, http.StatusConflict, "session already exists")
			return
		}
	}

	if err := s.manager.CreateSession(r.Context(), c.ID, req.Name); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	if s.notifyTUI != nil {
		s.notifyTUI(events.WebSessionActionMsg{ContainerID: c.ID})
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": req.Name})
}

// handleDestroySession handles DELETE /api/containers/{id}/sessions/{name}.
// Destroys the named tmux session. Returns 200 on success.
// Returns 400 if container is not running, 404 if container not found, 500 on internal error.
func (s *Server) handleDestroySession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")

	c, ok := s.manager.GetByNameOrID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}

	if !c.IsRunning() {
		writeError(w, http.StatusBadRequest, "container is not running")
		return
	}

	if err := s.manager.KillSession(r.Context(), c.ID, name); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to destroy session")
		return
	}

	if s.notifyTUI != nil {
		s.notifyTUI(events.WebSessionActionMsg{ContainerID: c.ID})
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "destroyed"})
}

// handleStartContainer handles POST /api/containers/{id}/start.
// Starts a stopped container via docker-compose. Returns 400 if already running,
// 404 if container not found, 500 on internal error.
func (s *Server) handleStartContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	c, ok := s.manager.GetByNameOrID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}

	if c.IsRunning() {
		writeError(w, http.StatusBadRequest, "container is already running")
		return
	}

	if err := s.manager.StartWithCompose(r.Context(), c.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start container")
		return
	}

	if s.notifyTUI != nil {
		s.notifyTUI(events.WebSessionActionMsg{ContainerID: c.ID})
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

// handleStopContainer handles POST /api/containers/{id}/stop.
// Stops a running container via docker-compose. Returns 404 if container not found,
// 400 if container is already stopped, 500 on internal error.
func (s *Server) handleStopContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	c, ok := s.manager.GetByNameOrID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}

	if !c.IsRunning() {
		writeError(w, http.StatusBadRequest, "container is not running")
		return
	}

	if err := s.manager.StopWithCompose(r.Context(), c.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to stop container")
		return
	}

	if s.notifyTUI != nil {
		s.notifyTUI(events.WebSessionActionMsg{ContainerID: c.ID})
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// handleDestroyContainer handles DELETE /api/containers/{id}.
// Destroys a container via docker-compose down. Returns 404 if container not found,
// 500 on internal error.
func (s *Server) handleDestroyContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	c, ok := s.manager.GetByNameOrID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}

	if err := s.manager.DestroyWithCompose(r.Context(), c.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to destroy container")
		return
	}

	if s.notifyTUI != nil {
		s.notifyTUI(events.WebSessionActionMsg{ContainerID: c.ID})
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "destroyed"})
}

// handleCreateWorktree handles POST /api/projects/{encodedPath}/worktrees.
// Creates a git worktree and auto-starts a container for it.
// Returns 400 for invalid name, 409 for duplicate branch, 500 on internal error.
func (s *Server) handleCreateWorktree(w http.ResponseWriter, r *http.Request) {
	projectPath, err := decodeProjectPath(r.PathValue("encodedPath"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project path encoding")
		return
	}

	var req CreateWorktreeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	if err := s.worktreeOps.ValidateName(req.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	wtPath, err := s.worktreeOps.Create(projectPath, req.Name)
	if err != nil {
		// Check if the error indicates the worktree already exists.
		// The worktree package embeds git output in errors; "already exists"
		// is the reliable substring from git's error message.
		if strings.Contains(err.Error(), "already exists") {
			writeError(w, http.StatusConflict, "worktree already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create worktree: "+err.Error())
		return
	}

	if !req.NoStart {
		// Auto-start container for the new worktree
		containerID, err := s.manager.StartWorktreeContainer(r.Context(), wtPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "worktree created but failed to start container: "+err.Error())
			return
		}

		if s.notifyTUI != nil {
			s.notifyTUI(events.WebSessionActionMsg{ContainerID: containerID})
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"name":         req.Name,
			"path":         wtPath,
			"container_id": containerID,
		})
	} else {
		// Return response without container_id
		writeJSON(w, http.StatusCreated, map[string]any{"name": req.Name, "path": wtPath})
	}
}

// handleDeleteWorktree handles DELETE /api/projects/{encodedPath}/worktrees/{name}.
// Performs compound operation: stop container (if running) -> destroy container -> git worktree remove.
// Returns error if git refuses (dirty worktree, unmerged branch).
func (s *Server) handleDeleteWorktree(w http.ResponseWriter, r *http.Request) {
	projectPath, err := decodeProjectPath(r.PathValue("encodedPath"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project path encoding")
		return
	}

	name := r.PathValue("name")

	// Use shared function for compound destroy operation
	if err := worktree.DestroyWorktreeWithContainer(r.Context(), s.manager, projectPath, name, s.worktreeOps); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if s.notifyTUI != nil {
		s.notifyTUI(events.WebSessionActionMsg{ContainerID: ""})
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleStartWorktreeContainer starts a container for a worktree that has no container yet.
// POST /api/projects/{encodedPath}/worktrees/{name}/start
func (s *Server) handleStartWorktreeContainer(w http.ResponseWriter, r *http.Request) {
	projectPath, err := decodeProjectPath(r.PathValue("encodedPath"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project path encoding")
		return
	}

	name := r.PathValue("name")

	// Resolve worktree path. For linked worktrees this is
	// <projectPath>/.worktrees/<name>. For the main worktree the path
	// is the project root itself (there is no .worktrees/main directory).
	wtPath := s.worktreeOps.WorktreeDir(projectPath, name)
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		// Fall back to project root (main worktree)
		if _, err := os.Stat(projectPath); os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "worktree not found")
			return
		}
		wtPath = projectPath
	}

	// Check if a container already exists for this worktree
	for _, c := range s.manager.List() {
		if c.ProjectPath == wtPath {
			writeError(w, http.StatusConflict, "worktree already has a container")
			return
		}
	}

	// Start container via devcontainer up
	containerID, err := s.manager.StartWorktreeContainer(r.Context(), wtPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start worktree container: "+err.Error())
		return
	}

	// Notify TUI
	if s.notifyTUI != nil {
		s.notifyTUI(events.WebSessionActionMsg{ContainerID: containerID})
	}

	// Build container response
	c, ok := s.manager.GetByNameOrID(containerID)
	if !ok {
		// Container was created but not found after refresh — return minimal response
		writeJSON(w, http.StatusCreated, map[string]any{
			"id":   containerID,
			"name": name,
		})
		return
	}

	writeJSON(w, http.StatusCreated, s.buildContainerResponse(r.Context(), c))
}

// handleCapturePane handles GET /api/containers/{id}/sessions/{name}/capture.
// Captures pane content from a tmux session with optional query parameters.
// Returns 200 with JSON containing content, cursor_y, and lines_requested.
// Returns 400 if container is not running, 404 if container not found, 500 on error.
func (s *Server) handleCapturePane(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")

	c, ok := s.manager.GetByNameOrID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}

	if !c.IsRunning() {
		writeError(w, http.StatusBadRequest, "container is not running")
		return
	}

	// Parse query parameters
	var opts tmux.CaptureOpts
	opts.FromCursor = -1 // disabled by default

	if v := r.URL.Query().Get("lines"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "invalid lines parameter")
			return
		}
		opts.Lines = n
	}

	if v := r.URL.Query().Get("from_cursor"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "invalid from_cursor parameter")
			return
		}
		opts.FromCursor = n
	}

	// Use c.ID (the canonical container ID) rather than the raw path param,
	// because GetByNameOrID may have resolved a container name to its ID.
	content, err := s.manager.CaptureSession(r.Context(), c.ID, name, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to capture pane")
		return
	}

	cursorY, err := s.manager.CursorPosition(r.Context(), c.ID, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get cursor position")
		return
	}

	linesRequested := opts.Lines
	if linesRequested == 0 {
		if opts.FromCursor >= 0 {
			linesRequested = opts.FromCursor // report from_cursor as lines_requested
		} else {
			linesRequested = -1 // indicate "all visible" in response
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"content":         content,
		"cursor_y":        cursorY,
		"lines_requested": linesRequested,
	})
}

// handleCaptureLines handles GET /api/containers/{id}/sessions/{name}/capture-lines.
// Captures the last N lines from a tmux session's scrollback history.
// Returns 200 with JSON containing content. Default lines is 20.
// Returns 400 if container is not running or lines param is invalid, 404 if container not found, 500 on error.
func (s *Server) handleCaptureLines(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")

	c, ok := s.manager.GetByNameOrID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}

	if !c.IsRunning() {
		writeError(w, http.StatusBadRequest, "container is not running")
		return
	}

	lines := 20 // default
	if v := r.URL.Query().Get("lines"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "invalid lines parameter")
			return
		}
		lines = n
	}

	content, err := s.manager.CaptureSessionLines(r.Context(), c.ID, name, lines)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to capture lines")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"content": content,
	})
}

// SendKeysRequest is the JSON body for sending keys to a tmux session.
type SendKeysRequest struct {
	Text string `json:"text"`
}

// handleSendKeys handles POST /api/containers/{id}/sessions/{name}/send.
// Sends keystrokes to a tmux session. Returns 204 on success.
// Returns 400 if container is not running or text is empty, 404 if container not found,
// 500 on internal error.
func (s *Server) handleSendKeys(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")

	c, ok := s.manager.GetByNameOrID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}

	if !c.IsRunning() {
		writeError(w, http.StatusBadRequest, "container is not running")
		return
	}

	var req SendKeysRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Text == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}

	if err := s.manager.SendToSession(r.Context(), c.ID, name, req.Text); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to send keys")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// writeJSON writes v as JSON with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response with the given status code and message.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// handleGetProjects handles GET /api/projects.
// Returns ProjectsListResponse with projects (matched to worktrees) and unmatched containers.
func (s *Server) handleGetProjects(w http.ResponseWriter, r *http.Request) {
	var projects []discovery.DiscoveredProject
	if s.scanner != nil {
		projects = s.scanner(r.Context())
	}

	containers := s.manager.List()

	result := s.buildProjectResponses(r.Context(), projects, containers)
	writeJSON(w, http.StatusOK, result)
}

// buildProjectResponses assembles ProjectsListResponse by matching containers to worktrees by path.
// Containers not matched to any project worktree appear in the Unmatched list.
func (s *Server) buildProjectResponses(ctx context.Context, projects []discovery.DiscoveredProject, containers []*container.Container) ProjectsListResponse {
	// Index containers by ProjectPath for O(1) lookup
	containersByPath := make(map[string]*container.Container, len(containers))
	matched := make(map[string]bool, len(containers))
	for _, c := range containers {
		if c.ProjectPath != "" {
			containersByPath[c.ProjectPath] = c
		}
	}

	result := make([]ProjectResponse, 0, len(projects))
	for _, proj := range projects {
		pr := ProjectResponse{
			Name:        proj.Name,
			Path:        proj.Path,
			EncodedPath: base64.URLEncoding.EncodeToString([]byte(proj.Path)),
			HasMakefile: proj.HasMakefile,
			Worktrees:   make([]WorktreeResponse, 0, len(proj.Worktrees)+1),
		}

		// Main worktree (the project root itself)
		mainWR := WorktreeResponse{
			Name:   "main",
			Path:   proj.Path,
			IsMain: true,
		}
		if c, ok := containersByPath[proj.Path]; ok {
			resp := s.buildContainerResponse(ctx, c)
			mainWR.Container = &resp
			matched[c.ID] = true
		}
		pr.Worktrees = append(pr.Worktrees, mainWR)

		// Linked worktrees
		for _, wt := range proj.Worktrees {
			wr := WorktreeResponse{
				Name:   wt.Name,
				Path:   wt.Path,
				IsMain: false,
			}
			if c, ok := containersByPath[wt.Path]; ok {
				resp := s.buildContainerResponse(ctx, c)
				wr.Container = &resp
				matched[c.ID] = true
			}
			pr.Worktrees = append(pr.Worktrees, wr)
		}

		result = append(result, pr)
	}

	// Collect unmatched containers
	var unmatched []ContainerResponse
	for _, c := range containers {
		if !matched[c.ID] {
			unmatched = append(unmatched, s.buildContainerResponse(ctx, c))
		}
	}
	if unmatched == nil {
		unmatched = []ContainerResponse{}
	}

	return ProjectsListResponse{
		Projects:  result,
		Unmatched: unmatched,
	}
}
