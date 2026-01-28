// pattern: Imperative Shell

package logging

import (
	"log/slog"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

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
	mu          sync.RWMutex
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
