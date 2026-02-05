// pattern: Imperative Shell

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
	// Parse outside the lock — parseEntry is a pure function with no shared state
	entry, err := s.parseEntry(p)
	if err != nil {
		// If we can't parse, still return success to not block logging
		return len(p), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, fmt.Errorf("write to closed channel sink")
	}

	// Non-blocking send with overflow handling — protected by mutex
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

	// Parse timestamp if present, preserving nanosecond precision
	if ts, ok := raw["ts"].(float64); ok {
		sec := int64(ts)
		nsec := int64((ts - float64(sec)) * 1e9)
		entry.Timestamp = time.Unix(sec, nsec)
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
