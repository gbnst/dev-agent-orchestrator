// pattern: Imperative Shell

package logging

import (
	"context"
	"fmt"
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

// LoggerProvider is an interface for obtaining scoped loggers.
// Both Manager and TestLogManager implement this interface.
type LoggerProvider interface {
	For(scope string) *ScopedLogger
}

// ScopedLogger provides a logger with scope context and additional field support.
// This is the public interface returned by LogManager.For().
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

// Scope returns the logger's hierarchical scope.
func (l *ScopedLogger) Scope() string {
	return l.scope
}

// Manager manages loggers with dual output (file + channel).
type Manager struct {
	baseZap     *zap.Logger
	channelSink *ChannelSink
	fileWriter  *lumberjack.Logger
	loggers     map[string]*ScopedLogger
	mu          sync.RWMutex
	level       zapcore.Level
}

// NewManager creates a new log manager with the given configuration.
func NewManager(cfg Config) (*Manager, error) {
	// Validate required configuration
	if cfg.FilePath == "" {
		return nil, fmt.Errorf("FilePath is required")
	}

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
		loggers:     make(map[string]*ScopedLogger),
		level:       level,
	}, nil
}

// For returns a logger for the given scope.
// Scopes are hierarchical (e.g., "container.abc123", "session.abc.mysession").
// Loggers are cached and reused for the same scope.
func (m *Manager) For(scope string) *ScopedLogger {
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

	logger := &ScopedLogger{
		slog:  slog.New(slogHandler),
		zap:   zapLogger,
		scope: scope,
	}

	m.loggers[scope] = logger
	return logger
}

// Entries returns the channel for consuming log entries.
func (m *Manager) Entries() <-chan LogEntry {
	return m.channelSink.Entries()
}

// GetChannelSink returns the channel sink for external log sources.
// This allows proxy log readers to send entries directly to the TUI channel.
func (m *Manager) GetChannelSink() *ChannelSink {
	return m.channelSink
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
	_ = m.Sync()
	_ = m.channelSink.Close()
	return m.fileWriter.Close()
}

// zapSlogHandler adapts zap.Logger to slog.Handler interface.
type zapSlogHandler struct {
	zap    *zap.Logger
	level  zapcore.Level
	attrs  []slog.Attr
	groups []string
}

func (h *zapSlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.slogToZapLevel(level) >= h.level
}

func (h *zapSlogHandler) Handle(_ context.Context, r slog.Record) error {
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
