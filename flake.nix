{
  description = "devagent - Development Agent Orchestrator";

  inputs = {
    flake-parts.url = "github:hercules-ci/flake-parts";
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    tsnsrv.url = "github:boinkor-net/tsnsrv";
  };

  outputs = inputs @ {
    self,
    flake-parts,
    ...
  }:
    flake-parts.lib.mkFlake {inherit inputs;} {
      imports = [
        inputs.flake-parts.flakeModules.easyOverlay
        inputs.flake-parts.flakeModules.partitions
      ];
      systems = [
        "x86_64-darwin"
        "x86_64-linux"
        "aarch64-darwin"
        "aarch64-linux"
      ];
      perSystem = {
        config,
        lib,
        pkgs,
        ...
      }: {
        overlayAttrs = {
          inherit (config.packages) devagent;
        };
        packages = {
          default = config.packages.devagent;
          devagent = let
            unwrapped = pkgs.buildGo124Module {
              pname = "devagent";
              version = "0.1.0";
              vendorHash = builtins.readFile ./devagent.sri;
              src = lib.sourceFilesBySuffices (lib.sources.cleanSource ./.) [
                ".go"
                ".mod"
                ".sum"
              ];
              ldflags = [
                "-s"
                "-w"
              ];
            };
            tsnsrvPkg = inputs.tsnsrv.packages.${pkgs.system}.tsnsrv;
          in
            pkgs.symlinkJoin {
              name = "devagent";
              paths = [unwrapped tsnsrvPkg];
            };
        };

        formatter = pkgs.alejandra;
      };

      partitionedAttrs = {
        apps = "dev";
        checks = "dev";
        devShells = "dev";
      };
      partitions.dev = {
        extraInputsFlake = ./dev;
        module = ./dev/flake-part.nix;
      };
      flake = {
        overlays.default = inputs.self.overlays.additions;
      };
    };
}
