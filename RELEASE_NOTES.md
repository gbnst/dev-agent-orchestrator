# devagent v0.2.0

Sandbox hardening and a new Python-only template. Highlights below; the full
list of merged pull requests is appended automatically.

## 🔒 Security

- **Make the whole `.devcontainer` directory read-only in the app container.**
  v0.1.0 shadowed only the proxy config subtree (`filter.py`) read-only. That
  left the rest of `.devcontainer` — `docker-compose.yml`, `Dockerfile`,
  `entrypoint.sh`, `post-create.sh` — writable through the workspace mount, so an
  agent could rewrite the container definition itself. The read-only bind now
  covers the entire `.devcontainer` tree, denying all writes under it. The proxy
  keeps its own read-write bind, and the persistent dotfiles and `.claude` state
  are mounted separately into `/home/vscode` (outside the workspace), so they
  stay writable. `cap_drop` of `SYS_ADMIN` still blocks remounting it
  read-write.

## ✨ Features

- **New `python-project` template.** A pure-Python variant of
  `python-fullstack` with the Node.js/npm toolchain removed: no Node install in
  the Dockerfile, no `NODE_EXTRA_CA_CERTS`, and `registry.npmjs.org` dropped from
  the proxy allowlist. Keeps Python 3.12, `uv`, `tmux`, `gh`, and Claude Code.
  Existing installs pick it up on upgrade through the same template-sync that
  delivers the hardening above.

## 🛠 Build & CI

- Build the Windows package in CI and drop the Nix `preCheck` HOME workaround
  (#45).
- Isolate the data directory in web API tests so they run hermetically.

---
