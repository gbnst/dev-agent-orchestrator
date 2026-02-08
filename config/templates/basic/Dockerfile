FROM mcr.microsoft.com/devcontainers/base:ubuntu

# Install tmux for terminal session management
RUN apt-get update && apt-get install -y tmux && rm -rf /var/lib/apt/lists/*

# Install Claude Code for vscode user (not root)
USER vscode
RUN curl -fsSL https://claude.ai/install.sh | bash
USER root
