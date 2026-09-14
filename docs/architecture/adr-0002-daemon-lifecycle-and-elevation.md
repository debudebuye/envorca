# ADR-0002: Daemon lifecycle and elevation

Status: Accepted

## Context

The spec requires the daemon to be the authoritative owner of infrastructure
state while also stating the daemon must not run as Administrator. It does not
define how the daemon is launched or how elevation is handled.

## Decision

- The daemon is a **user-session background process**, not a Windows Service.
  It is registered to start at logon via Task Scheduler and is otherwise
  started by `envorca start`.
- The daemon performs **no elevated operations in V1**. WSL invocations,
  Docker operations inside the distribution, file and SQLite access, and the
  named-pipe listener all run in the interactive user context without
  elevation.
- Privileged work (enabling the WSL feature, installing prerequisites) is the
  responsibility of the **installer**, which legitimately runs elevated.
  A `privileged helper` seam is reserved for the first concrete elevated need
  but is not built in V1.

## Consequences

- Installing the daemon as a user-session process keeps the IPC surface local
  and avoids the elevation/IPC complexity of a full Windows service.
- `envorca start`/`envorca stop` manage the daemon lifecycle.
- If a future need (e.g. firewall rules, distribution installation) requires
  elevation, it is added as a narrow privileged helper with its own IPC
  boundary, not by elevating the whole daemon.

## Non-goals

- No elevated runtime operations in V1.
- No automatic `.wslconfig` or system-level configuration in V1.