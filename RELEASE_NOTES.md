# devagent v0.1.0

First feature release since v0.0.1. Highlights below; the full list of merged
pull requests is appended automatically.

## 🔒 Security

- **Prevent sandbox escape via app-writable `filter.py`.** The mitmproxy egress
  allowlist lived inside the project tree the app container mounts read-write,
  so an agent could rewrite the allowlist (mitmproxy hot-reloads the script) and
  escape network isolation. The app now gets a read-only bind over the proxy
  config subtree — it can still read logs but can no longer write the filter or
  poison its bytecode cache.
- **Close egress allowlist bypass via spoofed `Host` header.** Authorization is
  enforced on the real connection target (and the `Host` header is also required
  to be allowlisted) rather than the client-controlled header alone.

## ✨ Features

- **First-run provisioning.** Config and devcontainer templates are now embedded
  in the binary and materialized into `~/.config/devagent/` on first run — no
  manual copying. `config.yaml` is written once and never overwritten; bundled
  templates are refreshed on upgrade (so template/security fixes reach you), with
  any edited template backed up to `templates.backup-<timestamp>/` first.
  Templates you add yourself are left untouched.
- **VS Code attach.** Press `v` to open a container in VS Code via the
  attached-container URI scheme; container IDs are now resolved in full
  (`docker ps --no-trunc`).
- **Structured CLI.** New `session readlines`/`tail`/`send` commands, an agent
  orchestration guide (`--agent-help`), and CLI delegation to the running
  instance with a standalone fallback.
- **Web UI lifecycle management.** Create, start, stop, and destroy projects,
  worktrees, and containers from the embedded SPA.
- **Single-instance enforcement.** File-based locking, instance discovery, an
  always-on web server, and crash recovery (`devagent cleanup`).

## 🐛 Fixes

- Match containers to worktrees by compose project name.
- Correct main worktree container paths and restore `sudo` in entrypoints.
- Show the `v` (VS Code) shortcut in the status-bar help when a container or
  session node is selected — the action worked but was never advertised.

## 📦 Dependencies

- Bump `github.com/charmbracelet/x/ansi` to 0.11.7, `go.uber.org/zap` to 1.28.0,
  and `github.com/fsnotify/fsnotify` to 1.10.1.
- Bump GitHub Actions: `softprops/action-gh-release` 2→3,
  `actions/upload-artifact` 4→7, `actions/download-artifact` 4→8.

---
