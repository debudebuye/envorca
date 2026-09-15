# Contributing to Envorca

Thanks for your interest in contributing to Envorca! This document explains how to get started.

## Getting started

### Prerequisites

- **Go 1.27+** (`go version` to check)
- **protoc 3.6+** (only needed if modifying `.proto` files)
- **WSL2** with a default Linux distribution (for end-to-end testing on Windows)

### Clone and build

```sh
git clone https://github.com/debudebuye/envorca.git
cd envorca
go work sync
make test
```

### Project structure

```
api/           protobuf contract, generated gRPC code, IPC transport (named pipe / Unix socket)
daemon/        the daemon (envorcad) — sole owner of infrastructure logic
cli/           thin CLI client (envorca) — no infrastructure logic allowed here
docs/          ADRs and configuration docs
scripts/       proto generation, integration tests
```

The core design principle: **the daemon is authoritative; clients are thin.** Any infrastructure logic (WSL probing, container operations, health checks) lives in `daemon/internal/` and is exposed only through the gRPC API in `api/`.

## Development workflow

### Making changes

1. Create a feature branch from `main`.
2. Make your changes. Follow existing code conventions.
3. Run `make test` and `make vet` before committing.
4. If you changed `api/proto/envorca/v1/envorca.proto`, run `make proto` to regenerate code.
5. Open a pull request against `main`.

### Code conventions

- **No infrastructure logic in the CLI.** The CLI calls the daemon API and renders output. That is all.
- **No shell strings for WSL/Docker calls.** All subprocess invocations use argument slices (`exec.CommandContext` with explicit args), never `sh -c "..."`.
- **Every health probe is a `health.Probe` function** that returns `ComponentStatus` with: Summary, Reason, Recommendation, SafeToFix.
- **Recovery actions implement the `recovery.Action` interface.** They must be idempotent and verifiable. Destructive actions set `RequiresConfirmation() = true`.
- **Use `slog` for logging.** Structured JSON to the log file, `log/slog` for everything.
- **Use `modernc.org/sqlite`** (pure Go, no CGO). Migrations live in `daemon/internal/state/migrations/`.
- **IPC uses argument slices, never shell strings.** The `wsl.Runner` type and `runtime.Driver` interface are the seams for testing.

### Writing tests

- Unit tests use fake runners (`wsl.Runner`) that return canned output.
- `daemon/internal/api/daemon_test.go` starts a real in-process gRPC server for end-to-end RPC testing.
- `cli/internal/client/client_test.go` uses a `fakeServer` implementation of the gRPC service.
- Integration tests (`scripts/integration/`) build real binaries and test the daemon ↔ CLI round trip.

### Running specific tests

```sh
go -C daemon test ./internal/api/...       # daemon API tests
go -C cli test ./cmd/envorca/...            # CLI render tests
go -C daemon test ./internal/diagnosis/...  # health probe tests
```

## Submitting changes

### Pull requests

- Keep PRs focused: one logical change per PR.
- Include a clear description of what changed and why.
- All CI checks must pass (build, test, vet, proto regeneration check).
- Update documentation if adding/changing user-facing behavior.
- Update ADRs for architectural changes.

### Commit messages

Use clear, descriptive commit messages. The project uses the style:

```
<type>: <short summary>
```

Types: `feat`, `fix`, `docs`, `test`, `chore`, `refactor`, `ci`.

Examples:
- `feat: add envorca ping command`
- `fix: handle empty WSL distro list gracefully`
- `docs: update README with contributing guidelines`

### Reporting bugs

Open an issue at https://github.com/debudebuye/envorca/issues with:

1. **What you expected to happen**
2. **What actually happened**
3. **Steps to reproduce**
4. **Output of `envorca status` and `envorca doctor`**
5. **OS version** (`ver` on Windows) and **WSL distribution** (`wsl --list --verbose`)

### Suggesting features

Open an issue with the `enhancement` label. Describe:
- The problem you're trying to solve
- Your proposed solution
- Any alternatives you considered

## Architecture questions

Architectural decisions are documented as ADRs in `docs/architecture/`. Before proposing a significant change, read the existing ADRs and check if your idea conflicts with one. If it does, open an issue to discuss updating the ADR.

## License

By contributing to Envorca, you agree that your contributions will be licensed under the [Apache License, Version 2.0](LICENSE).
