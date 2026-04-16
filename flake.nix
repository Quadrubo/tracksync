{
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
  inputs.nixpkgs-unstable.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    {
      self,
      nixpkgs,
      nixpkgs-unstable,
    }:
    let
      supportedSystems = [ "x86_64-linux" ];
      forEachSupportedSystem =
        f:
        nixpkgs.lib.genAttrs supportedSystems (
          system:
          f {
            pkgs = import nixpkgs { inherit system; };
            pkgs-unstable = import nixpkgs-unstable { inherit system; };
            inherit system;
          }
        );
    in
    {
      packages = forEachSupportedSystem (
        { pkgs, pkgs-unstable, ... }:
        {
          default = pkgs-unstable.buildGoModule {
            pname = "tracksync";
            version = self.shortRev or self.dirtyShortRev or "dev";
            src = ./tracksync;
            vendorHash = "sha256-M7X3CgkAxZNcD/bHCX+G24xq6QWswQT3AkaFYkHOXEY=";
          };
        }
      );

      nixosModules.default = import ./nixos/tracksync.nix self;

      devShells = forEachSupportedSystem (
        {
          pkgs,
          system,
          pkgs-unstable,
        }:
        {
          default = pkgs.mkShell {
            packages = with pkgs; [
              just
              jq
              pkgs-unstable.go
              pkgs-unstable.golangci-lint
            ];
          };
        }
      );
    };
}
