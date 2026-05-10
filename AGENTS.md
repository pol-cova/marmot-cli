# Marmot CLI: Agent Instructions

This file contains high-signal, repo-specific context to help OpenCode and other AI agents ramp up quickly.

## Architecture & Entry Points

- **Role**: Marmot CLI is a standalone database backup agent (MySQL, Postgres, MongoDB) written in Go.
- **Entry point**: `cmd/marmot/main.go`.
- **CLI Framework**: Cobra for command routing (`cmd/marmot/cmd/`), Viper for config management (`internal/config`).
- **Daemon Mode**: The CLI can run as a background service (`marmot service install` or `marmot start`) to execute scheduled backups via a cron package.

## Build Quirks

- **NEVER use `go build` directly.** Always use `make build`.
  - *Why*: The application relies on `LDFLAGS` version injection (`-X main.version=...`). If built with bare `go build`, the binary reports as `dev` version and loses its commit context.
- **CGO is Required**: The local queue logic (`internal/storage/queue.go`) uses `github.com/mattn/go-sqlite3`, which strictly requires `CGO_ENABLED=1` and a C compiler.
- **Cross-Compilation**: Because of the CGO requirement, natively cross-compiling Linux binaries from macOS using standard `GOOS=linux go build` will fail. 
  - **Workaround**: Use `make build-linux-docker`. This runs the build inside a `golang:1.25` Docker container, effectively bypassing host CGO limitations.

## Configurations & Data Paths

- **Environment Prefix**: Viper automatically reads environment variables prefixed with `MARMOT_`.
- **Paths**: The agent determines standard paths (config, db, logs) via `internal/config/paths.go`. 
  - On Linux, if `/etc/marmot/` exists or `/etc` is writable, it sets system-wide paths.
  - Otherwise, it falls back to `$HOME/.marmot/`.
  - Config files are usually expected at `config.yaml` within the determined directory.

## Testing & CI

- Run unit tests: `make test` or `make test-coverage`.
- Run linters: `make lint` (runs `go vet` and `golangci-lint` if installed).
- The CI pipeline enforces `make build`, `make test`, and `go vet ./...`.

## Project Docs

- **Trust `README.md`**: `README.md` is the primary source of truth for current functionality (direct-to-storage backup architecture). `marmot.md` may contain legacy references to a "MarmotHub" that is no longer the main focus. Trust the executable code in `internal/remote/storage.go` for available storage implementations.