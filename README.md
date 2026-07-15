# zhh — Remote Shell & File Transfer CLI

`zhh` lets you control remote machines from your terminal — like SSH, but with
multi-device awareness, cross-machine pipelines, and built-in file transfer.

## Quick Start

```bash
# On the machine to be controlled (beta mode is default)
zhh

# On the controller machine (alpha mode)
zhh alpha

# Connect to a specific beta by its last IP octet (mDNS discovery)
zhh alpha 42

# Connect to a specific beta directly by its IP address
zhh alpha 192.168.1.50

# Run a single command and exit
zhh a 42 - rmdir /s /q \mydir

# Run alpha and beta on the same machine (development / testing)
zhh twin

# One-off command via twin mode
zhh twin - ls -la
```

## How It Works

**Beta** (default mode) listens for incoming connections and advertises itself
on the network via mDNS. **Alpha** (controller) discovers betas automatically
and gives you an interactive shell.

**Twin mode** starts a beta server in the background and immediately connects
an alpha client to `127.0.0.1`. It's the same protocol as a real remote session
but runs locally — useful for testing, scripting, or when no other machines
are available.

## Features

- **Interactive Shell Editor:** Built on `readline` with full support for cursor navigation (left/right arrows), command history (up/down arrows), and persistent shell history.
- **Graceful Interrupts:** Hitting `Ctrl+C` on an empty prompt pops the active shell or disconnects the session (exactly like `exit`), rather than killing the terminal.
- **Pseudo-Terminal (PTY) Execution:** Commands on Unix-like beta servers run inside a PTY master. Visual output (such as columns, grids, and colors in `ls` or `grep`) matches native terminals perfectly.
- **Classic Bash Prompt:** Features a classic, high-visibility Bash-style prompt (bold green device/shell and bold blue directory path).
- **Persistent Pipeline Colorization:** Automatically assigns permanent, unique, bright ANSI colors to devices to keep complex cross-machine pipelines easily readable on dark terminals.
- **Direct IP Connections:** Connect straight to a machine by its IP (IPv4/IPv6), bypassing mDNS lookup.

## Device Addressing

| Syntax | Resolves to |
|--------|-------------|
| `alpha` / `a` / `1` | The controller (your machine) |
| `beta` / `b` / `2` | First connected beta |
| `3`, `4`, ... | Nth beta by connection order |
| `42`, `123` | Beta whose IP ends with that octet |
| `.42` | Explicit octet lookup |
| `192.168.1.50` / `fe80::1` | Beta with that specific IP address |

## Interactive Commands

| Command | Description |
|---------|-------------|
| `@switch` | List connected betas |
| `@switch 2` | Switch to beta with ID 2 (or octet 2) |
| `@switch .2` | Switch to beta with octet 2 |
| `@cp <src> <dst>` | Copy file between devices (supports quotes and spaces) |
| `@mv <src> <dst>` | Move file between devices (supports quotes and spaces) |
| `@renreg <pat> [rep]`| Batch rename files in active directory using regex |
| `@clear` / `@cls` | Clear the alpha terminal |
| `@whoami` | Show active beta system info (clean IP without port) |
| `@help` | Show available commands |
| `@exit` / `@quit` | Exit alpha mode |
| `#` | List available shells on active beta |
| `#bash` / `#cmd` | Switch active shell on beta |
| `exit` | Pop back to previous shell (disconnects from last) |

## Cross-Machine Pipelines

Use `$` to route a pipeline stage through a specific device:

```bash
ipconfig | $2 grep "192" | clip
```

This runs `ipconfig` on the active beta, pipes through `grep 192` on beta ID 2,
then sends the result to `clip` on the active beta.

Use `$` alone to run a stage on the alpha (controller):

```bash
ipconfig | $grep 192 | clip
```

Each stage gets colorized in real time as you type (highlighting targets using their device-specific colors) and runs concurrently.

## File Transfer Syntax

```
@cp [src_dev] <src_path> [dst_dev] <dst_path>
@mv [src_dev] <src_path> [dst_dev] <dst_path>
```

Device defaults to the active beta if omitted. Device is specified with `$N` or
`$.N` before the path:

```bash
@cp ./local.txt /remote/path              # active beta to active beta
@cp ./local.txt $2 /remote/path           # alpha to beta ID 2
@cp $2 /remote/file ./local               # beta ID 2 to alpha
@cp $2 /src/path $3 /dst/path             # beta ID 2 to beta ID 3
@cp $2:/remote/file ./local               # combined $N:path in one arg
```

## Shell Stack (exit)

Shells are tracked as a stack. `#bash` pushes bash on top, `exit` pops back:

```
default shell (bash)  →  #dash  →  #sh  →  exit  →  exit  →  exit
                                                          disconnect
```

## Build from Source

```bash
git clone -b develop https://github.com/mikelneonedwin/zhh
cd zhh
go build -o zhh .
```
