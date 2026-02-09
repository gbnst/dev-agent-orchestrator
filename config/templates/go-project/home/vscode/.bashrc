# ~/.bashrc: executed by bash for non-login shells

# Claude Code OAuth token (mounted by devagent)
if [ -f /run/secrets/claude-token ]; then
    export CLAUDE_CODE_OAUTH_TOKEN="$(cat /run/secrets/claude-token)"
fi

# GitHub CLI token (mounted by devagent)
if [ -f /run/secrets/github-token ] && [ -s /run/secrets/github-token ]; then
    export GH_TOKEN="$(cat /run/secrets/github-token)"
fi
