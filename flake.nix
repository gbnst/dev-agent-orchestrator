{
  description = "devagent - Development Agent Orchestrator";

  inputs = {
    flake-parts.url = "github:hercules-ci/flake-parts";
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    # nixpkgs-unstable HEAD has dropped the Go 1.24 builder (only buildGo123/
    # 125/126Module remain), so we cannot build devagent against floating
    # `nixpkgs`. Pin the last rev that still ships buildGo124Module/go_1_24 and
    # source only the Go toolchain from it; everything else tracks `nixpkgs`.
    # This must be a fixed rev, not the nixpkgs-unstable ref: a floating ref
    # would re-resolve to a tip where the Go 1.24 builder is gone, breaking the
    # build non-reproducibly.
    nixpkgs-go124.url = "github:NixOS/nixpkgs/5b265bda51b42a2a85af0a543c3e57b778b01b7d";
    tsnsrv.url = "github:boinkor-net/tsnsrv";
  };

  outputs = inputs @ {
    self,
    flake-parts,
    ...
  }:
    flake-parts.lib.mkFlake {inherit inputs;} {
      imports = [
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
        system,
        ...
      }: let
        # Pinned nixpkgs that still provides the Go 1.24 builder; see the
        # nixpkgs-go124 input comment above.
        pkgsGo124 = inputs.nixpkgs-go124.legacyPackages.${system};

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
          npmDepsHash = "sha256-GN10VImnQabNiJ4IipSMmnm2XOY50FXOjnNQSrDKT4I=";
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
        packages =
          {
            default = config.packages.devagent;
            devagent = let
              unwrapped = pkgsGo124.buildGo124Module {
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
            devagent-windows = pkgsGo124.buildGo124Module {
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
              postInstall = ''
                mv $out/bin/devagent $out/bin/devagent.exe
              '';
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
        overlays.default = final: prev: {
          devagent = self.packages.${prev.stdenv.hostPlatform.system}.devagent;
        };
      };
    };
}
