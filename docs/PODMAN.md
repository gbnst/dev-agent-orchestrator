# Podman Compatibility

When using Podman with docker-compose files, note the following:

## Known Issue: devcontainer CLI Bug #863

The devcontainer CLI has a known unfixed bug when using Podman with
`dockerComposeFile` property. Workarounds:

1. **Pin devcontainer CLI version**
   ```bash
   npm install -g @devcontainers/cli@0.58.0
   ```

2. **Use docker-compose binary with Podman backend**
   Configure Podman to work with the standalone docker-compose:
   ```bash
   # Enable Podman socket
   systemctl --user enable --now podman.socket

   # Set DOCKER_HOST to use Podman socket
   export DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock
   ```

## Testing with Podman

Run E2E tests specifically for Podman:
```bash
make test-e2e-podman
```
