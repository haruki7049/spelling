{
  nixConfig = {
    extra-substituters = [ "https://haruki7049.cachix.org" ];
    extra-trusted-public-keys = [
      "haruki7049.cachix.org-1:Hd6hnIsYnpDDNhg/ZX06QkLBaCgDoatgNPqrFnUqhMk="
    ];
  };

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };
    treefmt-nix = {
      url = "github:numtide/treefmt-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    inputs:
    inputs.flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];

      imports = [
        inputs.treefmt-nix.flakeModule
      ];

      perSystem =
        {
          config,
          lib,
          pkgs,
          system,
          ...
        }:
        let
          overlays = [ inputs.gomod2nix.overlays.default ];
          buildInputs = lib.optionals pkgs.stdenv.hostPlatform.isLinux [
            # Build-time dependencies
            pkgs.libx11
            pkgs.libxrandr
            pkgs.libxcursor
            pkgs.libxinerama
            pkgs.libxi
            pkgs.libxxf86vm
            pkgs.alsa-lib

            # Runtime dependencies
            pkgs.libGL
          ];
          nativeBuildInputs = [
            pkgs.go # Golang
            pkgs.pkg-config # pkg-config
            pkgs.nil # Nix LSP
            pkgs.gopls # Golang LSP
            pkgs.gomod2nix # gomod2nix for creating Hashes (./gomod2nix.toml)
            pkgs.cachix # cachix CLI
            pkgs.nushell # Script Runner
          ];

          spelling = pkgs.buildGoApplication {
            name = "spelling";
            src = lib.cleanSource ./.;
            modules = ./gomod2nix.toml;
            inherit buildInputs nativeBuildInputs;
          };
        in
        {
          _module.args.pkgs = import inputs.nixpkgs {
            inherit system overlays;
          };

          treefmt = {
            projectRootFile = ".git/config";

            # Nix
            programs.nixfmt.enable = true;

            # Go
            programs.gofmt.enable = true;

            # GitHub Actions
            programs.actionlint.enable = true;

            # Markdown
            programs.mdformat.enable = true;

            # ShellScript
            programs.shellcheck.enable = true;
            programs.shfmt.enable = true;
          };

          packages = {
            inherit spelling;
            default = spelling;
          };

          checks = {
            inherit spelling;
          };

          devShells.default = pkgs.mkShell {
            inherit buildInputs nativeBuildInputs;

            env.LD_LIBRARY_PATH = lib.makeLibraryPath buildInputs;
            inputsFrom = [ config.treefmt.build.devShell ];
          };
        };
    };
}
