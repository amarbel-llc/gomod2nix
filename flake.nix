{
  description = "Convert go.mod/go.sum to Nix packages";

  inputs.nixpkgs-master.url = "github:NixOS/nixpkgs/d233902339c02a9c334e7e593de68855ad26c4cb";
  inputs.flake-utils.url = "github:numtide/flake-utils";

  outputs =
    {
      self,
      nixpkgs-master,
      flake-utils,
      ...
    }:
    {
      overlays.default = import ./overlay.nix;

      templates = {
        app = {
          path = ./templates/app;
          description = "Gomod2nix packaged application";
        };
        container = {
          path = ./templates/container;
          description = "Gomod2nix packaged container";
        };
        default = self.templates.app;
      };
    }
    // (flake-utils.lib.eachSystem
      [
        "aarch64-linux"
        "aarch64-darwin"
        "x86_64-darwin"
        "x86_64-linux"
        "riscv64-linux"
      ]
      (
        system:
        let
          pkgs = nixpkgs-master.legacyPackages.${system};

          callPackage = pkgs.callPackage;

          inherit
            (callPackage ./builder {
              inherit gomod2nix;
            })
            mkGoEnv
            buildGoApplication
            hooks
            ;
          gomod2nix = callPackage ./default.nix {
            inherit
              buildGoApplication
              mkGoEnv
              hooks
              ;
          };
        in
        {
          packages.default = gomod2nix;
          legacyPackages = {
            # we cannot put them in packages because they are builder functions
            inherit
              mkGoEnv
              buildGoApplication
              gomod2nix
              hooks
              ;
          };
          devShells.default = callPackage ./shell.nix {
            inherit mkGoEnv gomod2nix;
          };
        }
      )
    );
}
