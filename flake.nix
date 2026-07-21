{
  description = "cRust — an interpreted language where every keyword is pizza jargon";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "crust";
          version = "0.1.0-dev";
          src = ./.;

          # No third-party Go dependencies yet (stdlib only), so there's
          # nothing to vendor. Once go.mod gains a `require`, this needs
          # to become a real hash — `nix build` will print the correct
          # value on a mismatch.
          vendorHash = null;

          subPackages = [ "cmd/crust" ];

          ldflags = [ "-s" "-w" "-X main.version=0.1.0-dev" ];

          meta = {
            description = "An interpreted language where every keyword is pizza jargon";
            homepage = "https://github.com/Sintfoap/cRust";
            license = pkgs.lib.licenses.mit;
            mainProgram = "crust";
          };
        };

        apps.default = flake-utils.lib.mkApp {
          drv = self.packages.${system}.default;
        };

        devShells.default = pkgs.mkShell {
          packages = [ pkgs.go ];
        };
      });
}
