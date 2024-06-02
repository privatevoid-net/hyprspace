{
  description = "Hyprspace";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };
  };

  outputs =
    inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
        "x86_64-darwin"
      ];
      flake.herculesCI.ciSystems = [
        "x86_64-linux"
        "aarch64-linux"
      ];

      imports = [
        ./dev
      ];

      perSystem =
        { config, pkgs, ... }:
        {
          packages = {
            default = pkgs.callPackage ./package.nix {};
            docs = pkgs.callPackage ./docs/package.nix {
              hyprspace = config.packages.default;
            };
          };
        };
    };
}
