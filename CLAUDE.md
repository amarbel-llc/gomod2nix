# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository.

## Overview

Gomod2nix converts Go module dependencies (go.mod/go.sum) into Nix package
definitions, enabling reproducible Go builds in Nix. It generates a
`gomod2nix.toml` file containing NAR hashes (SRI-format SHA256) for each
dependency, which the Nix builder uses to fetch and vendor modules.

## Build & Development Commands

``` sh
# Enter dev shell
nix-shell                    # or nix develop

# Build
go build                     # Go binary
nix build                    # Nix package

# Regenerate gomod2nix.toml (must be up-to-date for CI)
gomod2nix

# Lint
golangci-lint run

# Format
treefmt --ci                 # all formats
nixfmt-rfc-style flake.nix   # nix only

# Tests (uses custom runner, not `go test`)
go run tests/run.go                    # run all tests
go run tests/run.go list               # list available tests
go run tests/run.go run <test-name>    # run a single test
```

Tests are Nix-based integration tests: each test directory under `tests/`
contains a Go project that gets built with `nix-build`. The runner
(`tests/run.go`) either executes a `tests/*/script` file or runs gomod2nix +
nix-build on the test project. Tests blacklisted in CI: helm, minikube, cross.

## Architecture

### CLI → Generation → TOML

`main.go` → `internal/cmd/root.go` (Cobra CLI) → `internal/generate/generate.go`
→ `internal/schema/schema.go`

- **generate** command (default): runs `go mod download --json`, computes NAR
  hashes via go-nix, writes `gomod2nix.toml` (schema v3)
- **import** command: pre-imports packages into the Nix store using
  `nix-instantiate`
- Parallel execution via `internal/lib/executor.go` (default 10 workers)

### Nix Builder (`builder/`)

The builder provides two main Nix functions exported via `overlay.nix`:

- **buildGoApplication**: builds a Go application from source +
  `gomod2nix.toml`. Auto-selects Go version from go.mod. Creates vendor
  directory, optionally restores build cache, applies trimpath by default.
- **mkGoEnv**: creates a development shell with vendored dependencies and tools
  from tools.go.

Supporting components: - `builder/parser.nix` --- pure-Nix go.mod parser
(handles require/replace/exclude directives) - `builder/hooks/` --- four setup
hooks that configure Go environment, build, test, and install -
`builder/symlink/` --- Go tool that creates the vendor symlink tree -
`builder/cachegen/` --- Go tool that generates import files for build cache
priming - `builder/install/` --- Go tool that installs tools from tools.go

### Build Cache System

`gomod2nix.toml` includes a `cachePackages` list. The builder's `mkGoCacheEnv`
pre-compiles these packages into a zstd-compressed tar, which `goConfigHook`
restores during builds to speed up compilation.

### Cross-Compilation

Build hooks use `buildPackages` for host-time dependencies. The builder
distinguishes `nativeBuildInputs` (host) from `buildInputs` (target) following
nixpkgs conventions.

## CI Requirements

CI validates that `gomod2nix.toml` is up-to-date by regenerating it and diffing.
Always run `gomod2nix` after changing Go dependencies.
