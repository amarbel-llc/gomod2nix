# gomod2nix build & test orchestration.
# Recipe conventions: see eng-design_patterns-justfile(7).
# Devshell tools (go, gomod2nix, golangci-lint, treefmt) are invoked via
# `nix develop --command` so recipes work from a bare shell and inside the
# spinclass merge hook, not only under an active direnv devshell.

# Build and verify everything — the CI-equivalent target.
default: validate lint build test

[group("pre-build")]
validate: validate-devshell

# Build the devShell so vendor-env / mkGoEnv breakage that the prod build can mask fails here.
[group("pre-build")]
validate-devshell:
    nix build --no-link .#devShells.{{ arch() }}-linux.default

[group("pre-build")]
lint: lint-go lint-fmt

# Vet the Go sources with golangci-lint.
[group("pre-build")]
lint-go:
    nix develop --command golangci-lint run

# Read-only nix formatting gate (treefmt --ci); the modifying counterpart is codemod-fmt-treefmt.
[group("pre-build")]
lint-fmt:
    nix develop --command treefmt --ci

[group("build")]
build: build-gomod2nix build-go build-nix

# Regenerate gomod2nix.toml from go.mod/go.sum (CI requires it committed and current).
[group("build")]
build-gomod2nix:
    nix develop --command gomod2nix

# Compile ./gomod2nix — tests/run.go invokes that binary from the repo root.
[group("build")]
build-go: build-gomod2nix
    nix develop --command go build

# Build the Nix package (.#default).
[group("build")]
build-nix:
    nix build --show-trace

[group("post-build")]
test: test-go test-nix

# Go unit tests for the internal packages (internal/lib executor, internal/generate targets).
[group("post-build")]
test-go:
    nix develop --command go test ./...

# Full Nix integration suite — builds every tests/* project with nix-build (heavy; needs network).
[group("post-build")]
test-nix: build-go
    nix develop --command go run tests/run.go

[group("codemod")]
codemod-fmt: codemod-fmt-treefmt

# Rewrite the worktree to canonical nix formatting (treefmt); read-only gate is lint-fmt.
[group("codemod")]
codemod-fmt-treefmt:
    nix develop --command treefmt

# Tidy go.mod/go.sum, then regenerate gomod2nix.toml.
[group("maintenance")]
update-go: && build-gomod2nix
    nix develop --command go mod tidy

[group("maintenance")]
clean: clean-build

# Remove build artifacts (the nix result symlink and the go binary).
[group("maintenance")]
clean-build:
    rm -rf result gomod2nix
