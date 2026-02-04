# Claude Token Auto-Provisioning Design

## Summary

Enhance devagent's Claude Code authentication by auto-provisioning OAuth tokens and injecting them securely via bind mounts and shell profiles, eliminating secrets from devcontainer.json files.

## Definition of Done

1. **Token auto-provisioning**: devagent checks for `~/.claude/.devagent-claude-token` (XDG-aware), creates it via `claude setup-token` if missing
2. **No secrets in devcontainer.json**: Token injected via shell profile reading a mounted file, not as containerEnv value
3. **Read-only bind mount**: Token file mounted into container at a known path
4. **XDG compliance audit**: Fix any hardcoded `~/.claude` paths to use XDG_CONFIG_HOME fallback

## Glossary

- **XDG Base Directory**: Freedesktop.org specification for user directory locations (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`)
- **OAuth token**: Long-lived authentication token for Claude Code Pro/Max subscriptions (vs API keys)
- **containerEnv**: devcontainer.json field that sets environment variables in the container (persisted to disk)

## Architecture

### XDG-Aware Claude Directory Resolution

A new function resolves the user's `.claude` directory:

1. Check `XDG_CONFIG_HOME` environment variable
2. If set, return `$XDG_CONFIG_HOME/claude`
3. If not set, return `$HOME/.claude`

Token file location: `{claudeConfigDir}/.devagent-claude-token`

### Token Auto-Provisioning Flow

```
Container Create Request
        │
        ▼
┌─────────────────────────┐
│ Check for token file    │
│ {claudeConfigDir}/      │
│ .devagent-claude-token  │
└───────────┬─────────────┘
            │
    ┌───────┴───────┐
    │ File exists?  │
    └───────┬───────┘
            │
      ┌─────┴─────┐
      │           │
     Yes          No
      │           │
      ▼           ▼
┌──────────┐  ┌─────────────────┐
│ Read     │  │ Run             │
│ token    │  │ claude setup-token│
└────┬─────┘  └────────┬────────┘
     │                 │
     │           ┌─────┴─────┐
     │           │ Success?  │
     │           └─────┬─────┘
     │           ┌─────┴─────┐
     │          Yes          No
     │           │           │
     │           ▼           ▼
     │      ┌─────────┐  ┌─────────┐
     │      │ Save    │  │ Log     │
     │      │ token   │  │ warning │
     │      └────┬────┘  │ continue│
     │           │       └────┬────┘
     └─────┬─────┘            │
           │                  │
           ▼                  ▼
    ┌────────────┐    ┌────────────┐
    │ Add mount  │    │ No mount   │
    │ to config  │    │ added      │
    └────────────┘    └────────────┘
           │                  │
           └────────┬─────────┘
                    ▼
           ┌────────────────┐
           │ Continue with  │
           │ container build│
           └────────────────┘
```

**Error handling is non-blocking**: If `claude` CLI is missing or `setup-token` fails, log a warning and continue building the container without the token.

### Mount and Shell Profile Injection

**devcontainer.json mount** (when token exists):
```json
{
  "mounts": [
    "source={claudeConfigDir}/.devagent-claude-token,target=/run/secrets/claude-token,type=bind,readonly"
  ]
}
```

**Shell profile snippet** (added to .bashrc and .zshrc in templates):
```bash
# Claude Code OAuth token (mounted by devagent)
if [ -f /run/secrets/claude-token ]; then
    export CLAUDE_CODE_OAUTH_TOKEN="$(cat /run/secrets/claude-token)"
fi
```

### Removal of containerEnv Token Injection

The existing pattern that reads tokens and injects them into `containerEnv["CLAUDE_CODE_OAUTH_TOKEN"]` is removed entirely. This eliminates secrets from devcontainer.json files.

## Existing Patterns Followed

- XDG path resolution pattern from `getConfigDir()` in `internal/config/config.go`
- Mount generation pattern from existing `.claude` directory mounts in `devcontainer.go`
- Template home directory structure from `config/templates/basic/home/vscode/`

## Implementation Phases

### Phase 1: XDG Path Resolution and Token Provisioning
- Add `getClaudeConfigDir()` function with XDG support
- Add `ensureClaudeToken()` function that checks/creates token
- Remove old `readClaudeAuthToken()` function

### Phase 2: Mount Generation
- Modify `Generate()` to add read-only bind mount when token exists
- Remove `CLAUDE_CODE_OAUTH_TOKEN` from `containerEnv` injection

### Phase 3: Shell Profile Templates
- Add `.bashrc` to `config/templates/basic/home/vscode/`
- Add `.zshrc` to `config/templates/basic/home/vscode/`
- Add home directory structure to `go-project` template (may require Dockerfile)

### Phase 4: Testing and Verification
- Test container creation with token present
- Test container creation without token (warning logged, continues)
- Test XDG_CONFIG_HOME override
- Verify Claude Code authentication works inside container

## Additional Considerations

### Security
- Token file is bind-mounted read-only
- Token never written to devcontainer.json
- Shell profile only exports if file exists (graceful degradation)

### Backward Compatibility
- Old `~/.claude/create-auth-token` path is not supported (clean break)
- Existing containers will need to be recreated to use new mechanism
