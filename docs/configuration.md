# Configuration

The daemon reads an optional YAML config file. Location: `ENVORCA_CONFIG`,
or none (defaults apply). The `--config` flag on any CLI command reads the
same file to locate the daemon's IPC endpoint.

## Defaults

| Setting     | Windows                  | Linux/macOS                       |
|-------------|--------------------------|-----------------------------------|
| `state_dir` | `%LOCALAPPDATA%\Envorca` | `$XDG_DATA_HOME/envorca` (`~/.local/share/envorca`) |
| `log_level` | `info`                   | `info`                            |
| `socket`    | `\\.\pipe\envorca`       | `<state_dir>/run/envorca.sock`    |

## Example

```yaml
state_dir: C:\Users\me\AppData\Local\Envorca
log_level: debug
socket: \\.\pipe\envorca
```

## Endpoint overrides

Clients and the daemon resolve the IPC endpoint from the same rule
(`api/ipc`): the `socket` setting if set, then the `ENVORCA_SOCKET`
environment variable, then the platform default. This keeps one endpoint
resolution rule across daemon, CLI, and tests.

When a config file is present (via `ENVORCA_CONFIG` or `--config`), the CLI
reads its `socket` field to locate the daemon, so a custom socket set in the
config is respected by both daemon and clients.

## State layout

```
<state_dir>/
  config.yaml       (optional)
  state.db          SQLite: settings, diagnostics history, repair history
  logs/
    envorca.log     structured JSON daemon log
    daemon.out.log  raw daemon process output while detached-starting
  run/
    envorca.sock    IPC endpoint (Unix); envorca.pid
```

### History tables

Two history tables are recorded automatically:

- **`diagnostics`** — status snapshots recorded on every `GetStatus` call
  (capped at 200 most recent).
- **`repair_history`** — outcomes of every repair action (capped at 1,000
  most recent).

Records are pruned on insert so the database cannot grow without bound.
