FROM mcr.microsoft.com/devcontainers/go:1.25

# Copy home directory dotfiles
COPY --chown=vscode:vscode home/vscode/ /home/vscode/

# Install Claude Code for vscode user (not root)
USER vscode
RUN curl -fsSL https://claude.ai/install.sh | bash
USER root
