# Configuration

The daemon reads an optional YAML config file. Location: `RUNORKA_CONFIG`,
or none (defaults apply).

## Defaults

| Setting     | Windows                  | Linux/macOS                       |
|-------------|--------------------------|-----------------------------------|
| `state_dir` | `%LOCALAPPDATA%\Runorka` | `$XDG_DATA_HOME/runorka` (`~/.local/share/runorka`) |
| `log_level` | `info`                   | `info`                            |
| `socket`    | `\\.\pipe\runorka`       | `<state_dir>/run/runorka.sock`    |

## Example

```yaml
state_dir: C:\Users\me\AppData\Local\Runorka
log_level: debug
socket: \\.\pipe\runorka
```

## Endpoint overrides

Clients and the daemon resolve the IPC endpoint from the same rule
(`api/ipc`): the `socket` setting if set, then the `RUNORKA_SOCKET`
environment variable, then the platform default. This keeps one endpoint
resolution rule across daemon, CLI, and tests.

## State layout

```
<state_dir>/
  config.yaml       (optional)
  state.db          SQLite: settings, and later projects/diagnostics/repairs
  logs/
    runorka.log     structured JSON daemon log
    daemon.out.log  raw daemon process output while detached-starting
  run/
    runorka.sock    IPC endpoint (Unix); runorka.pid
```