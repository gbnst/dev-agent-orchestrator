FROM mcr.microsoft.com/devcontainers/base:ubuntu

# Install tmux for terminal session management
RUN apt-get update && apt-get install -y tmux && rm -rf /var/lib/apt/lists/*

# Copy home directory dotfiles
COPY --chown=vscode:vscode home/vscode/ /home/vscode/

# Install Claude Code for vscode user (not root)
USER vscode
RUN curl -fsSL https://claude.ai/install.sh | bash
USER root
