# Proxy Logging and Details Panel Design

## Summary

Add comprehensive HTTP request logging to the mitmproxy sidecar, with logs displayed in the TUI alongside container logs. Introduce a details panel (toggled via arrow keys) that shows full request/response data for proxy entries or structured fields for regular log entries. Domain configuration gains a `log` attribute to opt specific domains out of logging.

## Definition of Done

1. **Proxy request logging**: mitmproxy writes JSONL to a mounted volume with full request/response data (method, URL, headers, status, duration, bodies) — text content truncated at 100KB, binary content not logged.

2. **Configurable logging opt-out**: Domain entries in filter.py gain an attribute to disable logging for specific URLs/domains (allowed through proxy but not logged).

3. **Unified log view**: Proxy request logs appear interspersed with container logs in the TUI log panel, scoped to the selected container (not visible in "all logs").

4. **Details panel**: Pressing right arrow opens a 60%-width details panel on the right showing:
   - For proxy requests: full request/response details (headers, bodies, etc.)
   - For regular log entries: the structured Fields map

5. **Navigation**: Left arrow hides the details panel, returning to the log list view.

**Out of scope:**
- Changing how regular container logs are captured
- Proxy logs visible in "all logs" view (only per-container)

## Glossary

- **JSONL**: JSON Lines format — one JSON object per line, suitable for append-only logging and streaming reads
- **Proxy sidecar**: The mitmproxy container that runs alongside the app container, intercepting all HTTP/HTTPS traffic
- **Bind mount**: Docker volume type that maps a host filesystem path into a container
- **Scope**: Hierarchical log category (e.g., `container.myapp`, `proxy.myapp`) used for filtering

## Architecture

### Data Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Host Filesystem                                                          │
│                                                                          │
│  <project>/.devcontainer/proxy-logs/requests.jsonl                      │
│       ▲                                              │                   │
│       │ write                                        │ tail/read         │
│       │                                              ▼                   │
│  ┌────┴─────────────┐                         ┌──────────────┐          │
│  │ Proxy Container  │                         │ devagent TUI │          │
│  │ (mitmproxy)      │                         │ (host)       │          │
│  └──────────────────┘                         └──────────────┘          │
│                                                      │                   │
│                                                      ▼                   │
│                                               Log Panel with             │
│                                               Details View               │
└─────────────────────────────────────────────────────────────────────────┘
```

### Components Modified

1. **filter.py.tmpl** — Add request logging with JSONL output and log opt-out support
2. **docker-compose.yml.tmpl** — Add bind mount for proxy logs directory
3. **internal/logging/** — Add proxy log reader and ProxyRequest type
4. **internal/tui/** — Add details panel, arrow key navigation, proxy log integration

## Existing Patterns Followed

### Logging System (internal/logging/)
- `LogEntry` struct with `Timestamp`, `Level`, `Scope`, `Message`, `Fields`
- `MatchesScope(prefix)` for filtering by scope prefix
- Channel-based log consumption in TUI via `Manager.Entries()`

### TUI Architecture (internal/tui/)
- Bubbletea model-update-view pattern
- Panel toggling via boolean flags (e.g., `logPanelOpen`)
- Viewport for scrollable content
- Focus management between panels (`panelFocus` enum)

### Template System (internal/container/compose.go)
- `TemplateData` struct passed to Go templates
- Templates in `config/templates/<name>/`

## Design Details

### 1. Domain Configuration Format

Extend `ALLOWED_DOMAINS` to support dict entries with a `log` attribute:

```python
ALLOWED_DOMAINS = [
    {"domain": "api.anthropic.com", "log": True},
    {"domain": "statsigapi.net", "log": False},  # Allow but don't log
    "*.github.com",  # Plain string = log: True (backward compatible)
]
```

Helper function normalizes entries:
```python
def _parse_domain_entry(entry):
    if isinstance(entry, str):
        return {"domain": entry, "log": True}
    return entry
```

### 2. JSONL Log Format

Each line is a JSON object:

```json
{
  "ts": 1707235200.123,
  "method": "GET",
  "url": "https://api.github.com/user",
  "status": 200,
  "duration_ms": 45,
  "req_headers": {"Content-Type": "application/json", "Authorization": "Bearer sk-..."},
  "res_headers": {"Content-Type": "application/json"},
  "req_body": null,
  "res_body": "{\"login\": \"user\", ...}"
}
```

Rules:
- `req_body` / `res_body`: `null` if binary content-type or empty; truncated at 100KB with `[truncated]` suffix
- Binary detection: Check `Content-Type` header for `text/`, `application/json`, `application/xml`, etc.

### 3. File Location and .gitignore

```
<project>/
└── .devcontainer/
    ├── devcontainer.json      # Committed
    ├── .gitignore             # Committed (ignores runtime data)
    └── proxy-logs/            # Runtime, git-ignored
        └── requests.jsonl
```

`.devcontainer/.gitignore`:
```gitignore
proxy-logs/
*.jsonl
```

Docker Compose template addition:
```yaml
proxy:
  volumes:
    - {{.ProjectPath}}/.devcontainer/proxy-logs:/var/log/proxy
```

### 4. Go Proxy Log Reader

New file `internal/logging/proxy.go`:

```go
type ProxyRequest struct {
    Timestamp   time.Time
    Method      string
    URL         string
    Status      int
    DurationMs  int64
    ReqHeaders  map[string]string
    ResHeaders  map[string]string
    ReqBody     *string  // nil if binary/empty
    ResBody     *string  // nil if binary/empty
}

type ProxyLogReader struct {
    path      string
    entries   chan LogEntry
    // ...
}

func (r *ProxyLogReader) Start(ctx context.Context, containerName string) error
```

- Tails the JSONL file using `fsnotify` or polling
- Parses each line into `ProxyRequest`
- Converts to `LogEntry` with:
  - `Scope`: `"proxy." + containerName`
  - `Level`: `"INFO"` (or `"WARN"` for 4xx, `"ERROR"` for 5xx)
  - `Message`: `"GET https://api.github.com/user 200 45ms"`
  - `Fields["_proxyRequest"]`: Full `ProxyRequest` struct for details panel

### 5. TUI Details Panel

New model fields:
```go
type Model struct {
    // ...existing fields
    detailsPanelOpen  bool
    selectedLogIndex  int
    detailsViewport   viewport.Model
}
```

Layout when details panel is open:
```
┌──────────────────────────────────┬──────────────────────────────────────────────┐
│ Log List (40%)                   │ Details Panel (60%)                          │
│                                  │                                              │
│ 15:04:05 INFO [container] msg    │ Request Details                              │
│ 15:04:06 GET api.github.com 200  │ ─────────────────────────────────────────── │
│ 15:04:07 WARN [container] msg    │ Method: GET                                  │
│ > 15:04:08 POST api.anthr... 201 │ URL: https://api.anthropic.com/v1/messages   │
│                                  │ Status: 201 Created                          │
│                                  │ Duration: 1.2s                               │
│                                  │                                              │
│                                  │ Request Headers:                             │
│                                  │   Content-Type: application/json             │
│                                  │   ...                                        │
└──────────────────────────────────┴──────────────────────────────────────────────┘
```

Key bindings:
- `→` (Right arrow): Open details panel, focus moves to details
- `←` (Left arrow): Close details panel, focus returns to log list
- `↑/↓` in log list: Change selected entry, details update
- `↑/↓` in details panel: Scroll details viewport

Details content:
- **For proxy requests** (`Fields["_proxyRequest"]` exists): Show formatted request/response
- **For regular logs**: Show `Fields` map as key-value pairs

### 6. Proxy Log Lifecycle

1. **Container start**: Go creates `.devcontainer/proxy-logs/` directory, ensures `.gitignore` exists
2. **Proxy writes**: mitmproxy appends to `requests.jsonl` on each request
3. **Go tails**: `ProxyLogReader` watches the file, emits `LogEntry` to the logging channel
4. **TUI displays**: Log entries interspersed by timestamp, filtered by container scope
5. **Container stop**: Log file persists for later review; cleaned up on container destroy (optional)

## Implementation Phases

### Phase 1: Proxy JSONL Logging
- Modify `filter.py.tmpl` to write JSONL with request/response data
- Add `log` attribute support to domain configuration
- Implement text truncation (100KB) and binary detection

### Phase 2: Docker Compose Integration
- Add bind mount for proxy logs in `docker-compose.yml.tmpl`
- Add `ProxyLogPath` to `TemplateData` struct
- Create `.devcontainer/proxy-logs/` directory on container start
- Ensure `.devcontainer/.gitignore` excludes runtime data

### Phase 3: Go Proxy Log Reader
- Create `ProxyRequest` type in `internal/logging/`
- Implement `ProxyLogReader` with file tailing
- Convert proxy requests to `LogEntry` with `proxy.<name>` scope
- Integrate with existing logging channel

### Phase 4: TUI Details Panel
- Add `detailsPanelOpen`, `selectedLogIndex`, `detailsViewport` to Model
- Implement left/right arrow key handling
- Render details panel at 60% width
- Format proxy request details and log entry fields

### Phase 5: Polish and Edge Cases
- Handle log file rotation/truncation
- Add visual indicator for selected log entry
- Test with large request/response bodies
- Verify .gitignore is created correctly

## Additional Considerations

### Performance
- File tailing should use efficient watching (fsnotify) rather than polling
- Large JSONL files: Consider log rotation or max file size
- Details panel rendering: Cache formatted content to avoid re-rendering on scroll

### Error Handling
- Missing log file: Create on first write, reader waits for file to exist
- Malformed JSONL lines: Skip and log warning
- File permission errors: Surface in TUI logs

### Future Enhancements (Out of Scope)
- Search/filter within proxy logs
- Export proxy logs to HAR format
- Request replay from details panel
