{
  self,
  inputs,
  ...
}: {
  imports = [
    inputs.devshell.flakeModule
    inputs.generate-go-sri.flakeModules.default
  ];
  systems = [
    "x86_64-linux"
    "aarch64-darwin"
  ];

  perSystem = {
    config,
    pkgs,
    system,
    ...
  }: let
    # Pinned nixpkgs that still ships the Go 1.24 toolchain; mirrors the
    # nixpkgs-go124 input in the top-level flake (nixpkgs-unstable HEAD has
    # dropped Go 1.24). Keep this rev in sync with that input.
    pkgsGo124 = inputs.nixpkgs-go124.legacyPackages.${system};
  in {
    go-sri-hashes.devagent = {};

    devshells.default = {
      commands = [
        {
          name = "regenSRI";
          category = "dev";
          help = "Regenerate devagent.sri in case the module SRI hash should change";
          command = "${config.apps.generate-sri-devagent.program}";
        }
      ];
      packages = [
        pkgsGo124.go_1_24
        pkgs.gopls
        (pkgs.delve.override {buildGoModule = pkgsGo124.buildGo124Module;})
        pkgs.golangci-lint
        pkgs.lefthook
        inputs.tsnsrv.packages.${pkgs.system}.tsnsrv
      ];
    };
  };
}
