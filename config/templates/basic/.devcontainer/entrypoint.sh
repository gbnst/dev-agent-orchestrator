#!/bin/sh
# Install mitmproxy CA certificate into the system trust store.
# This runs as the container entrypoint, BEFORE VS Code connects,
# ensuring TLS interception works for VS Code Server downloads.
#
# The proxy healthcheck (service_healthy) guarantees the cert file
# exists before the app container starts, so this wait loop is a
# belt-and-suspenders fallback with a short timeout.

CERT_SRC="/tmp/mitmproxy-certs/mitmproxy-ca-cert.pem"
CERT_DST="/usr/local/share/ca-certificates/mitmproxy-ca-cert.crt"

timeout=10
while [ ! -f "$CERT_SRC" ] && [ "$timeout" -gt 0 ]; do
    sleep 1
    timeout=$((timeout - 1))
done

if [ -f "$CERT_SRC" ]; then
    cp "$CERT_SRC" "$CERT_DST"
    update-ca-certificates
fi

exec "$@"
