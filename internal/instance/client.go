// pattern: Imperative Shell
package instance

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Client is a thin HTTP client for communicating with a running devagent instance.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a Client targeting the given base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// List fetches the project list from the running instance.
// Returns raw JSON bytes from GET /api/projects (projects with worktrees
// and matched containers, plus unmatched containers).
func (c *Client) List() ([]byte, error) {
	return c.get("/api/projects")
}

// get performs a GET request and returns the response body.
func (c *Client) get(path string) ([]byte, error) {
	resp, err := c.httpClient.Get(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to devagent: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		msg := extractErrorMessage(body)
		return nil, fmt.Errorf("devagent returned status %d: %s", resp.StatusCode, msg)
	}

	return body, nil
}

// post performs a POST request with no body and returns the response body.
func (c *Client) post(path string) ([]byte, error) {
	req, err := http.NewRequest("POST", c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to devagent: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := extractErrorMessage(body)
		return nil, fmt.Errorf("devagent returned status %d: %s", resp.StatusCode, msg)
	}

	return body, nil
}

// delete performs a DELETE request and returns the response body.
func (c *Client) delete(path string) ([]byte, error) {
	req, err := http.NewRequest("DELETE", c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to devagent: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := extractErrorMessage(body)
		return nil, fmt.Errorf("devagent returned status %d: %s", resp.StatusCode, msg)
	}

	return body, nil
}

// postJSON performs a POST request with a JSON body and returns the response body.
func (c *Client) postJSON(path string, body any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to devagent: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := extractErrorMessage(respBody)
		return nil, fmt.Errorf("devagent returned status %d: %s", resp.StatusCode, msg)
	}

	return respBody, nil
}

// extractErrorMessage attempts to extract the error message from a JSON response body.
// If the body is not valid JSON or doesn't have an "error" field, returns the raw body string.
func extractErrorMessage(body []byte) string {
	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
		return errResp.Error
	}
	return string(body)
}

// StartContainer starts a stopped container.
func (c *Client) StartContainer(id string) ([]byte, error) {
	return c.post("/api/containers/" + id + "/start")
}

// StopContainer stops a running container.
func (c *Client) StopContainer(id string) ([]byte, error) {
	return c.post("/api/containers/" + id + "/stop")
}

// DestroyContainer destroys a container.
func (c *Client) DestroyContainer(id string) ([]byte, error) {
	return c.delete("/api/containers/" + id)
}

// CreateSession creates a tmux session in the named container.
func (c *Client) CreateSession(containerID, sessionName string) ([]byte, error) {
	return c.postJSON("/api/containers/"+containerID+"/sessions", map[string]string{"name": sessionName})
}

// DestroySession destroys a tmux session in the named container.
func (c *Client) DestroySession(containerID, sessionName string) ([]byte, error) {
	return c.delete("/api/containers/" + containerID + "/sessions/" + sessionName)
}

// CreateWorktree creates a git worktree within a project.
// If noStart is true, creates the worktree without starting a container.
func (c *Client) CreateWorktree(projectPath, name string, noStart bool) ([]byte, error) {
	encoded := base64.URLEncoding.EncodeToString([]byte(projectPath))
	body := map[string]any{"name": name}
	if noStart {
		body["no_start"] = true
	}
	return c.postJSON("/api/projects/"+encoded+"/worktrees", body)
}

// ReadSession captures pane content from a tmux session.
// If lines > 0, captures last N lines; otherwise captures visible pane.
func (c *Client) ReadSession(containerID, session string, lines int) ([]byte, error) {
	path := "/api/containers/" + containerID + "/sessions/" + session + "/capture"
	if lines > 0 {
		path += "?lines=" + strconv.Itoa(lines)
	}
	return c.get(path)
}

// ReadLines captures the last N lines from a session's scrollback history.
func (c *Client) ReadLines(containerID, session string, lines int) ([]byte, error) {
	path := "/api/containers/" + containerID + "/sessions/" + session + "/capture-lines"
	if lines > 0 {
		path += "?lines=" + strconv.Itoa(lines)
	}
	return c.get(path)
}

// ReadSessionFromCursor captures pane content from a cursor position forward.
// Used by the tail polling loop.
func (c *Client) ReadSessionFromCursor(containerID, session string, fromCursor int) ([]byte, error) {
	path := "/api/containers/" + containerID + "/sessions/" + session + "/capture"
	path += "?from_cursor=" + strconv.Itoa(fromCursor)
	return c.get(path)
}

// SendToSession sends keystrokes to a tmux session.
func (c *Client) SendToSession(containerID, session, text string) error {
	_, err := c.postJSON("/api/containers/"+containerID+"/sessions/"+session+"/send", map[string]string{"text": text})
	return err
}

// NewClientWithTimeout creates a Client with a custom timeout.
// Used for long-running operations like worktree creation with devcontainer builds.
func NewClientWithTimeout(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: timeout},
	}
}
