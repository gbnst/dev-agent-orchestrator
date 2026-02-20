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
    ...
  }: {
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
        pkgs.go_1_24
        pkgs.gopls
        (pkgs.delve.override { buildGoModule = pkgs.buildGo124Module; })
        pkgs.golangci-lint
        pkgs.lefthook
      ];
    };
  };
}
