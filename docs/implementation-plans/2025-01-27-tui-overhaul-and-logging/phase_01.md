# TUI Overhaul and Logging Infrastructure - Phase 1: Logging Infrastructure

**Goal:** Establish structured logging with file rotation and TUI-consumable channel sink.

**Architecture:** Centralized LogManager with Zap Tee core routing logs to both a JSON file (via Lumberjack rotation) and a buffered channel for TUI consumption. Named loggers are scoped hierarchically (e.g., `container.<id>`, `session.<ctr>.<name>`).

**Tech Stack:** Go 1.24+, go.uber.org/zap v1.27.1, gopkg.in/natefinch/lumberjack.v2 v2.2.1

**Scope:** 7 phases from original design (this is phase 1 of 7)

**Codebase verified:** 2025-01-27

---

## Phase Overview

This phase creates the logging foundation that all subsequent phases depend on. We create:
1. LogEntry type for structured TUI consumption
2. ChannelSink implementing zapcore.WriteSyncer for non-blocking TUI integration
3. LogManager with Tee core routing to file (JSON) and channel (console format)

**Testing approach:** Table-driven tests with custom mocks, following existing codebase patterns. No external test frameworks.

---

<!-- START_SUBCOMPONENT_A (tasks 1-3) -->
## Subcomponent A: LogEntry Type and Channel Sink

<!-- START_TASK_1 -->
### Task 1: Add Zap and Lumberjack dependencies

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/go.mod`

**Step 1: Add dependencies**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go get go.uber.org/zap@v1.27.1 gopkg.in/natefinch/lumberjack.v2@v2.2.1
```

Expected: Dependencies added to go.mod and go.sum updated

**Step 2: Verify dependencies**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go mod tidy && go build ./...
```

Expected: Build succeeds with no errors

**Step 3: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add go.mod go.sum && git commit -m "chore: add zap and lumberjack logging dependencies"
```
<!-- END_TASK_1 -->

<!-- START_TASK_2 -->
### Task 2: Create LogEntry type

**Files:**
- Create: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/logging/entries.go`

**Step 1: Write the test**

Create `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/logging/entries_test.go`:

```go
package logging

import (
	"testing"
	"time"
)

func TestLogEntry_String(t *testing.T) {
	tests := []struct {
		name     string
		entry    LogEntry
		contains []string
	}{
		{
			name: "basic entry",
			entry: LogEntry{
				Timestamp: time.Date(2025, 1, 27, 10, 30, 0, 0, time.UTC),
				Level:     "INFO",
				Scope:     "app",
				Message:   "application started",
			},
			contains: []string{"10:30:00", "INFO", "app", "application started"},
		},
		{
			name: "entry with fields",
			entry: LogEntry{
				Timestamp: time.Date(2025, 1, 27, 10, 30, 0, 0, time.UTC),
				Level:     "ERROR",
				Scope:     "container.abc123",
				Message:   "operation failed",
				Fields:    map[string]any{"error": "connection refused"},
			},
			contains: []string{"ERROR", "container.abc123", "operation failed", "connection refused"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entry.String()
			for _, want := range tt.contains {
				if !containsString(got, want) {
					t.Errorf("String() = %q, should contain %q", got, want)
				}
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"debug", "DEBUG"},
		{"DEBUG", "DEBUG"},
		{"info", "INFO"},
		{"INFO", "INFO"},
		{"warn", "WARN"},
		{"warning", "WARN"},
		{"error", "ERROR"},
		{"ERROR", "ERROR"},
		{"unknown", "INFO"},
		{"", "INFO"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseLevel(tt.input)
			if got != tt.want {
				t.Errorf("ParseLevel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/logging/... -v
```

Expected: FAIL - package logging doesn't exist yet

**Step 3: Write the implementation**

Create `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/logging/entries.go`:

```go
package logging

import (
	"fmt"
	"strings"
	"time"
)

// LogEntry represents a structured log entry for TUI consumption.
// It contains all information needed to display and filter logs in the UI.
type LogEntry struct {
	Timestamp time.Time      // When the log was created
	Level     string         // DEBUG, INFO, WARN, ERROR
	Scope     string         // Hierarchical scope (e.g., "container.abc123")
	Message   string         // Log message
	Fields    map[string]any // Additional structured fields
}

// String returns a human-readable representation of the log entry.
func (e LogEntry) String() string {
	var sb strings.Builder
	sb.WriteString(e.Timestamp.Format("15:04:05"))
	sb.WriteString(" ")
	sb.WriteString(e.Level)
	sb.WriteString(" ")
	sb.WriteString("[")
	sb.WriteString(e.Scope)
	sb.WriteString("] ")
	sb.WriteString(e.Message)

	if len(e.Fields) > 0 {
		sb.WriteString(" ")
		first := true
		for k, v := range e.Fields {
			if !first {
				sb.WriteString(" ")
			}
			sb.WriteString(fmt.Sprintf("%s=%v", k, v))
			first = false
		}
	}

	return sb.String()
}

// MatchesScope returns true if the entry's scope starts with the given prefix.
// An empty prefix matches all entries.
func (e LogEntry) MatchesScope(prefix string) bool {
	if prefix == "" {
		return true
	}
	return strings.HasPrefix(e.Scope, prefix)
}

// ParseLevel normalizes a log level string to uppercase.
// Returns "INFO" for unknown levels.
func ParseLevel(level string) string {
	switch strings.ToLower(level) {
	case "debug":
		return "DEBUG"
	case "info":
		return "INFO"
	case "warn", "warning":
		return "WARN"
	case "error":
		return "ERROR"
	default:
		return "INFO"
	}
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/logging/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/logging/entries.go internal/logging/entries_test.go && git commit -m "feat(logging): add LogEntry type for structured TUI consumption"
```
<!-- END_TASK_2 -->

<!-- START_TASK_3 -->
### Task 3: Create ChannelSink

**Files:**
- Create: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/logging/sink.go`
- Create: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/logging/sink_test.go`

**Step 1: Write the test**

Create `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/logging/sink_test.go`:

```go
package logging

import (
	"encoding/json"
	"testing"
	"time"
)

func TestChannelSink_Write(t *testing.T) {
	sink := NewChannelSink(10)
	defer sink.Close()

	// Write a log entry as JSON (simulating what Zap sends)
	entry := map[string]any{
		"level":   "info",
		"ts":      time.Now().Unix(),
		"logger":  "test.scope",
		"msg":     "test message",
		"fieldA":  "valueA",
	}
	data, _ := json.Marshal(entry)
	data = append(data, '\n')

	n, err := sink.Write(data)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("Write() = %d, want %d", n, len(data))
	}

	// Read from channel
	select {
	case got := <-sink.Entries():
		if got.Message != "test message" {
			t.Errorf("Message = %q, want %q", got.Message, "test message")
		}
		if got.Scope != "test.scope" {
			t.Errorf("Scope = %q, want %q", got.Scope, "test.scope")
		}
		if got.Level != "INFO" {
			t.Errorf("Level = %q, want %q", got.Level, "INFO")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for log entry")
	}
}

func TestChannelSink_NonBlocking(t *testing.T) {
	// Create sink with buffer size 2
	sink := NewChannelSink(2)
	defer sink.Close()

	entry := map[string]any{"level": "info", "msg": "test", "logger": "app"}
	data, _ := json.Marshal(entry)
	data = append(data, '\n')

	// Write 5 entries (more than buffer)
	for i := 0; i < 5; i++ {
		n, err := sink.Write(data)
		if err != nil {
			t.Fatalf("Write() error on iteration %d: %v", i, err)
		}
		if n != len(data) {
			t.Errorf("Write() = %d, want %d", n, len(data))
		}
	}

	// Should not block - oldest entries dropped
	// Drain what's available
	count := 0
	for {
		select {
		case <-sink.Entries():
			count++
		default:
			goto done
		}
	}
done:
	// Should have at most buffer size entries
	if count > 2 {
		t.Errorf("got %d entries, expected at most 2", count)
	}
}

func TestChannelSink_Sync(t *testing.T) {
	sink := NewChannelSink(10)
	defer sink.Close()

	// Sync should not error
	if err := sink.Sync(); err != nil {
		t.Errorf("Sync() error = %v", err)
	}
}

func TestChannelSink_Close(t *testing.T) {
	sink := NewChannelSink(10)
	sink.Close()

	// Write after close should not panic
	_, err := sink.Write([]byte(`{"msg":"test"}`))
	if err == nil {
		t.Error("Write() after Close() should return error")
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/logging/... -v -run TestChannelSink
```

Expected: FAIL - NewChannelSink undefined

**Step 3: Write the implementation**

Create `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/logging/sink.go`:

```go
package logging

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ChannelSink implements zapcore.WriteSyncer and routes parsed log entries
// to a channel for TUI consumption. Writes are non-blocking; if the channel
// is full, the oldest entry is dropped.
type ChannelSink struct {
	entries chan LogEntry
	mu      sync.Mutex
	closed  bool
}

// NewChannelSink creates a new channel sink with the specified buffer size.
func NewChannelSink(bufferSize int) *ChannelSink {
	return &ChannelSink{
		entries: make(chan LogEntry, bufferSize),
	}
}

// Write implements io.Writer. It parses the JSON log entry from Zap and
// sends a LogEntry to the channel. Non-blocking: drops oldest if full.
func (s *ChannelSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return 0, fmt.Errorf("channel sink closed")
	}
	s.mu.Unlock()

	entry, err := s.parseEntry(p)
	if err != nil {
		// If we can't parse, still return success to not block logging
		return len(p), nil
	}

	// Non-blocking send with overflow handling
	select {
	case s.entries <- entry:
	default:
		// Channel full - drop oldest and retry
		select {
		case <-s.entries:
		default:
		}
		select {
		case s.entries <- entry:
		default:
		}
	}

	return len(p), nil
}

// Sync implements zapcore.WriteSyncer. No-op for channel sink.
func (s *ChannelSink) Sync() error {
	return nil
}

// Close closes the entries channel. Safe to call multiple times.
func (s *ChannelSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.closed {
		s.closed = true
		close(s.entries)
	}
	return nil
}

// Entries returns the channel for consuming log entries.
func (s *ChannelSink) Entries() <-chan LogEntry {
	return s.entries
}

// parseEntry converts JSON log data from Zap into a LogEntry.
func (s *ChannelSink) parseEntry(data []byte) (LogEntry, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return LogEntry{}, err
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Fields:    make(map[string]any),
	}

	// Extract standard fields
	if msg, ok := raw["msg"].(string); ok {
		entry.Message = msg
		delete(raw, "msg")
	}

	if level, ok := raw["level"].(string); ok {
		entry.Level = ParseLevel(level)
		delete(raw, "level")
	} else {
		entry.Level = "INFO"
	}

	if logger, ok := raw["logger"].(string); ok {
		entry.Scope = logger
		delete(raw, "logger")
	} else {
		entry.Scope = "app"
	}

	// Parse timestamp if present
	if ts, ok := raw["ts"].(float64); ok {
		entry.Timestamp = time.Unix(int64(ts), 0)
		delete(raw, "ts")
	}

	// Remove caller info from fields (keep it internal)
	delete(raw, "caller")
	delete(raw, "stacktrace")

	// Remaining fields go into Fields map
	for k, v := range raw {
		entry.Fields[k] = v
	}

	return entry, nil
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/logging/... -v -run TestChannelSink
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/logging/sink.go internal/logging/sink_test.go && git commit -m "feat(logging): add ChannelSink for non-blocking TUI log consumption"
```
<!-- END_TASK_3 -->
<!-- END_SUBCOMPONENT_A -->

<!-- START_SUBCOMPONENT_B (tasks 4-5) -->
## Subcomponent B: LogManager

<!-- START_TASK_4 -->
### Task 4: Create LogManager with Tee core

**Files:**
- Create: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/logging/manager.go`
- Create: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/logging/manager_test.go`

**Step 1: Write the test**

Create `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/logging/manager_test.go`:

```go
package logging

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := Config{
		FilePath:   logFile,
		MaxSizeMB:  10,
		MaxBackups: 5,
		MaxAgeDays: 7,
		Level:      "debug",
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer mgr.Close()

	// Verify log file was created
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		// File may not exist until first write, that's OK
	}

	// Verify entries channel is available
	if mgr.Entries() == nil {
		t.Error("Entries() returned nil")
	}
}

func TestManager_For(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := Config{
		FilePath:   logFile,
		MaxSizeMB:  10,
		MaxBackups: 5,
		MaxAgeDays: 7,
		Level:      "debug",
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer mgr.Close()

	// Get named logger
	logger := mgr.For("container.abc123")
	if logger == nil {
		t.Fatal("For() returned nil")
	}

	// Same scope should return same logger (cached)
	logger2 := mgr.For("container.abc123")
	if logger != logger2 {
		t.Error("For() should return cached logger for same scope")
	}

	// Different scope should return different logger
	logger3 := mgr.For("container.xyz789")
	if logger == logger3 {
		t.Error("For() should return different logger for different scope")
	}
}

func TestManager_LoggingToChannel(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := Config{
		FilePath:       logFile,
		MaxSizeMB:      10,
		MaxBackups:     5,
		MaxAgeDays:     7,
		Level:          "debug",
		ChannelBufSize: 100,
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer mgr.Close()

	// Log a message
	logger := mgr.For("test.component")
	logger.Info("test message", "key", "value")

	// Sync to ensure write completes
	mgr.Sync()

	// Check channel received entry
	select {
	case entry := <-mgr.Entries():
		if entry.Message != "test message" {
			t.Errorf("Message = %q, want %q", entry.Message, "test message")
		}
		if entry.Scope != "test.component" {
			t.Errorf("Scope = %q, want %q", entry.Scope, "test.component")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for log entry in channel")
	}
}

func TestManager_LoggingToFile(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := Config{
		FilePath:   logFile,
		MaxSizeMB:  10,
		MaxBackups: 5,
		MaxAgeDays: 7,
		Level:      "debug",
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Log a message
	logger := mgr.For("file.test")
	logger.Info("file test message")

	// Close to flush
	mgr.Close()

	// Check file contains entry
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	content := string(data)
	if !containsSubstring(content, "file test message") {
		t.Errorf("log file should contain message, got: %s", content)
	}
	if !containsSubstring(content, "file.test") {
		t.Errorf("log file should contain scope, got: %s", content)
	}
}

func TestManager_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := Config{
		FilePath:   logFile,
		MaxSizeMB:  10,
		MaxBackups: 5,
		MaxAgeDays: 7,
		Level:      "debug",
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer mgr.Close()

	// Create some loggers
	mgr.For("container.abc")
	mgr.For("container.xyz")
	mgr.For("session.abc.s1")

	// Cleanup container.abc and its sessions
	mgr.Cleanup("container.abc")

	// container.abc should be removed from cache
	// But we can't easily test internal cache state without exporting it
	// Just verify no panic and logger still works after cleanup
	logger := mgr.For("container.abc")
	logger.Info("after cleanup")
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/logging/... -v -run TestManager -run TestNew
```

Expected: FAIL - NewManager undefined

**Step 3: Write the implementation**

Create `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/logging/manager.go`:

```go
package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config holds configuration for the LogManager.
type Config struct {
	FilePath       string // Path to log file
	MaxSizeMB      int    // Max size in MB before rotation
	MaxBackups     int    // Max number of old log files to keep
	MaxAgeDays     int    // Max days to keep old log files
	Level          string // Minimum log level (debug, info, warn, error)
	ChannelBufSize int    // Buffer size for TUI channel (default 1000)
}

// Logger wraps slog.Logger to provide a consistent interface.
// We use slog internally but expose a simpler interface.
type Logger struct {
	slog *slog.Logger
	zap  *zap.Logger
}

// Info logs at INFO level.
func (l *Logger) Info(msg string, args ...any) {
	l.slog.Info(msg, args...)
}

// Debug logs at DEBUG level.
func (l *Logger) Debug(msg string, args ...any) {
	l.slog.Debug(msg, args...)
}

// Warn logs at WARN level.
func (l *Logger) Warn(msg string, args ...any) {
	l.slog.Warn(msg, args...)
}

// Error logs at ERROR level.
func (l *Logger) Error(msg string, args ...any) {
	l.slog.Error(msg, args...)
}

// Manager manages loggers with dual output (file + channel).
type Manager struct {
	baseZap     *zap.Logger
	channelSink *ChannelSink
	fileWriter  *lumberjack.Logger
	loggers     map[string]*Logger
	mu          sync.RWMutex
	level       zapcore.Level
}

// NewManager creates a new log manager with the given configuration.
func NewManager(cfg Config) (*Manager, error) {
	// Set defaults
	if cfg.ChannelBufSize == 0 {
		cfg.ChannelBufSize = 1000
	}
	if cfg.MaxSizeMB == 0 {
		cfg.MaxSizeMB = 10
	}
	if cfg.MaxBackups == 0 {
		cfg.MaxBackups = 5
	}
	if cfg.MaxAgeDays == 0 {
		cfg.MaxAgeDays = 7
	}

	// Parse level
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level = zapcore.InfoLevel
	}

	// Ensure log directory exists
	if err := os.MkdirAll(filepath.Dir(cfg.FilePath), 0755); err != nil {
		return nil, err
	}

	// Create file writer with rotation
	fileWriter := &lumberjack.Logger{
		Filename:   cfg.FilePath,
		MaxSize:    cfg.MaxSizeMB,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAgeDays,
		Compress:   true,
	}

	// Create channel sink for TUI
	channelSink := NewChannelSink(cfg.ChannelBufSize)

	// Encoder configuration
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.EpochTimeEncoder
	encoderCfg.EncodeLevel = zapcore.LowercaseLevelEncoder

	// File core (JSON)
	fileCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.AddSync(fileWriter),
		level,
	)

	// Channel core (JSON for parsing)
	channelCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.AddSync(channelSink),
		level,
	)

	// Combine cores with Tee
	core := zapcore.NewTee(fileCore, channelCore)

	// Create base logger
	baseZap := zap.New(core)

	return &Manager{
		baseZap:     baseZap,
		channelSink: channelSink,
		fileWriter:  fileWriter,
		loggers:     make(map[string]*Logger),
		level:       level,
	}, nil
}

// For returns a logger for the given scope.
// Scopes are hierarchical (e.g., "container.abc123", "session.abc.mysession").
// Loggers are cached and reused for the same scope.
func (m *Manager) For(scope string) *Logger {
	m.mu.RLock()
	if logger, ok := m.loggers[scope]; ok {
		m.mu.RUnlock()
		return logger
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if logger, ok := m.loggers[scope]; ok {
		return logger
	}

	// Create named zap logger
	zapLogger := m.baseZap.Named(scope)

	// Create slog handler backed by zap
	slogHandler := &zapSlogHandler{
		zap:   zapLogger,
		level: m.level,
	}

	logger := &Logger{
		slog: slog.New(slogHandler),
		zap:  zapLogger,
	}

	m.loggers[scope] = logger
	return logger
}

// Entries returns the channel for consuming log entries.
func (m *Manager) Entries() <-chan LogEntry {
	return m.channelSink.Entries()
}

// Sync flushes all buffered logs.
func (m *Manager) Sync() error {
	return m.baseZap.Sync()
}

// Cleanup removes cached loggers with the given scope prefix.
// Call this when a container or session is destroyed.
func (m *Manager) Cleanup(scopePrefix string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for scope := range m.loggers {
		if strings.HasPrefix(scope, scopePrefix) {
			delete(m.loggers, scope)
		}
	}
}

// Close syncs and closes all resources.
func (m *Manager) Close() error {
	m.Sync()
	m.channelSink.Close()
	return m.fileWriter.Close()
}

// zapSlogHandler adapts zap.Logger to slog.Handler interface.
type zapSlogHandler struct {
	zap    *zap.Logger
	level  zapcore.Level
	attrs  []slog.Attr
	groups []string
}

func (h *zapSlogHandler) Enabled(_ any, level slog.Level) bool {
	return h.slogToZapLevel(level) >= h.level
}

func (h *zapSlogHandler) Handle(_ any, r slog.Record) error {
	fields := make([]zap.Field, 0, r.NumAttrs()+len(h.attrs))

	// Add handler attrs
	for _, attr := range h.attrs {
		fields = append(fields, zap.Any(attr.Key, attr.Value.Any()))
	}

	// Add record attrs
	r.Attrs(func(attr slog.Attr) bool {
		fields = append(fields, zap.Any(attr.Key, attr.Value.Any()))
		return true
	})

	switch r.Level {
	case slog.LevelDebug:
		h.zap.Debug(r.Message, fields...)
	case slog.LevelInfo:
		h.zap.Info(r.Message, fields...)
	case slog.LevelWarn:
		h.zap.Warn(r.Message, fields...)
	case slog.LevelError:
		h.zap.Error(r.Message, fields...)
	default:
		h.zap.Info(r.Message, fields...)
	}

	return nil
}

func (h *zapSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &zapSlogHandler{
		zap:    h.zap,
		level:  h.level,
		attrs:  newAttrs,
		groups: h.groups,
	}
}

func (h *zapSlogHandler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name
	return &zapSlogHandler{
		zap:    h.zap.Named(name),
		level:  h.level,
		attrs:  h.attrs,
		groups: newGroups,
	}
}

func (h *zapSlogHandler) slogToZapLevel(level slog.Level) zapcore.Level {
	switch {
	case level >= slog.LevelError:
		return zapcore.ErrorLevel
	case level >= slog.LevelWarn:
		return zapcore.WarnLevel
	case level >= slog.LevelInfo:
		return zapcore.InfoLevel
	default:
		return zapcore.DebugLevel
	}
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/logging/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/logging/manager.go internal/logging/manager_test.go && git commit -m "feat(logging): add LogManager with Zap Tee core and named loggers"
```
<!-- END_TASK_4 -->

<!-- START_TASK_5 -->
### Task 5: Test file rotation

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/logging/manager_test.go`

**Step 1: Add rotation test**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/logging/manager_test.go`:

```go
func TestManager_FileRotation(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "rotate.log")

	// Use tiny max size to trigger rotation
	cfg := Config{
		FilePath:   logFile,
		MaxSizeMB:  1, // 1MB - smallest practical size
		MaxBackups: 2,
		MaxAgeDays: 7,
		Level:      "debug",
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer mgr.Close()

	logger := mgr.For("rotation.test")

	// Write enough data to potentially trigger rotation
	// This is more of a smoke test - actual rotation happens at file level
	bigMessage := string(make([]byte, 1000))
	for i := 0; i < 100; i++ {
		logger.Info(bigMessage, "iteration", i)
	}

	mgr.Sync()

	// Verify file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("log file should exist after writing")
	}
}
```

**Step 2: Run test**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/logging/... -v -run TestManager_FileRotation
```

Expected: PASS

**Step 3: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/logging/manager_test.go && git commit -m "test(logging): add file rotation smoke test"
```
<!-- END_TASK_5 -->
<!-- END_SUBCOMPONENT_B -->

<!-- START_TASK_6 -->
### Task 6: Run all tests and verify phase complete

**Step 1: Run all logging tests**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/logging/... -v
```

Expected: All tests pass

**Step 2: Run full test suite**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./...
```

Expected: All existing tests still pass

**Step 3: Verify build**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go build ./...
```

Expected: Build succeeds
<!-- END_TASK_6 -->

<!-- START_SUBCOMPONENT_C (tasks 7) -->
## Subcomponent C: Test Utilities

<!-- START_TASK_7 -->
### Task 7: Add NopLogger and NewTestLogManager utilities

**Files:**
- Create: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/logging/testing.go`

**Step 1: Write the test**

Create `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/logging/testing_test.go`:

```go
package logging

import (
	"testing"
)

func TestNopLogger(t *testing.T) {
	logger := NopLogger()
	if logger == nil {
		t.Fatal("NopLogger() returned nil")
	}

	// Should not panic
	logger.Debug("test")
	logger.Info("test")
	logger.Warn("test")
	logger.Error("test")
}

func TestNopLogger_With(t *testing.T) {
	logger := NopLogger()
	withLogger := logger.With("key", "value")
	if withLogger == nil {
		t.Fatal("With() returned nil")
	}

	// Should not panic
	withLogger.Info("test with fields")
}

func TestNewTestLogManager(t *testing.T) {
	lm := NewTestLogManager(10)
	if lm == nil {
		t.Fatal("NewTestLogManager() returned nil")
	}
	defer lm.Close()

	// Get logger and write
	logger := lm.For("test")
	logger.Info("test message")

	// Should receive entry on channel
	select {
	case entry := <-lm.Channel():
		if entry.Message != "test message" {
			t.Errorf("expected 'test message', got %q", entry.Message)
		}
		if entry.Scope != "test" {
			t.Errorf("expected scope 'test', got %q", entry.Scope)
		}
	default:
		t.Error("no entry received on channel")
	}
}

func TestNewTestLogManager_Channel(t *testing.T) {
	lm := NewTestLogManager(5)
	defer lm.Close()

	ch := lm.Channel()
	if ch == nil {
		t.Error("Channel() returned nil")
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/logging/... -v -run TestNopLogger
```

Expected: FAIL - NopLogger undefined

**Step 3: Write the implementation**

Create `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/logging/testing.go`:

```go
package logging

import (
	"log/slog"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ScopedLogger provides a logger with scope context and additional field support.
// This is the public interface returned by LogManager.Logger().
type ScopedLogger struct {
	slog  *slog.Logger
	zap   *zap.Logger
	scope string
}

// Info logs at INFO level.
func (l *ScopedLogger) Info(msg string, args ...any) {
	if l.slog != nil {
		l.slog.Info(msg, args...)
	}
}

// Debug logs at DEBUG level.
func (l *ScopedLogger) Debug(msg string, args ...any) {
	if l.slog != nil {
		l.slog.Debug(msg, args...)
	}
}

// Warn logs at WARN level.
func (l *ScopedLogger) Warn(msg string, args ...any) {
	if l.slog != nil {
		l.slog.Warn(msg, args...)
	}
}

// Error logs at ERROR level.
func (l *ScopedLogger) Error(msg string, args ...any) {
	if l.slog != nil {
		l.slog.Error(msg, args...)
	}
}

// With returns a new ScopedLogger with the given key-value pairs added to all log entries.
func (l *ScopedLogger) With(args ...any) *ScopedLogger {
	if l.slog == nil {
		return l
	}
	return &ScopedLogger{
		slog:  l.slog.With(args...),
		zap:   l.zap,
		scope: l.scope,
	}
}

// NopLogger returns a logger that discards all output.
// Use in tests or when logging is not configured.
func NopLogger() *ScopedLogger {
	return &ScopedLogger{
		slog:  nil, // nil slog means all logging is no-op
		zap:   nil,
		scope: "",
	}
}

// TestLogManager provides a LogManager suitable for tests.
// It writes to a channel only (no file) for easy verification.
type TestLogManager struct {
	channelSink *ChannelSink
	baseZap     *zap.Logger
	loggers     map[string]*ScopedLogger
}

// NewTestLogManager creates a LogManager for testing that only writes to a channel.
func NewTestLogManager(bufferSize int) *TestLogManager {
	channelSink := NewChannelSink(bufferSize)

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.EpochTimeEncoder
	encoderCfg.EncodeLevel = zapcore.LowercaseLevelEncoder

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.AddSync(channelSink),
		zapcore.DebugLevel,
	)

	return &TestLogManager{
		channelSink: channelSink,
		baseZap:     zap.New(core),
		loggers:     make(map[string]*ScopedLogger),
	}
}

// For returns a scoped logger for the given scope name.
// Named For() to match the production Manager API.
func (m *TestLogManager) For(scope string) *ScopedLogger {
	if logger, ok := m.loggers[scope]; ok {
		return logger
	}

	zapLogger := m.baseZap.Named(scope)
	slogHandler := &zapSlogHandler{
		zap:   zapLogger,
		level: zapcore.DebugLevel,
	}

	logger := &ScopedLogger{
		slog:  slog.New(slogHandler),
		zap:   zapLogger,
		scope: scope,
	}

	m.loggers[scope] = logger
	return logger
}

// Channel returns the channel for receiving log entries.
func (m *TestLogManager) Channel() <-chan LogEntry {
	return m.channelSink.Entries()
}

// Close closes the test log manager.
func (m *TestLogManager) Close() error {
	return m.channelSink.Close()
}
```

**Step 4: Update LogManager to use ScopedLogger**

The `LogManager.Logger()` method should return `*ScopedLogger` instead of `*Logger`. Update `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/logging/manager.go` to:

1. Remove the `Logger` type (it's now `ScopedLogger` in testing.go)
2. Update `Manager.For()` to return `*ScopedLogger`
3. Update internal `loggers` map to use `*ScopedLogger`

The key changes:
- Rename internal map type: `loggers map[string]*ScopedLogger`
- Return type of `For()`: `*ScopedLogger`

**Step 5: Run tests to verify they pass**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/logging/... -v
```

Expected: All tests PASS

**Step 6: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/logging/testing.go internal/logging/testing_test.go internal/logging/manager.go && git commit -m "feat(logging): add NopLogger and NewTestLogManager test utilities"
```
<!-- END_TASK_7 -->
<!-- END_SUBCOMPONENT_C -->

---

## Phase Completion Checklist

- [ ] LogManager can be instantiated with file path
- [ ] Named loggers can be created and retrieved
- [ ] Log entries appear in both file (JSON) and channel (LogEntry structs)
- [ ] File rotation configured (actual rotation tested via Lumberjack)
- [ ] NopLogger() returns a no-op logger for tests
- [ ] NewTestLogManager() creates a channel-only manager for tests
- [ ] ScopedLogger.With() supports adding fields
- [ ] Unit tests pass for logger creation, entry routing
- [ ] All existing tests still pass
