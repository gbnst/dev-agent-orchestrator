FROM mcr.microsoft.com/devcontainers/python:3.12-bookworm

# Install Node.js 22 LTS
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Install uv package manager
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

# Install tmux for terminal session management
RUN apt-get update && apt-get install -y --no-install-recommends \
    make \
    tmux \
    && rm -rf /var/lib/apt/lists/*

# Match container vscode user to host uid so bind mounts are read/write
ARG HOST_UID=501
RUN usermod -u ${HOST_UID} vscode

# Install Claude Code and make it available system-wide.
# Copy to /usr/local/bin/ so it's available regardless of home directory mount configuration.
USER vscode
RUN curl -fsSL https://claude.ai/install.sh | bash
USER root
RUN cp /home/vscode/.local/bin/claude /usr/local/bin/claude
USER vscode
