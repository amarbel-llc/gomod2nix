# AGENTS.md

Guidance for coding agents working in this repository. `CLAUDE.md` is a symlink
to this file so Claude Code (claude.ai/code) picks it up too.

## Overview

Gomod2nix converts Go module dependencies (go.mod/go.sum) into Nix package
definitions, enabling reproducible Go builds in Nix. It generates a
`gomod2nix.toml` file containing NAR hashes (SRI-format SHA256) for each
dependency, which the Nix builder uses to fetch and vendor modules.

## Build & Development Commands

This repo is driven by a `justfile` (`just --list` for the full set), following
`eng-design_patterns-justfile(7)`. Recipes wrap the underlying tools in
`nix develop --command`, so they work from a bare shell and inside CI / the
spinclass merge hook, not only under an active direnv devshell.

``` sh
just                 # default: validate lint build test (the CI-equivalent gate)

just build           # build-gomod2nix (regen gomod2nix.toml) + build-go + build-nix
just build-go        # compile ./gomod2nix
just build-nix       # build the Nix package (.#default)

just lint            # lint-go (golangci-lint) + lint-fmt (treefmt --ci, read-only)
just codemod-fmt     # rewrite the tree to canonical nix formatting (treefmt)

just test             # test-go + test-nix (the fast lane)
just test-go          # Go unit tests: go test ./...
just test-nix         # fast Nix integration suite (merge gate): go run tests/run.go
just test-nix-heavy   # heavy CI-only fixtures (minikube, cross); not in `default`
just test-nix-one X   # one integration test by name (e.g. `just test-nix-one helm`)
just list-tests       # list the fast integration tests
just list-tests-heavy # list the heavy CI-only integration tests

just validate-gomod2nix-toml   # fail if gomod2nix.toml is stale (regen + diff)
just update-go                 # go mod tidy, then regen gomod2nix.toml
```

The repo has two kinds of tests:

- **Go unit tests** (`just test-go` → `go test ./...`) covering the `internal/*`
  packages (e.g. `internal/lib` executor, `internal/generate` targets). The
  nested `templates/*` projects are separate Go modules and are excluded from
  `./...`.
- **Nix integration tests** (`just test-nix` → `go run tests/run.go`): each test
  directory under `tests/` contains a Go project that gets built with
  `nix-build`. The runner (`tests/run.go`) either executes a `tests/*/script`
  file or runs gomod2nix + nix-build on the test project. `tests/run.go` sorts
  fixtures into three categories (an explicit list, not the old `GITHUB_ACTIONS`
  env hack):
  - **fast** — the merge gate (`just test-nix`, `run.go list`): the small
    fixtures plus the in-repo `cgo-codegen` fixture (cgo via zlib/pkg-config plus
    a build-time codegen step — the unique paths the giants used to cover).
  - **heavy** — `minikube`, `cross`: a CI-only blocking lane (`just
    test-nix-heavy`, `run.go list-heavy`), excluded from `default` / the merge hook.
  - **quarantined** — `helm`: excluded from every automated lane until
    amarbel-llc/gomod2nix#17 (a `github.com/ugorji/go` vendoring bug) is fixed.
    Still runnable by name: `go run tests/run.go run helm`.

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

CI (`.github/workflows/ci.yml`) routes every job through the justfile, so the
gate matches `just` locally. It runs `lint-fmt` (formatting), `lint-go`
(golangci-lint), `test-go` (Go unit tests), `validate-gomod2nix-toml` (fails if
`gomod2nix.toml` is stale — it regenerates and diffs), the fast Nix integration
suite as a per-test matrix (`list-tests` → `test-nix-one`), and a separate heavy
matrix (`list-tests-heavy` → `test-nix-one`) for the CI-only `minikube`/`cross`
fixtures (`helm` quarantined — see above). Always run `gomod2nix` (or `just
update-go`) after changing Go dependencies.
