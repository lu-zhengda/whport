# whport

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
PORT   PID    PROCESS        USER      PROTOCOL
22     1      launchd        root      tcp
80     421    httpd          _www      tcp
3000   12345  node           zhengda   tcp
5432   8901   postgres       zhengda   tcp
8080   23456  java           zhengda   tcp

$ whport info 3000
Port:     3000/tcp
Process:  node (PID 12345)
User:     zhengda
CPU:      2.1%
Memory:   148.3 MB
Command:  node server.js
Children: 3 worker processes

$ whport kill 3000
Killed process node (PID 12345) on port 3000
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

## License

[MIT](LICENSE)
