FROM mcr.microsoft.com/devcontainers/base:ubuntu

# Install Claude Code for vscode user (not root)
USER vscode
RUN curl -fsSL https://claude.ai/install.sh | bash
USER root
