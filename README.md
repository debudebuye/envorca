# ENVORCA

Envorca makes Linux development on Windows just work.

Envorca manages the WSL2, Linux, and container infrastructure behind Windows
development workloads so the developer can focus on their project, not on
virtualization internals. It detects problems, explains them, and recovers
from them safely.

## Status

**M2 — Doctor & Repair (complete)**

- Go daemon (`daemon/`) with gRPC API, streaming events, structured logging,
  SQLite state, and a user-session lifecycle.
- Local IPC over Windows named pipes (`api/ipc`); Unix domain sockets are used
  on non-Windows builds for development and testing.
- Thin CLI (`cli/`): full command set for diagnosis, repair, and observability.
- Seven health probes: daemon, Windows version, virtualization, WSL, kernel,
  memory, container runtime.
- Recovery actions: `wsl.start` (safe) and `wsl.restart` (requires confirmation).

## CLI commands

| Command | Description |
|---------|-------------|
| `envorca start [--config X]` | Start the Envorca daemon (detached background process) |
| `envorca stop` | Gracefully shut the daemon down |
| `envorca status` | Daemon state, overall health, and per-component summary |
| `envorca doctor` | Detailed per-component diagnosis: what's wrong, why, and how to fix it |
| `envorca repair [--yes]` | Propose and apply safe, deterministic repair actions |
| `envorca events [--replay]` | Stream daemon events (startup, repairs, lifecycle) |
| `envorca history [--limit N]` | View recorded diagnostic snapshots and repair outcomes |
| `envorca ping` | Check daemon liveness and version |

A root-level `--config` flag is available on all commands. When provided,
the CLI reads the config file's `socket` setting to locate the daemon,
honouring the same endpoint resolution rule as the daemon itself (see
`docs/configuration.md`).

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
# Build both binaries (Linux) with ldflags version injection.
make build

# Build for the current platform.
make build VERSION=0.2.0

# Cross-compile Windows binaries.
make cross

# Run all tests.
make test

# Run go vet.
make vet
```

Or directly:

```sh
go work sync            # resolve workspace module versions
go build -C daemon ./cmd/envorcad
go build -C cli ./cmd/envorca
```

## Test

```sh
make test               # all unit tests
./scripts/integration/status-roundtrip.sh   # daemon<->CLI round trip
./scripts/integration/faults.sh             # fault injection + repair cycle
```

The daemon and CLI cross-compile for `windows/amd64`:

```sh
make cross
```

## Quick start (Windows)

```sh
envorca start                    # launch the daemon (detached background process)
envorca status                   # report daemon and environment status
envorca doctor                   # detailed per-component diagnosis with recommendations
envorca repair                   # apply safe fixes (prompts before destructive actions)
envorca stop                     # gracefully shut the daemon down
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

## Versioning

Daemon and CLI versions are injected at build time via `-ldflags`. Development
builds report `0.1.0-dev`; tagged releases pin the exact version. Use:

```sh
make build VERSION=1.0.0
```
