# ADR-0001: Container bridge for V1

Status: Accepted

## Context

The daemon is a Windows process. Development containers run inside a WSL2
distribution. Every container operation (list, start, stop, logs, exec,
compose) must cross the Windows/Linux boundary, and that mechanism was not
defined by the product spec.

## Decision

For V1 the daemon reaches containers by invoking `wsl.exe` as a subprocess
with argument slices (never shell strings) and parses Docker CLI JSON output.
All container access goes through a `RuntimeDriver` interface; the only V1
implementation shells out to the Docker CLI inside the selected distribution.

The runtime driver is explicitly **not** a container runtime. V1 uses Docker
CLI (dockerd, which already wraps containerd) for maximum compatibility with
existing `docker build` / `docker run` / `docker compose` workflows.

## Consequences

- No in-distribution agent daemon to build, version, or secure in V1.
- Per-call overhead of spawning `wsl.exe` (~hundreds of ms) is acceptable for
  interactive developer tooling.
- The `RuntimeDriver` seam allows a future in-distribution agent to replace
  the passthrough without changing daemon callers.
- `wsl.exe` output is UTF-16 on some Windows builds; all parsing goes through
  a decoding helper and `wsl --output` (UTF-8) is preferred.

## Non-goals

- No custom container runtime, filesystem, or networking stack in V1.
- Raw containerd integration deferred until there is a concrete V1 need that
  the Docker driver cannot satisfy.