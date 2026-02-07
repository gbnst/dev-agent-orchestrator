# Logging Domain

Last verified: 2026-02-06

## Purpose
Provides structured logging with dual output: rotating JSON files for post-mortem analysis and a buffered channel for live TUI consumption. Scoped loggers enable automatic filtering by context. Supports external log sources (proxy logs) via direct channel injection.

## Contracts
- **Exposes**: `Manager`, `ScopedLogger`, `LogEntry`, `LoggerProvider` interface, `NopLogger()`, `NewTestLogManager()`, `ProxyRequest`, `ProxyLogReader`, `ParseProxyRequest()`
- **Guarantees**: Channel never blocks (drops oldest on overflow). File rotation at configured size. Scopes are hierarchical (e.g., `container.abc123`, `proxy.abc123`). ProxyLogReader uses fsnotify + 5s polling safeguard for Docker bind mount compatibility.
- **Expects**: Valid file path for log output. Caller consumes channel entries to prevent memory growth.

## Dependencies
- **Uses**: go.uber.org/zap, gopkg.in/natefinch/lumberjack.v2, github.com/fsnotify/fsnotify
- **Used by**: TUI (Model), container.Manager, tmux.Client, main.go
- **Boundary**: Pure logging infrastructure; no domain logic

## Key Decisions
- Zap over slog: Tee core enables dual sinks without custom handler
- JSON file format: grep-friendly for debugging
- 1000-entry ring buffer: Bounds TUI memory usage
- ProxyLogReader uses ChannelSink.Send() for non-Zap log injection
- ProxyRequest stored in LogEntry.Fields["_proxyRequest"] for details panel access

## Invariants
- ScopedLogger.Info/Debug/Warn/Error are nil-safe (NopLogger pattern)
- LogEntry.Scope always set (defaults to "app" if missing)
- Channel sink is non-blocking; full buffer drops oldest entry
- ChannelSink.Write() holds mutex for closed-check and channel send atomically; JSON parsing happens outside lock
- ChannelSink.Send() is non-blocking with same overflow behavior as Write()
- ProxyLogReader tails from end of file (tail -f behavior); doesn't replay history

## Key Files
- `manager.go` - Manager with Zap Tee core, ScopedLogger, LoggerProvider interface, GetChannelSink()
- `sink.go` - ChannelSink implementing zapcore.WriteSyncer, Send() for external sources
- `entries.go` - LogEntry struct with MatchesScope() for filtering
- `proxy.go` - ProxyRequest struct, ProxyLogReader (file watcher + JSONL parser), ParseProxyRequest()
- `testing.go` - NopLogger() and NewTestLogManager() for tests

## Gotchas
- Manager.For() caches loggers; call Cleanup() when containers destroyed
- LogEntry.Fields is mutable map; don't modify after creation
- ProxyLogReader watches parent directory (file may not exist yet)
- ProxyLogReader.Start() blocks until context cancelled
