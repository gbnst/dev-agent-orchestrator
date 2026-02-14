package logging

import (
	"testing"
	"time"
)

func TestParseProxyRequest_ValidJSON(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantMethod string
		wantURL    string
		wantStatus int
	}{
		{
			name:       "simple GET request",
			input:      `{"ts": 1707235200.123, "method": "GET", "url": "https://api.example.com/users", "status": 200, "duration_ms": 45, "req_headers": {}, "res_headers": {}, "req_body": null, "res_body": null}`,
			wantMethod: "GET",
			wantURL:    "https://api.example.com/users",
			wantStatus: 200,
		},
		{
			name:       "POST with body",
			input:      `{"ts": 1707235200.0, "method": "POST", "url": "https://api.example.com/data", "status": 201, "duration_ms": 100, "req_headers": {"Content-Type": "application/json"}, "res_headers": {"Content-Type": "application/json"}, "req_body": "{\"key\": \"value\"}", "res_body": "{\"id\": 1}"}`,
			wantMethod: "POST",
			wantURL:    "https://api.example.com/data",
			wantStatus: 201,
		},
		{
			name:       "error response",
			input:      `{"ts": 1707235200.0, "method": "GET", "url": "https://api.example.com/notfound", "status": 404, "duration_ms": 10, "req_headers": {}, "res_headers": {}, "req_body": null, "res_body": null}`,
			wantMethod: "GET",
			wantURL:    "https://api.example.com/notfound",
			wantStatus: 404,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := ParseProxyRequest([]byte(tt.input))
			if err != nil {
				t.Fatalf("ParseProxyRequest() error = %v", err)
			}
			if req.Method != tt.wantMethod {
				t.Errorf("Method = %v, want %v", req.Method, tt.wantMethod)
			}
			if req.URL != tt.wantURL {
				t.Errorf("URL = %v, want %v", req.URL, tt.wantURL)
			}
			if req.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", req.Status, tt.wantStatus)
			}
		})
	}
}

func TestParseProxyRequest_InvalidJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "invalid JSON",
			input: "not json",
		},
		{
			name:  "incomplete JSON",
			input: `{"ts": 123`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseProxyRequest([]byte(tt.input))
			if err == nil {
				t.Error("ParseProxyRequest() expected error, got nil")
			}
		})
	}
}

func TestParseProxyRequest_Timestamp(t *testing.T) {
	input := `{"ts": 1707235200.5, "method": "GET", "url": "https://example.com", "status": 200, "duration_ms": 0, "req_headers": {}, "res_headers": {}, "req_body": null, "res_body": null}`

	req, err := ParseProxyRequest([]byte(input))
	if err != nil {
		t.Fatalf("ParseProxyRequest() error = %v", err)
	}

	// Verify timestamp parsed correctly
	expected := time.Unix(1707235200, 500000000)
	if !req.Timestamp.Equal(expected) {
		t.Errorf("Timestamp = %v, want %v", req.Timestamp, expected)
	}
}

func TestProxyRequest_ToLogEntry(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		wantLevel     string
		containerName string
		wantScope     string
	}{
		{
			name:          "success status - INFO",
			status:        200,
			wantLevel:     "INFO",
			containerName: "myapp",
			wantScope:     "proxy.myapp",
		},
		{
			name:          "client error - WARN",
			status:        404,
			wantLevel:     "WARN",
			containerName: "myapp",
			wantScope:     "proxy.myapp",
		},
		{
			name:          "server error - ERROR",
			status:        500,
			wantLevel:     "ERROR",
			containerName: "myapp",
			wantScope:     "proxy.myapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ProxyRequest{
				Timestamp:  time.Now(),
				Method:     "GET",
				URL:        "https://example.com/path",
				Status:     tt.status,
				DurationMs: 100,
			}

			entry := req.ToLogEntry(tt.containerName)

			if entry.Level != tt.wantLevel {
				t.Errorf("Level = %v, want %v", entry.Level, tt.wantLevel)
			}
			if entry.Scope != tt.wantScope {
				t.Errorf("Scope = %v, want %v", entry.Scope, tt.wantScope)
			}
			if entry.Fields["_proxyRequest"] == nil {
				t.Error("Fields[_proxyRequest] should not be nil")
			}
		})
	}
}

func TestProxyRequest_ToLogEntry_Message(t *testing.T) {
	req := &ProxyRequest{
		Timestamp:  time.Now(),
		Method:     "POST",
		URL:        "https://api.example.com/data",
		Status:     201,
		DurationMs: 250,
	}

	entry := req.ToLogEntry("container")

	expectedMsg := "201 POST https://api.example.com/data 250ms"
	if entry.Message != expectedMsg {
		t.Errorf("Message = %v, want %v", entry.Message, expectedMsg)
	}
}

func TestNewProxyLogReader_WithChannelSink(t *testing.T) {
	sink := NewChannelSink(10)
	defer func() { _ = sink.Close() }()

	tmpDir := t.TempDir()
	logPath := tmpDir + "/requests.jsonl"

	reader, err := NewProxyLogReader(logPath, "testcontainer", sink)
	if err != nil {
		t.Fatalf("NewProxyLogReader() error = %v", err)
	}

	// Verify reader was created with correct fields
	if reader.filePath != logPath {
		t.Errorf("filePath = %v, want %v", reader.filePath, logPath)
	}
	if reader.containerName != "testcontainer" {
		t.Errorf("containerName = %v, want %v", reader.containerName, "testcontainer")
	}
	if reader.sink != sink {
		t.Error("sink not set correctly")
	}
}
