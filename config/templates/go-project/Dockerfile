FROM mcr.microsoft.com/devcontainers/go:1.25

# Install Claude Code and make it available system-wide.
# Copy to /usr/local/bin/ so it's available regardless of home directory mount configuration.
USER vscode
RUN curl -fsSL https://claude.ai/install.sh | bash
USER root
RUN cp /home/vscode/.local/bin/claude /usr/local/bin/claude
USER vscode
