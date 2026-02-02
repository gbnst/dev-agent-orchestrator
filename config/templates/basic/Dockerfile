FROM mcr.microsoft.com/devcontainers/base:ubuntu

# Copy home directory dotfiles
COPY --chown=vscode:vscode home/vscode/ /home/vscode/

# Install Claude Code for vscode user (not root)
USER vscode
RUN curl -fsSL https://claude.ai/install.sh | bash
USER root
