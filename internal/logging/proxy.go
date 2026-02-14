// pattern: Functional Core

package logging

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ProxyRequest represents a logged HTTP request/response from the mitmproxy sidecar.
// This struct matches the JSONL format written by filter.py.
type ProxyRequest struct {
	Timestamp  time.Time         // When the response completed
	Method     string            // HTTP method (GET, POST, etc.)
	URL        string            // Full request URL
	Status     int               // HTTP status code
	DurationMs int64             // Request duration in milliseconds
	ReqHeaders map[string]string // Request headers
	ResHeaders map[string]string // Response headers
}

// proxyRequestJSON matches the JSONL format from filter.py.
type proxyRequestJSON struct {
	Ts         float64           `json:"ts"`
	Method     string            `json:"method"`
	URL        string            `json:"url"`
	Status     int               `json:"status"`
	DurationMs int64             `json:"duration_ms"`
	ReqHeaders map[string]string `json:"req_headers"`
	ResHeaders map[string]string `json:"res_headers"`
}

// ParseProxyRequest parses a JSONL line into a ProxyRequest.
func ParseProxyRequest(line []byte) (*ProxyRequest, error) {
	var raw proxyRequestJSON
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse proxy request JSON: %w", err)
	}

	// Convert Unix timestamp to time.Time
	sec := int64(raw.Ts)
	nsec := int64((raw.Ts - float64(sec)) * 1e9)
	timestamp := time.Unix(sec, nsec)

	return &ProxyRequest{
		Timestamp:  timestamp,
		Method:     raw.Method,
		URL:        raw.URL,
		Status:     raw.Status,
		DurationMs: raw.DurationMs,
		ReqHeaders: raw.ReqHeaders,
		ResHeaders: raw.ResHeaders,
	}, nil
}

// ToLogEntry converts a ProxyRequest to a LogEntry for TUI consumption.
// The containerName is used to create the scope (e.g., "proxy.mycontainer").
func (p *ProxyRequest) ToLogEntry(containerName string) LogEntry {
	// Determine log level based on status code
	level := "INFO"
	if p.Status >= 400 && p.Status < 500 {
		level = "WARN"
	} else if p.Status >= 500 {
		level = "ERROR"
	}

	// Format message: "200 GET https://api.example.com/path 45ms"
	message := fmt.Sprintf("%d %s %s %dms", p.Status, p.Method, p.URL, p.DurationMs)

	// Store full ProxyRequest in Fields for details panel
	return LogEntry{
		Timestamp: p.Timestamp,
		Level:     level,
		Scope:     "proxy." + containerName,
		Message:   message,
		Fields: map[string]any{
			"_proxyRequest": p,
		},
	}
}

// ProxyLogReader tails a JSONL proxy log file and converts entries to LogEntry.
// It watches the file for changes using fsnotify with a polling safeguard
// for Docker bind mount compatibility.
type ProxyLogReader struct {
	filePath      string
	containerName string
	sink          *ChannelSink
	watcher       *fsnotify.Watcher

	mu     sync.Mutex
	file   *os.File
	offset int64
	closed bool
}

// NewProxyLogReader creates a new reader for the given proxy log file.
// The containerName is used for the log scope (e.g., "proxy.mycontainer").
// Entries are sent to the provided ChannelSink.
func NewProxyLogReader(filePath, containerName string, sink *ChannelSink) (*ProxyLogReader, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &ProxyLogReader{
		filePath:      filePath,
		containerName: containerName,
		sink:          sink,
		watcher:       watcher,
	}, nil
}

// Start begins watching the proxy log file for new entries.
// It returns when the context is cancelled.
func (r *ProxyLogReader) Start(ctx context.Context) error {
	// Watch parent directory (file may not exist yet)
	dir := filepath.Dir(r.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	if err := r.watcher.Add(dir); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	// Try to open the file if it exists (seek to end to skip existing content)
	r.mu.Lock()
	_ = r.openFile(true)
	r.mu.Unlock()

	// Polling safeguard for Docker bind mounts (5 second interval)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = r.Close()
			return ctx.Err()

		case event, ok := <-r.watcher.Events:
			if !ok {
				return nil
			}

			// Filter for our target file
			if filepath.Clean(event.Name) != filepath.Clean(r.filePath) {
				continue
			}

			// Handle file creation
			if event.Has(fsnotify.Create) {
				r.mu.Lock()
				_ = r.openFile(false) // Read from beginning for new files
				r.readNewLines()      // Read any content written with the create
				r.mu.Unlock()
			}

			// Handle file writes
			if event.Has(fsnotify.Write) {
				r.mu.Lock()
				r.readNewLines()
				r.mu.Unlock()
			}

			// Handle file removal/rename (log rotation)
			if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				r.mu.Lock()
				r.closeFile()
				r.mu.Unlock()
			}

		case <-ticker.C:
			// Polling safeguard: check for new content even if events missed
			r.mu.Lock()
			if r.file == nil {
				_ = r.openFile(false) // Read from beginning if file appeared
			}
			r.readNewLines()
			r.mu.Unlock()

		case err, ok := <-r.watcher.Errors:
			if !ok {
				return nil
			}
			// Log error but continue - don't fail on transient errors
			_ = err // Would log here if we had a logger
		}
	}
}

// openFile opens the log file.
// If seekToEnd is true, seeks to the end (for tail -f behavior on existing files).
// If seekToEnd is false, starts from the beginning (for newly created files).
func (r *ProxyLogReader) openFile(seekToEnd bool) error {
	if r.file != nil {
		return nil // Already open
	}

	file, err := os.Open(r.filePath)
	if err != nil {
		return err
	}

	var offset int64
	if seekToEnd {
		// Seek to end for tail behavior (skip existing content)
		offset, err = file.Seek(0, io.SeekEnd)
		if err != nil {
			_ = file.Close()
			return err
		}
	}
	// If not seekToEnd, offset stays 0 (read from beginning)

	r.file = file
	r.offset = offset
	return nil
}

// closeFile closes the current file handle.
func (r *ProxyLogReader) closeFile() {
	if r.file != nil {
		_ = r.file.Close()
		r.file = nil
		r.offset = 0
	}
}

// readNewLines reads any new lines appended to the file since last read.
func (r *ProxyLogReader) readNewLines() {
	if r.file == nil {
		return
	}

	// Seek to last known position
	if _, err := r.file.Seek(r.offset, io.SeekStart); err != nil {
		return
	}

	scanner := bufio.NewScanner(r.file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Parse the JSONL line
		req, err := ParseProxyRequest(line)
		if err != nil {
			// Skip malformed lines
			continue
		}

		// Convert to LogEntry and send via ChannelSink
		entry := req.ToLogEntry(r.containerName)
		r.sink.Send(entry)
	}

	// Update offset to current position
	if pos, err := r.file.Seek(0, io.SeekCurrent); err == nil {
		r.offset = pos
	}
}

// Close stops the reader and releases resources.
func (r *ProxyLogReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}
	r.closed = true

	r.closeFile()
	return r.watcher.Close()
}
