FROM mcr.microsoft.com/devcontainers/base:ubuntu

# Install tmux for terminal session management
RUN apt-get update && apt-get install -y tmux && rm -rf /var/lib/apt/lists/*

# Install Claude Code and make it available system-wide.
# The installer places the binary in ~/.claude/bin/ which gets shadowed
# by the /home/vscode volume mount at runtime, so copy it to /usr/local/bin/.
USER vscode
RUN curl -fsSL https://claude.ai/install.sh | bash
USER root
RUN cp /home/vscode/.claude/bin/claude /usr/local/bin/claude
