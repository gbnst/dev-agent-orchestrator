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
        "x86_64-linux"
        "aarch64-darwin"
        "aarch64-linux"
      ];
      perSystem = {
        config,
        lib,
        pkgs,
        ...
      }: let
        version = let
          versionFile = ./. + "/.version";
        in
          if builtins.pathExists versionFile
          then builtins.replaceStrings ["\n"] [""] (builtins.readFile versionFile)
          else "0.1.0-dev";

        frontend = pkgs.buildNpmPackage {
          pname = "devagent-frontend";
          inherit version;
          src = ./internal/web/frontend;
          npmDepsHash = "sha256-KyaxBD/tYyTrXqHfVtZpPMUNEOMegkqxqmhBKNdb300=";
          buildPhase = ''
            npm run build
          '';
          installPhase = ''
            cp -r dist $out
          '';
        };

        goSrc = let
          cleanedGoSrc = lib.sources.cleanSourceWith {
            src = lib.sources.cleanSource ./.;
            filter = path: type: let
              baseName = builtins.baseNameOf path;
              relPath = lib.removePrefix (toString ./. + "/") (toString path);
            in
              # Exclude frontend source (we inject the built dist separately)
              !(lib.hasPrefix "internal/web/frontend" relPath)
              # Exclude dev/docs/CI artifacts
              && baseName != ".direnv"
              && baseName != ".worktrees"
              && baseName != "work"
              && baseName != "result"
              && baseName != "docs"
              && baseName != ".github";
          };
        in
          pkgs.runCommand "devagent-src" {} ''
            cp -r ${cleanedGoSrc} $out
            chmod -R u+w $out
            mkdir -p $out/internal/web/frontend
            cp -r ${frontend} $out/internal/web/frontend/dist
          '';
      in {
        overlayAttrs = {
          inherit (config.packages) devagent;
        };
        packages =
          {
            default = config.packages.devagent;
            devagent = let
              unwrapped = pkgs.buildGo124Module {
                pname = "devagent";
                inherit version;
                vendorHash = builtins.readFile ./devagent.sri;
                src = goSrc;
                ldflags = [
                  "-s"
                  "-w"
                  "-X main.version=${version}"
                ];
              };
              tsnsrvPkg = inputs.tsnsrv.packages.${pkgs.system}.tsnsrv;
            in
              pkgs.symlinkJoin {
                name = "devagent";
                paths = [unwrapped tsnsrvPkg];
              };
          }
          // lib.optionalAttrs pkgs.stdenv.isLinux {
            devagent-windows = pkgs.buildGo124Module {
              pname = "devagent-windows";
              inherit version;
              vendorHash = builtins.readFile ./devagent.sri;
              src = goSrc;
              env = {
                CGO_ENABLED = "0";
                GOOS = "windows";
                GOARCH = "amd64";
              };
              ldflags = [
                "-s"
                "-w"
                "-X main.version=${version}"
              ];
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
