# Logging Domain

Last verified: 2026-01-27

## Purpose
Provides structured logging with dual output: rotating JSON files for post-mortem analysis and a buffered channel for live TUI consumption. Scoped loggers enable automatic filtering by context.

## Contracts
- **Exposes**: `Manager`, `ScopedLogger`, `LogEntry`, `LoggerProvider` interface, `NopLogger()`, `NewTestLogManager()`
- **Guarantees**: Channel never blocks (drops oldest on overflow). File rotation at configured size. Scopes are hierarchical (e.g., `container.abc123`).
- **Expects**: Valid file path for log output. Caller consumes channel entries to prevent memory growth.

## Dependencies
- **Uses**: go.uber.org/zap, gopkg.in/natefinch/lumberjack.v2
- **Used by**: TUI (Model), container.Manager, tmux.Client, main.go
- **Boundary**: Pure logging infrastructure; no domain logic

## Key Decisions
- Zap over slog: Tee core enables dual sinks without custom handler
- JSON file format: grep-friendly for debugging
- 1000-entry ring buffer: Bounds TUI memory usage

## Invariants
- ScopedLogger.Info/Debug/Warn/Error are nil-safe (NopLogger pattern)
- LogEntry.Scope always set (defaults to "app" if missing)
- Channel sink is non-blocking; full buffer drops oldest entry

## Key Files
- `manager.go` - Manager with Zap Tee core, ScopedLogger, LoggerProvider interface
- `sink.go` - ChannelSink implementing zapcore.WriteSyncer
- `entries.go` - LogEntry struct with MatchesScope() for filtering
- `testing.go` - NopLogger() and NewTestLogManager() for tests

## Gotchas
- Manager.For() caches loggers; call Cleanup() when containers destroyed
- LogEntry.Fields is mutable map; don't modify after creation
