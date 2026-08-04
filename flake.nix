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

          # go.mod gained its first third-party dependencies (bubbletea +
          # lipgloss, for `crust debug`'s TUI), so `vendorHash = null`
          # (valid only for a stdlib-only module) no longer works.
          # `lib.fakeHash` is nixpkgs' own placeholder for exactly this
          # situation — `nix build` fails with a hash mismatch that
          # prints the real value to paste in here. Not computed in this
          # change since it needs an actual Nix build to produce (not
          # something derivable by reading go.sum), so the very next
          # `nix build`/`nix develop` against this commit is expected to
          # fail once, informatively, until someone with Nix available
          # updates this to the printed hash.
          vendorHash = pkgs.lib.fakeHash;

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

        # For hacking on cRust's own source — Go toolchain only, not the
        # built `crust` binary itself (you'd use `go run`/`go build`
        # directly while working in this repo, not the packaged CLI).
        devShells.default = pkgs.mkShell {
          packages = [ pkgs.go ];
        };

        # For *using* cRust — e.g. writing/running AoC solutions — without
        # needing a separate workspace flake or touching your shell's
        # global PATH: `nix develop .#crust` drops you into a shell with
        # just the built `crust` CLI on PATH.
        devShells.crust = pkgs.mkShell {
          packages = [ self.packages.${system}.default ];

          shellHook = ''
            echo "$(crust --version) ready — try: crust --help"
          '';
        };
      });
}
