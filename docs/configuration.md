# Configuration

The daemon reads an optional YAML config file. Location: `ENVORKA_CONFIG`,
or none (defaults apply).

## Defaults

| Setting     | Windows                  | Linux/macOS                       |
|-------------|--------------------------|-----------------------------------|
| `state_dir` | `%LOCALAPPDATA%\Envorka` | `$XDG_DATA_HOME/envorka` (`~/.local/share/envorka`) |
| `log_level` | `info`                   | `info`                            |
| `socket`    | `\\.\pipe\envorka`       | `<state_dir>/run/envorka.sock`    |

## Example

```yaml
state_dir: C:\Users\me\AppData\Local\Envorka
log_level: debug
socket: \\.\pipe\envorka
```

## Endpoint overrides

Clients and the daemon resolve the IPC endpoint from the same rule
(`api/ipc`): the `socket` setting if set, then the `ENVORKA_SOCKET`
environment variable, then the platform default. This keeps one endpoint
resolution rule across daemon, CLI, and tests.

## State layout

```
<state_dir>/
  config.yaml       (optional)
  state.db          SQLite: settings, and later projects/diagnostics/repairs
  logs/
    envorka.log     structured JSON daemon log
    daemon.out.log  raw daemon process output while detached-starting
  run/
    envorka.sock    IPC endpoint (Unix); envorka.pid
```