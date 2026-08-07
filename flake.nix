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
          # Alpha-stage patch versioning: v0.1.0 through v0.1.68 are
          # tagged retroactively across the project's existing history,
          # one per meaningful feature/fix commit (purely cosmetic/
          # merge-noise commits skipped) -- this repo has been used to
          # solve real Advent of Code puzzles, past the point "0.1.0-dev"
          # honestly described it. Going forward, bump this (and the
          # matching -X main.version= below, and tag the commit
          # v0.1.<n>) on each commit that ships a real feature or fix,
          # the same granularity the retroactive tags used. Not every
          # commit needs one -- a docs typo or CI tweak doesn't -- use
          # judgment the way the retroactive pass did.
          version = "0.1.69";

          # Only the Go build's actual inputs -- go.mod/go.sum/cmd/internal,
          # plus examples/. `src = ./.` used to pull in the whole repo
          # (docs/, editors/, assets/, TODO.md, ...), none of which the
          # build touches (nothing uses //go:embed). That meant editing any
          # of those unrelated files busted Nix's build cache and forced a
          # full rebuild -- including buildGoModule's default checkPhase
          # running the entire `go test ./...` suite -- on the next `nix
          # develop .#crust` or direnv reload, even with zero Go source
          # changes. examples/ looks like it belongs in that same
          # "unrelated to the build" pile, but it isn't: several tests
          # (cmd/crust/main_test.go, internal/lexer/lexer_test.go,
          # internal/format/format_test.go) glob examples/*.crust as real
          # test fixtures -- exercising `crust run`/`tokens`/`parse`/
          # `develop` and the formatter against actual `.crust` source, not
          # synthetic snippets. checkPhase runs those tests, so leaving
          # examples/ out of src made every `nix build` fail (files not
          # found) despite `go test ./...` passing fine outside the sandbox.
          src = pkgs.lib.fileset.toSource {
            root = ./.;
            fileset = pkgs.lib.fileset.unions [
              ./go.mod
              ./go.sum
              ./cmd
              ./examples
              ./internal
            ];
          };

          # go.mod's first third-party dependencies (bubbletea + lipgloss,
          # for `crust debug`'s TUI) meant `vendorHash = null` (valid only
          # for a stdlib-only module) no longer worked. This is the real
          # hash printed by `nix build` against go.sum, replacing the
          # `lib.fakeHash` placeholder that was here until someone with
          # Nix actually ran the build once.
          vendorHash = "sha256-uwBJAqN4sIepiiJf9lCDumLqfKJEowQO2tOiSWD3Fig=";

          subPackages = [ "cmd/crust" ];

          ldflags = [ "-s" "-w" "-X main.version=0.1.69" ];

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
