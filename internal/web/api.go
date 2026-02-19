// pattern: Imperative Shell

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"devagent/internal/container"
	"devagent/internal/tui"
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
	c, ok := s.manager.Get(id)
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
	_, ok := s.manager.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}

	sessions, err := s.manager.ListSessions(r.Context(), id)
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

	c, ok := s.manager.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}

	if !c.IsRunning() {
		writeError(w, http.StatusBadRequest, "container is not running")
		return
	}

	sessions, err := s.manager.ListSessions(r.Context(), id)
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

	if err := s.manager.CreateSession(r.Context(), id, req.Name); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	if s.notifyTUI != nil {
		s.notifyTUI(tui.WebSessionActionMsg{ContainerID: id})
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": req.Name})
}

// handleDestroySession handles DELETE /api/containers/{id}/sessions/{name}.
// Destroys the named tmux session. Returns 200 on success.
// Returns 400 if container is not running, 404 if container not found, 500 on internal error.
func (s *Server) handleDestroySession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")

	c, ok := s.manager.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}

	if !c.IsRunning() {
		writeError(w, http.StatusBadRequest, "container is not running")
		return
	}

	if err := s.manager.KillSession(r.Context(), id, name); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to destroy session")
		return
	}

	if s.notifyTUI != nil {
		s.notifyTUI(tui.WebSessionActionMsg{ContainerID: id})
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "destroyed"})
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
