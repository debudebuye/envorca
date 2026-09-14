# ADR-0003: IPC transport and security

Status: Accepted

## Context

The CLI, the future desktop application, and the daemon must share a strongly
typed, local-only RPC channel. The spec requires: no unnecessary network
server, authenticated local clients, command validation, and least privilege.

## Decision

- Transport is **gRPC over a Windows named pipe** on Windows. The pipe is
  `\\.\pipe\envorka` with an SDDL ACL granting access only to SYSTEM, built-in
  Administrators, and the current interactive user.
- On non-Windows builds (developer machines, CI), the same interface is
  provided over a **Unix domain socket** so the full stack is testable without
  Windows.
- The transport lives in the shared API module (`api/ipc`) as a single
  package owned by both daemon and clients, so endpoint resolution, security,
  and dial/listen semantics have one implementation.
- The daemon exposes **no TCP listener**. `envorka start` and `envorka stop`
  perform a liveness dial before spawning or shutting down the daemon.
- Only member clients of the pipe/socket ACL can reach the daemon. This is a
  single-user-machine model; stronger per-RPC authentication is deferred until
  a concrete multi-session need appears.

## Consequences

- Command construction never uses shell strings; the WSL/Docker bridge passes
  argument slices.
- Clients are thin: they can call only the RPCs the daemon defines.
- Destructive operations carry explicit confirmation in the API contract.

## Non-goals

- No network-facing daemon endpoint.
- No custom authentication protocol in V1; pipe/socket ACLs plus the
  API surface are the trust boundary.