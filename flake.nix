{
  description = "flash — spaced-repetition CLI with TUI and optional server";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };

        flash = pkgs.buildGoModule {
          pname = "flash";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-J1eItW24CwGUoUpqG6iRwY/xiYKX3BHlQR1GkQx3jyo=";
          meta = {
            description = "Spaced-repetition flashcard CLI";
            mainProgram = "flash";
          };
        };

        dockerImage = pkgs.dockerTools.buildLayeredImage {
          name = "flash";
          tag = "latest";
          contents = [ flash pkgs.cacert ];
          config = {
            Entrypoint = [ "/bin/flash" "serve" ];
            ExposedPorts = { "8765/tcp" = {}; };
            Env = [ "FLASH_SERVE_DATA=/data" ];
            Volumes = { "/data" = {}; };
          };
        };
      in
      {
        packages = {
          default = flash;
          docker  = dockerImage;
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [ go gopls gotools scdoc ];
        };
      }
    );
}
