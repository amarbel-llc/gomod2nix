# Gomod2nix Nix API

## Public functions

### buildGoApplication

Arguments:

- **modules** Path to `gomod2nix.toml` (\_default: `pwd + "/gomod2nix.toml"`).
- **src** Path to sources (\_default: `pwd`).
- **pwd** Path to working directory (\_default: `null`).
- **go** The Go compiler to use (can be omitted).
- **subPackages** Only build these specific sub packages.
- **allowGoReference** Allow references to the Go compiler in the output closure (\_default: `false`).
- **tags** A list of tags to pass the Go compiler during the build (\_default: `[ ]`).
- **ldflags** A list of `ldflags` to pass the Go compiler during the build (\_default: `[ ]`). Auto-injected version/commit ldflags are prepended; caller-supplied entries here take precedence since Go honors the last `-X` for a given symbol. See [Version injection](#version-injection) below.
- **commit** Git commit SHA baked into the binary via `-X main.commit=<commit>` (\_default: `src.rev or src.shortRev or "unknown"`). See [Version injection](#version-injection).
- **nativeBuildInputs** A list of packages to include in the build derivation (\_default: `[ ]`).

All other arguments are passed verbatim to `stdenv.mkDerivation`.

### Version injection

`buildGoApplication` automatically prepends two entries to `ldflags`:

```
-X main.version=<version>
-X main.commit=<commit>
```

For this to show up in the binary, declare matching `var` blocks in your `main` package:

```go
var (
    version = "dev"
    commit  = "unknown"
)
```

The defaults make it obvious when a binary was built outside of Nix.

#### Defaults

- `version` resolves from the `version` argument you pass to `buildGoApplication`. If you don't pass one, it falls back to the version in `gomod2nix.toml` (when building from a `goPackagePath`) or to `"dev"`.
- `commit` resolves from `src.rev` or `src.shortRev` when `src` is a flake input or fetched source, otherwise `"unknown"`.

#### The `cleanSourceWith` pitfall

`lib.cleanSourceWith` strips git metadata from `src`, so `src.rev` disappears. In that case the default `commit` will be `"unknown"` — pass `commit` explicitly:

```nix
buildGoApplication {
  src = lib.cleanSourceWith {
    src = self;
    filter = path: type: lib.hasSuffix ".go" path || baseNameOf path == "go.mod";
  };
  commit = self.shortRev or self.dirtyShortRev or "unknown";
  # ...
}
```

### mkGoEnv

Arguments:

- **pwd** Path to working directory.
- **modules** Path to `gomod2nix.toml` (\_default: `pwd + "/gomod2nix.toml"`).
- **toolsGo** Path to `tools.go` (\_default: `pwd + "/tools.go"`).

All other arguments are passed verbatim to `stdenv.mkDerivation`.
