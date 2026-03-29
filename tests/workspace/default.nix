{
  runCommand,
  buildGoApplication,
}:

let
  drv = buildGoApplication {
    pname = "workspace-test";
    version = "0.0.1";
    src = ./.;
    pwd = ./.;
    modules = ./gomod2nix.toml;
    subPackages = [ "moduleB" ];
  };
in
runCommand "workspace-test-assert" { } ''
  if ! ${drv}/bin/moduleB | grep -q "Hello, Workspace!"; then
    echo "workspace binary output unexpected!"
    ${drv}/bin/moduleB
    exit 1
  fi

  ln -s ${drv} $out
''
