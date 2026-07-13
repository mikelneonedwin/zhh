# zhh — Remote Shell & File Transfer CLI

`zhh` lets you control remote machines from your terminal — like SSH, but with
multi-device awareness, cross-machine pipelines, and built-in file transfer.

## Quick Start

```bash
# On the machine to be controlled (beta mode is default)
zhh

# On the controller machine (alpha mode)
zhh alpha

# Connect to a specific beta by its last IP octet
zhh alpha 42

# Run a single command and exit
zhh a 42 - rmdir /s /q \mydir
```

## How It Works

**Beta** (default mode) listens for incoming connections and advertises itself
on the network via mDNS. **Alpha** (controller) discovers betas automatically
and gives you an interactive shell.

## Addressing

| Identifier | Resolves to |
|------------|-------------|
| `alpha` / `a` / `1` | The controller (your machine) |
| `beta` / `b` / `2` | First connected beta |
| `3`, `4`, … | Nth beta by connection order |
| `42`, `123` | Beta whose IP ends with that octet |
| `.42` | Explicit octet lookup |

## Interactive Commands

| Command | Description |
|---------|-------------|
| `@switch` | List connected betas and switch active one |
| `@cp a#/src b#/dst` | Copy file between devices |
| `@move a#/src b#/dst` | Move file between devices |
| `@whoami` | Show active beta system info |
| `@help` | Show available commands |
| `@exit` / `@quit` | Exit alpha mode |
| `#` | List available shells on active beta |
| `#bash` / `#cmd` | Switch active shell on beta |

## Cross-Machine Pipelines

Use `$$` to pipe a command stage through your local machine:

```bash
ipconfig | $$grep 192 | clip
```

This runs `ipconfig` on the beta (Windows), pipes through `grep 192` on the
alpha (Linux/macOS), then sends the result to `clip` on the original beta.

## File Transfer Syntax

```
@cp <source> <dest>
@move <source> <dest>
```

Each path is `device#/path/on/device`. For example:

```bash
@cp alpha#./local.txt beta#C:\remote\file.txt    # upload
@cp beta#C:\remote\file.txt alpha#./local.txt    # download
@cp 123#~/doc.txt 124#~/doc.txt                  # beta to beta
```

Shortcuts: `2#/path` or `2:/path` for numeric ID, `.42#/path` or `.42:/path`
for octet lookup.

## Build from Source

```bash
git clone https://github.com/mikelneonedwin/zhh
cd zhh
go build -o zhh .
```
