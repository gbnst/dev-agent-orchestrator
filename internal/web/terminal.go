// pattern: Imperative Shell

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"os/exec"

	"github.com/coder/websocket"
	"github.com/creack/pty"

	"devagent/internal/container"
)

// ResizeMessage is sent from the browser when the terminal viewport changes.
type ResizeMessage struct {
	Type string `json:"type"` // "resize"
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// HandleTerminal upgrades to websocket and bridges PTY I/O for a tmux session.
func (s *Server) HandleTerminal(w http.ResponseWriter, r *http.Request) {
	containerID := r.PathValue("id")
	sessionName := r.PathValue("name")

	// Validate container exists and is running
	c, ok := s.manager.Get(containerID)
	if !ok {
		http.Error(w, "container not found", http.StatusNotFound)
		return
	}
	if c.State != container.StateRunning {
		http.Error(w, "container is not running", http.StatusBadRequest)
		return
	}

	// Verify session exists by listing sessions
	sessions, err := s.manager.ListSessions(r.Context(), containerID)
	if err != nil {
		http.Error(w, "failed to list sessions", http.StatusInternalServerError)
		return
	}
	sessionExists := false
	for _, sess := range sessions {
		if sess.Name == sessionName {
			sessionExists = true
			break
		}
	}
	if !sessionExists {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	// Upgrade to websocket — IMPORTANT: do NOT use r.Context() after this.
	// Restrict to localhost origins to prevent cross-origin WebSocket attacks.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"127.0.0.1:*", "localhost:*"},
	})
	if err != nil {
		s.logger.Error("websocket accept failed", "error", err)
		return
	}
	defer func() { _ = conn.CloseNow() }()
	conn.SetReadLimit(1 << 20) // 1 MB read limit

	// Build docker exec command matching Session.AttachCommand() flags
	remoteUser := c.RemoteUser
	if remoteUser == "" {
		remoteUser = container.DefaultRemoteUser
	}

	cmd := exec.Command(
		s.manager.RuntimePath(),
		"exec", "-it",
		"-u", remoteUser,
		"-e", "TERM=xterm-256color",
		"-e", "COLORTERM=truecolor",
		containerID,
		"tmux", "-u", "attach-session", "-t", sessionName,
	)

	// Start command with PTY
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		s.logger.Error("pty start failed", "error", err)
		_ = conn.Close(websocket.StatusInternalError, "terminal failed to start")
		return
	}
	defer func() { _ = ptmx.Close() }()
	defer func() { _ = cmd.Wait() }()

	s.logger.Info("terminal connected",
		"container", containerID,
		"session", sessionName,
	)

	ctx := context.Background()

	// PTY output → WebSocket (binary frames)
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				return
			}
			if err := conn.Write(ctx, websocket.MessageBinary, buf[:n]); err != nil {
				return
			}
		}
	}()

	// WebSocket → PTY input (binary = keystrokes, text = control messages)
	go func() {
		for {
			msgType, data, err := conn.Read(ctx)
			if err != nil {
				// Websocket closed — close PTY to stop the output goroutine
				_ = ptmx.Close()
				return
			}
			if msgType == websocket.MessageText {
				var msg ResizeMessage
				if json.Unmarshal(data, &msg) == nil && msg.Type == "resize" {
					_ = pty.Setsize(ptmx, &pty.Winsize{Rows: msg.Rows, Cols: msg.Cols})
					continue
				}
			}
			// Write raw input to PTY; errors are non-fatal (process may have exited)
			_, _ = ptmx.Write(data)
		}
	}()

	// Block until PTY output goroutine exits (process exited or PTY closed)
	<-done

	s.logger.Info("terminal disconnected",
		"container", containerID,
		"session", sessionName,
	)

	_ = conn.Close(websocket.StatusNormalClosure, "terminal closed")
}
