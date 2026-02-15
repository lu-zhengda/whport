# whport

Port & process manager for macOS — see what's listening, kill by port, and monitor changes live.

## Install

```bash
brew tap lu-zhengda/tap
brew install whport
```

## Quick Start

```bash
whport           # Launch interactive TUI
whport --help    # Show all commands
```

## Commands

| Command | Description                              |
|---------|------------------------------------------|
| `list`  | List all listening ports                 |
| `kill`  | Kill a process by port                   |
| `info`  | Get detailed port information            |
| `watch` | Watch port changes in real-time          |

## TUI

Launch without arguments for interactive mode. Browse listening ports, filter by process or protocol, and kill processes with a keyboard-driven interface.

<!-- Screenshot placeholder: ![whport TUI](docs/tui.png) -->

## License

MIT
