{
  description = "ontos development environment";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" ];
      forAllSystems = f:
        nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      # No packages.default, deliberately: scripts/setup.sh builds the binary
      # into ~/.local/bin, because a nixos-rebuild is password-gated and ontos
      # has to be rebuildable from inside a session.
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go
            golangci-lint
            shellcheck
            shfmt
            bash
          ];

          # go.mod pins exactly the Go that nixpkgs ships, so there is no slack.
          # If nixpkgs ever lags, fail loudly rather than silently downloading a
          # toolchain and defeating the point of pinning one here.
          GOTOOLCHAIN = "local";
        };
      });
    };
}
