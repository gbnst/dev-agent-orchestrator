# Tsnsrv Domain

Last verified: 2026-02-22

## Purpose
Integrates with tsnsrv to expose the web UI on a Tailscale tailnet. Builds CLI arguments from config and reads the service FQDN from tsnsrv's persisted state.

## Contracts
- **Exposes**: `BuildProcessConfig()`, `BuildProcessConfigWith()`, `ReadServiceURL()`
- **Guarantees**: BuildProcessConfig resolves tsnsrv binary via exec.LookPath. BuildProcessConfigWith is pure (no LookPath, testable). ReadServiceURL returns (fallbackURL, false) on any read/parse error. Process config uses OnFailure restart with 5 retries and 3s delay.
- **Expects**: tsnsrv binary in PATH (for BuildProcessConfig). Valid TailscaleConfig from config package. State directory with tailscaled.state file (for ReadServiceURL).

## Dependencies
- **Uses**: config.TailscaleConfig, config.ResolvePathFunc, process.Config
- **Used by**: main.go (startTsnsrv helper)
- **Boundary**: Config building and state reading only; does not run the process itself

## Key Decisions
- Functional Core / Imperative Shell split: BuildProcessConfigWith is pure, BuildProcessConfig wraps with LookPath
- ReadServiceURL parses base64-encoded tailscale state file format (profiles + current-profile)
- Falls back to `<scheme>://<name>.<tailnet>.ts.net` placeholder when state unavailable
- Supports both direct profile key lookup and Key field matching (tailscale state format quirk)

## Invariants
- ReadServiceURL never errors; returns (fallback, false) on failure
- Plaintext config flag controls both scheme (http vs https) and -plaintext CLI arg
- Upstream URL is always the last positional argument in built args

## Key Files
- `tsnsrv.go` - BuildProcessConfig, BuildProcessConfigWith, ReadServiceURL
