<!-- banner: place an orca/banner image here (see assets/banner.png) -->
<!-- <p align="center"><img src="assets/banner.png" alt="Envorca — Linux development on Windows just work" width="700"></p> -->

<p align="center">
  <strong>Envorca makes Linux development on Windows just work.</strong>
</p>

---

Envorca is a local infrastructure daemon that manages WSL2, Linux, and Docker containers behind Windows development workflows. It watches your dev environment, explains what is broken in plain language, and fixes it safely — so you can focus on shipping code instead of debugging virtualization.

**Built for developers who use WSL2 + Docker on Windows every day.**

---

## Why Envorca

WSL2 is powerful but fragile. Distributions silently die. The Docker daemon stops answering. Virtualization breaks after a Windows update. When things go wrong, the errors are cryptic and the fixes are scattered across forum threads.

Envorca solves this with one command:

```
$ envorca doctor

ENVORCA DOCTOR

  COMPONENT              STATUS    NOTES
  daemon                 HEALTHY   daemon running
  windows                HEALTHY   Windows 10.0 (build 22631)
  virtualization         HEALTHY   CPU virtualization and Virtual Machine Platform available
  wsl                    HEALTHY   WSL2 ready; default distro Ubuntu-24.04 (running)
  linux_kernel           HEALTHY   6.6.13.1-2
  resources              HEALTHY   memory: 31.8 GB installed, 14.2 GB available
  container_runtime      HEALTHY   Docker ready (server 27.5.1); 3 running, 5 total

  Overall: HEALTHY
  Summary: 0 problems detected — the environment is healthy.
```

When something **is** broken, it tells you *what* went wrong, *why* it matters, and *how* to fix it — and when it can fix it safely, it offers to do so:

```
$ envorca doctor

  wsl                    CRITICAL  Linux environment failed to start
        Why: Containers cannot start while the WSL environment is broken.
        Recommend: Restart the WSL environment.
  wsl: Restart the WSL environment.
  Envorca can fix this. Run 'envorca repair' to apply the repair.
```

```
$ envorca repair

Envorca can fix 1 issue(s):

  wsl.start: Start the WSL environment
        Boots the default Linux distribution. Does not affect user data.

Apply these repairs now? [y/N]: y
  wsl.start: ok (verified)

All repairs applied. Run 'envorca doctor' to confirm.
```

---

## What Envorca does

| Capability | How |
|---|---|
| **Detects** problems across your entire WSL2/Docker stack | Seven health probes covering: daemon health, Windows OS, virtualization firmware + services, WSL lifecycle, Linux kernel version, system memory, and Docker container runtime |
| **Explains** problems in plain language | Every probe returns: what is wrong, why it matters, and Envorca's recommendation |
| **Fixes** problems safely | Recovery actions follow a `diagnose → propose → confirm → execute → verify` flow; destructive actions always require explicit user confirmation |
| **Remembers** environment history | Diagnostic snapshots and repair outcomes are recorded in a local SQLite database with bounded retention (200 diagnostics, 1000 repairs) |
| **Streams events** in real time | A publish/subscribe bus lets the CLI (and future UIs) stream daemon events as they happen, with a replay buffer for missed events |

---

## Health probes

| Component | What it checks |
|---|---|
| `daemon` | Envorca daemon is serving requests |
| `windows` | OS version is readable (`RtlGetVersion` on Windows) |
| `virtualization` | CPU virtualization firmware (VT-x/AMD-V), SLAT, `vmcompute` and `WslService` services |
| `wsl` | WSL is installed, default distribution exists, distribution boots successfully |
| `linux_kernel` | WSL kernel version ≥ 5.10 |
| `resources` | Installed and available system memory |
| `container_runtime` | Docker CLI installed in WSL, Docker daemon answering, running/total container count |

---

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
| `envorca version` | Print CLI version |

A root-level `--config` flag is available on all commands. When provided, the CLI reads the config file's `socket` setting to locate the daemon, honoring the same endpoint resolution rule as the daemon itself (see `docs/configuration.md`).

---

## Quick start

### Install

Build from source (requires Go 1.27+):

```sh
git clone https://github.com/debudebuye/envorca.git
cd envorca
make build          # builds bin/envorcad + bin/envorca
```

Or cross-compile Windows binaries from any platform:

```sh
make cross
```

### Use

```sh
export PATH="$PWD/bin:$PATH"

envorca start        # launch the daemon in the background
envorca doctor       # see what's happening with your environment
envorca repair       # apply safe fixes
envorca status       # quick health overview
envorca stop         # shut down the daemon when done
```

---

## How it works

Envorca follows a **daemon + thin client** architecture:

```
CLI (envorca)          Daemon (envorcad)
    |                        |
    |--- gRPC over local IPC --->
    |    (named pipe /         |
    |     Unix socket)         |
    |                    WSL2 / Docker CLI
    |                        |
    |                    SQLite state
```

- **Daemon** (`daemon/`) — the single authoritative owner of all infrastructure logic. Never runs as Administrator in V1. Communicates with WSL2 via `wsl.exe` argument slices (never shell strings). All container access goes through a `RuntimeDriver` interface (ADR-0001).
- **CLI** (`cli/`) — a thin Cobra client that dials the daemon over local IPC and renders responses. Contains zero infrastructure logic.
- **API** (`api/`) — the shared gRPC contract (protobuf) and IPC transport (Windows named pipe with SDDL ACL, or Unix domain socket for dev/test).

### IPC security

- No TCP listener, no TLS — the daemon only listens on a local IPC endpoint.
- Windows: named pipe `\\.\pipe\envorca` with an SDDL ACL granting access only to SYSTEM, Administrators, and the current interactive user.
- Unix: domain socket with filesystem permission boundary.
- Destructive operations (`wsl.restart`) require explicit `confirmed=true` in the API.

---

## Repository layout

```
api/           shared API module: protobuf contract + gRPC code + IPC transport
daemon/        the Envorca daemon (envorcad) — sole owner of infrastructure logic
cli/           thin command-line client (envorca)
scripts/       proto generation, dev and integration helpers
docs/          configuration guide + architecture decision records (ADRs)
.github/       CI workflows (build, test, Windows integration)
```

### Architecture decision records

| ADR | Decision |
|-----|----------|
| [ADR-0001](docs/architecture/adr-0001-container-bridge.md) | Container bridge — `wsl.exe` + Docker CLI inside the distribution, behind a `RuntimeDriver` seam |
| [ADR-0002](docs/architecture/adr-0002-daemon-lifecycle-and-elevation.md) | User-session daemon, not a Windows Service; no elevated operations in V1 |
| [ADR-0003](docs/architecture/adr-0003-ipc-transport-security.md) | Named pipe / Unix socket transport; pipe ACL is the trust boundary |

---

## Configuration

Envorca uses an optional YAML config file. When absent, platform defaults apply.

| Setting | Windows default | Linux/macOS default |
|---------|----------------|-------------------|
| `state_dir` | `%LOCALAPPDATA%\Envorca` | `~/.local/share/envorca` |
| `log_level` | `info` | `info` |
| `socket` | `\\.\pipe\envorca` | `<state_dir>/run/envorca.sock` |

```yaml
# Example config.yaml
state_dir: C:\Users\me\AppData\Local\Envorca
log_level: debug
socket: \\.\pipe\envorca
```

Override with `--config path/to/config.yaml` or `ENVORCA_CONFIG=path/to/config.yaml`. See [`docs/configuration.md`](docs/configuration.md) for the full reference.

---

## Build and test

```sh
make build            # build both binaries
make build VERSION=1.0.0  # with explicit version
make cross            # cross-compile Windows/amd64 binaries
make test             # all unit tests
make vet              # go vet across all modules
make proto            # regenerate gRPC code (requires protoc)
make install          # install to ~/go/bin
```

Integration tests:

```sh
./scripts/integration/status-roundtrip.sh   # daemon ↔ CLI round trip
./scripts/integration/faults.sh             # fault injection + repair cycle (Windows)
```

---

## Roadmap

- [ ] **M1 — Foundation** ✅ daemon, CLI, gRPC API, IPC, events, state, logging
- [ ] **M2 — Doctor & Repair** ✅ health probes, recovery actions, history, diagnostics
- [ ] **M3 — Production Readiness** ✅ CI, tests, build tooling, documentation
- [ ] **M4 — Desktop UI** TUI or native app using the same gRPC API
- [ ] **M5 — Advanced Repair** Docker daemon restart, `wsl --update`, more recovery actions
- [ ] **M6 — Multi-project** Per-project health context and container awareness

---

## Contributing

Envorca is an open-source project and contributions are welcome. See [`CONTRIBUTING.md`](CONTRIBUTING.md) for how to get started.

### Development setup

```sh
git clone https://github.com/debudebuye/envorca.git
cd envorca
make test             # verify everything passes
```

The project is a Go workspace with three modules (`api`, `daemon`, `cli`). Run `go work sync` after pulling. On Linux, the full stack runs over Unix sockets and is testable without Windows.

### Reporting issues

Open an issue at https://github.com/debudebuye/envorca/issues. Include:
- What you expected to happen
- What actually happened
- Output of `envorca status` and `envorca doctor`
- OS version and WSL distribution

---

## License

Licensed under the [Apache License, Version 2.0](LICENSE).

Copyright 2024–2026 [debudebuye](https://github.com/debudebuye).
