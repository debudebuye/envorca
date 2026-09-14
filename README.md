# ENVORCA

Envorca makes Linux development on Windows just work.

Envorca manages the WSL2, Linux, and container infrastructure behind Windows
development workloads so the developer can focus on their project, not on
virtualization internals. It detects problems, explains them, and recovers
from them safely.

## Status

**M1 — Foundation (in progress)**

- Go daemon (`daemon/`) with gRPC API, streaming events, structured logging,
  SQLite state, and a user-session lifecycle.
- Local IPC over Windows named pipes (`api/ipc`); Unix domain sockets are used
  on non-Windows builds for development and testing.
- Thin CLI (`cli/`): `envorca status`, `envorca start`, `envorca stop`.

Goal of M1: `envorca status` round-trips through the daemon.

## Repository layout

```
api/       shared API module: proto contract + generated code + IPC transport
daemon/    the Envorca daemon (the only owner of infrastructure logic)
cli/       thin command-line client
scripts/   proto generation, dev and integration helpers
docs/      architecture decisions (ADRs)
```

The CLI and the future desktop UI must not contain infrastructure logic. The
daemon is authoritative; clients are thin.

## Build

Requires Go 1.27+ and, for proto regeneration, protoc 3.6+ with
`protoc-gen-go` and `protoc-gen-go-grpc`.

```sh
go work sync            # resolve workspace module versions
./scripts/gen-proto.sh  # regenerate gRPC code from api/proto (CI-verified)
go build ./...          # everything
```

## Test

```sh
go test ./...           # from the workspace root or any module
./scripts/integration/status-roundtrip.sh   # daemon<->CLI round trip
```

The daemon and CLI cross-compile for `windows/amd64`:

```sh
cd daemon && GOOS=windows GOARCH=amd64 go build ./cmd/...
cd cli    && GOOS=windows GOARCH=amd64 go build ./cmd/...
```

## Quick start (dev machines)

On Windows:

```sh
envorca start     # launch the daemon (autostarts via Task Scheduler)
envorca status    # report daemon and environment status
envorca stop      # gracefully shut the daemon down
```

On Linux development machines the same commands run against a Unix-socket
transport so the full stack is testable without Windows.

## State and configuration

- State: `%LOCALAPPDATA%\Envorca` on Windows,
  `$XDG_DATA_HOME/envorca` (default `~/.local/share/envorca`) elsewhere.
- Optional YAML config overrides defaults; see `docs/configuration.md`.

## Design

Architectural decisions are recorded in `docs/architecture/` as ADRs. Start
with:

- `adr-0001-container-bridge.md` — how the daemon reaches containers in WSL
- `adr-0002-daemon-lifecycle-and-elevation.md` — user-session daemon
- `adr-0003-ipc-transport-security.md` — named-pipe transport and auth