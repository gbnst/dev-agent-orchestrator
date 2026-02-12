#!/bin/bash
set -e

# ============================================================
# mitmproxy CA certificate installation
# ============================================================
timeout=30
while [ ! -f /tmp/mitmproxy-certs/mitmproxy-ca-cert.pem ] && [ $timeout -gt 0 ]; do
    sleep 1
    timeout=$((timeout-1))
done
if [ -f /tmp/mitmproxy-certs/mitmproxy-ca-cert.pem ]; then
    sudo cp /tmp/mitmproxy-certs/mitmproxy-ca-cert.pem /usr/local/share/ca-certificates/mitmproxy-ca-cert.crt
    sudo update-ca-certificates
fi

# ============================================================
# Project-specific setup
# Add project initialization commands below (e.g., dependency
# install, database migrations, build steps).
# ============================================================
uv sync || true
