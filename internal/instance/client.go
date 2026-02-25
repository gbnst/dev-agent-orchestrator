// pattern: Imperative Shell
package instance

import (
	"fmt"
	"io"
	"net/http"
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

// List fetches the container list from the running instance.
// Returns raw JSON bytes (preserving the existing output format).
func (c *Client) List() ([]byte, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/containers")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to devagent: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("devagent returned status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
