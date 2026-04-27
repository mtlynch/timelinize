{
  description = "Nix flake for Timelinize";

  inputs = {
    flake-utils.url = "github:numtide/flake-utils";
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = {
    self,
    flake-utils,
    nixpkgs,
  }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = import nixpkgs {
        inherit system;
      };

      lib = pkgs.lib;
      go =
        if pkgs ? go_1_25
        then pkgs.go_1_25
        else pkgs.go;
      buildGoModule = pkgs.buildGoModule.override {inherit go;};
      sourceRoot = toString ./.;
      src = lib.cleanSourceWith {
        src = ./.;
        filter = path: type: let
          relPath = lib.removePrefix (sourceRoot + "/") (toString path);
        in
          !(
            relPath == "flake.nix"
            || relPath == "flake.lock"
            || relPath == "result"
            || lib.hasPrefix "reference/" relPath
          );
      };

      runtimeTools = [
        pkgs.ffmpeg
        pkgs.uv
        pkgs.xdg-utils
      ];

      timelinize = buildGoModule {
        pname = "timelinize";
        version = "0.0.0";
        inherit src;
        vendorHash = "sha256-p33MPNpAP1iBUJq4kMzwvLQ0IEx2I6weVZWTP/BHN8U=";
        subPackages = ["."];

        env.CGO_ENABLED = 1;

        nativeBuildInputs = [
          pkgs.makeWrapper
          pkgs.pkg-config
        ];

        buildInputs = [
          pkgs.sqlite
          pkgs.vips
        ];

        ldflags = [
          "-s"
          "-w"
        ];

        postInstall = ''
          wrapProgram "$out/bin/timelinize" \
            --prefix PATH : ${lib.makeBinPath runtimeTools}
        '';

        meta = {
          mainProgram = "timelinize";
        };
      };

      serve = pkgs.writeShellApplication {
        name = "serve";
        runtimeInputs = [timelinize];
        text = ''
          if [ -n "''${PORT:-}" ] && [ -z "''${TLZ_ADMIN_ADDR:-}" ]; then
            export TLZ_ADMIN_ADDR="127.0.0.1:$PORT"
          fi

          exec timelinize serve "$@"
        '';
      };
    in {
      packages = {
        inherit serve timelinize;
        default = timelinize;
      };

      apps = {
        serve = {
          type = "app";
          program = "${serve}/bin/serve";
        };

        default = self.apps.${system}.serve;
      };

      devShells.default = pkgs.mkShell {
        packages =
          [
            go
            pkgs.pkg-config
            pkgs.sqlite
            pkgs.vips
          ]
          ++ runtimeTools;
      };
    });
}
