# whport

> ⚠️ **Deprecated — no longer actively maintained.**
>
> Port and process workflows are moving to [`pstop`](https://github.com/lu-zhengda/pstop):
>
> | whport workflow | pstop equivalent | Status |
> |-----------------|-----------------|--------|
> | `whport list` | `pstop list` (all processes, no port-first filter) | Partial — PID-centric, not port-centric |
> | `whport info <port>` | `pstop find <name>` → `pstop info <pid>` (shows ports) | Partial — requires PID lookup first |
> | `whport kill <port>` | Port-first kill by port number | **Pending** — not yet in pstop |
> | `whport watch` | `pstop watch <pid>` or TUI | Partial |
>
> `whport` continues to work as-is on supported macOS versions, but no bug fixes or
> feature updates are planned. For port-first workflows, `whport` remains the current
> option until pstop gains native port-kill support.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Platform: macOS](https://img.shields.io/badge/Platform-macOS-lightgrey.svg)](https://github.com/lu-zhengda/whport)
[![Homebrew](https://img.shields.io/badge/Homebrew-lu--zhengda/tap-orange.svg)](https://github.com/lu-zhengda/homebrew-tap)

Port & process manager for macOS — find what's listening, kill by port, and monitor changes live.

## Install

```bash
brew tap lu-zhengda/tap
brew install whport
```

## Usage

```
$ whport list
PORT   PROTO  PID    PROCESS    USER    STATE
5000   TCP    644    ControlCe  user    LISTEN
7000   TCP    644    ControlCe  user    LISTEN
7265   TCP    76742  Raycast    user    LISTEN
26443  TCP    87971  OrbStack   user    LISTEN

$ whport info 5000
Port:        5000/TCP
State:       LISTEN
Process:     ControlCe (PID 644)
Command:     /System/Library/CoreServices/ControlCenter.app/Contents/MacOS/ControlCenter
User:        user
CPU:         0.0%
Memory:      116.0 MB (RSS)
Parent PID:  1
```

## Commands

| Command | Description | Example |
|---------|-------------|---------|
| `list` | List all listening ports | `whport list` |
| `list --port <n>` | Filter by port number | `whport list --port 3000` |
| `list --process <name>` | Filter by process name | `whport list --process node` |
| `list --protocol <tcp\|udp>` | Filter by protocol | `whport list --protocol tcp` |
| `list --all` | Include ESTABLISHED connections | `whport list --all` |
| `info <port>` | Detailed process info (PID, CPU, memory, children) | `whport info 8080` |
| `kill <port>` | Kill process on port (SIGTERM) | `whport kill 3000` |
| `kill <port> --force` | Force kill (SIGKILL) | `whport kill 3000 --force` |
| `kill <port> --signal <sig>` | Custom signal | `whport kill 3000 --signal SIGHUP` |
| `watch` | Live auto-refresh port table | `whport watch --interval 5` |

All commands support `--json` for machine-readable output.

## TUI

Launch `whport` without arguments for an interactive port dashboard. Browse listening ports, filter by process or protocol, and kill processes with a keyboard-driven interface.

## Safety

- **Always `info` before `kill`** — check what owns the port before terminating
- **Default is SIGTERM** — allows graceful shutdown; use `--force` (SIGKILL) only as a last resort
- **Verify after kill** — run `whport list --port <n>` to confirm the port is freed

## Claude Code

Available as a skill in the [macos-toolkit](https://github.com/lu-zhengda/macos-toolkit) Claude Code plugin. Ask Claude "what's on port 8080?" and it runs whport automatically.

## License

[MIT](LICENSE)
