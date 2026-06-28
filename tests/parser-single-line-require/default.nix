# Regression test for issue #15.
#
# `go mod tidy` emits a module's lone direct dependency as a single-line
# `require x v1` and its indirect dependencies as a `require ( ... )` block
# whenever the module has exactly one direct dependency. The go.mod parser
# must merge the single-line require into the same attrset the block
# accumulates; before the fix it stored the single line as a bare string and
# the following block aborted eval with "expected a set but found a string".
#
# This test forces `parseGoMod`'s require attrset (the build path otherwise
# leaves it as an unforced lazy thunk, which is why the bug stayed latent in
# this repo's integration suite), so a regression fails at eval time.
{ runCommand }:

let
  inherit (import ../../builder/parser.nix) parseGoMod;

  parsed = parseGoMod ''
    module example.com/one-direct-dep

    go 1.24

    require example.com/direct v1.2.3

    require (
      example.com/indirect-a v0.1.0 // indirect
      example.com/indirect-b v0.2.0 // indirect
    )
  '';

  expected = {
    "example.com/direct" = "v1.2.3";
    "example.com/indirect-a" = "v0.1.0";
    "example.com/indirect-b" = "v0.2.0";
  };
in
assert parsed.require == expected;
runCommand "parser-single-line-require-ok" { } "touch $out"
