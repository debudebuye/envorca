# Configuration

The daemon reads an optional YAML config file. Location: `ENVORCA_CONFIG`,
or none (defaults apply).

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

## State layout

```
<state_dir>/
  config.yaml       (optional)
  state.db          SQLite: settings, and later projects/diagnostics/repairs
  logs/
    envorca.log     structured JSON daemon log
    daemon.out.log  raw daemon process output while detached-starting
  run/
    envorca.sock    IPC endpoint (Unix); envorca.pid
```