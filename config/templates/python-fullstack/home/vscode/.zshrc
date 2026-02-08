# ~/.zshrc: executed by zsh for interactive shells

# UTF-8 and color support (required for proper terminal rendering)
export LANG=C.UTF-8
export LC_ALL=C.UTF-8
export COLORTERM=truecolor

# Claude Code OAuth token (mounted by devagent)
if [ -f /run/secrets/claude-token ]; then
    export CLAUDE_CODE_OAUTH_TOKEN="$(cat /run/secrets/claude-token)"
fi
