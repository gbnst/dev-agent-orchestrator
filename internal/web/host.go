// pattern: Imperative Shell

package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strings"

	"devagent/internal/events"
	"devagent/internal/tmux"
)

// validSessionName matches alphanumeric names with hyphens and underscores.
var validSessionName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// parseHostSessions parses tmux list-sessions output into SessionResponse slices.
// Uses the consolidated tmux.ParseListSessions parser with an empty containerID
// for host sessions (which have no container).
func parseHostSessions(output string) []SessionResponse {
	tmuxSessions := tmux.ParseListSessions("", output)
	sessions := make([]SessionResponse, len(tmuxSessions))
	for i, ts := range tmuxSessions {
		sessions[i] = SessionResponse{
			Name:     ts.Name,
			Windows:  ts.Windows,
			Attached: ts.Attached,
		}
	}
	return sessions
}

// listHostSessions runs tmux list-sessions on the host and returns parsed results.
// Returns an empty slice (not an error) when no tmux server is running.
func listHostSessions() ([]SessionResponse, error) {
	out, err := exec.Command("tmux", "list-sessions").CombinedOutput()
	if err != nil {
		// "no server running" or "no sessions" → return empty, not error
		if strings.Contains(string(out), "no server running") ||
			strings.Contains(string(out), "no sessions") {
			return []SessionResponse{}, nil
		}
		return nil, fmt.Errorf("tmux list-sessions: %s", strings.TrimSpace(string(out)))
	}
	return parseHostSessions(string(out)), nil
}

// handleListHostSessions handles GET /api/host/sessions.
func (s *Server) handleListHostSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := listHostSessions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list host sessions")
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

// handleCreateHostSession handles POST /api/host/sessions.
func (s *Server) handleCreateHostSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if !validSessionName.MatchString(req.Name) {
		writeError(w, http.StatusBadRequest, "name must contain only alphanumeric characters, hyphens, and underscores")
		return
	}

	// Check for duplicate
	sessions, err := listHostSessions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list host sessions")
		return
	}
	for _, sess := range sessions {
		if sess.Name == req.Name {
			writeError(w, http.StatusConflict, "session already exists")
			return
		}
	}

	out, err := exec.Command("tmux", "-u", "new-session", "-d", "-s", req.Name).CombinedOutput()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create session: %s", strings.TrimSpace(string(out))))
		return
	}

	s.events.Notify()
	if s.notifyTUI != nil {
		s.notifyTUI(events.WebSessionActionMsg{ContainerID: "__host__"})
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": req.Name})
}

// handleDestroyHostSession handles DELETE /api/host/sessions/{name}.
func (s *Server) handleDestroyHostSession(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !validSessionName.MatchString(name) {
		writeError(w, http.StatusBadRequest, "invalid session name")
		return
	}

	out, err := exec.Command("tmux", "kill-session", "-t", name).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "session not found") {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to destroy session: %s", strings.TrimSpace(string(out))))
		return
	}

	s.events.Notify()
	if s.notifyTUI != nil {
		s.notifyTUI(events.WebSessionActionMsg{ContainerID: "__host__"})
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "destroyed"})
}
