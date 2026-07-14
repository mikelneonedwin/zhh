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

## Device Addressing

| Syntax | Resolves to |
|--------|-------------|
| `alpha` / `a` / `1` | The controller (your machine) |
| `beta` / `b` / `2` | First connected beta |
| `3`, `4`, ... | Nth beta by connection order |
| `42`, `123` | Beta whose IP ends with that octet |
| `.42` | Explicit octet lookup |

## Interactive Commands

| Command | Description |
|---------|-------------|
| `@switch` | List connected betas |
| `@switch 2` | Switch to beta with ID 2 (or octet 2) |
| `@switch .2` | Switch to beta with octet 2 |
| `@cp <src> <dst>` | Copy file between devices |
| `@mv <src> <dst>` | Move file between devices |
| `@whoami` | Show active beta system info |
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

Each stage gets a unique colour based on its target device, so you can
visually track which commands run where.

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
