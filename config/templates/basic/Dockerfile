FROM mcr.microsoft.com/devcontainers/base:ubuntu

# Install tmux for terminal session management
RUN apt-get update && apt-get install -y tmux && rm -rf /var/lib/apt/lists/*

# Install Claude Code and make it available system-wide.
# Copy to /usr/local/bin/ so it's available regardless of home directory mount configuration.
USER vscode
RUN curl -fsSL https://claude.ai/install.sh | bash
USER root
RUN cp /home/vscode/.local/bin/claude /usr/local/bin/claude
USER vscode
