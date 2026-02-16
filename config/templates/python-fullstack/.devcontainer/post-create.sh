#!/bin/bash
set -e

# ============================================================
# Project-specific setup
# Add project initialization commands below (e.g., dependency
# install, database migrations, build steps).
# ============================================================
uv sync || true
