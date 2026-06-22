{
  description = "flash — spaced-repetition CLI with TUI and optional server";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          config.allowUnfree = true;
          config.android_sdk.accept_license = true;
        };

        flash = pkgs.buildGoModule {
          pname = "flash";
          version = "0.1.0";
          src = ./cli;
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

        androidComposition = pkgs.androidenv.composeAndroidPackages {
          cmdLineToolsVersion = "13.0";
          platformToolsVersion = "35.0.2";
          buildToolsVersions = [
            "35.0.0"
            "34.0.0"
          ];
          platformVersions = [
            "35"
            "34"
          ];

          includeNDK = true;
          ndkVersions = [ "29.0.14206865" ];

          includeEmulator = true;
          includeSources = false;

          systemImageTypes = [ "google_apis" ];
          abiVersions = [ "x86_64" ];
        };

        androidSdk = androidComposition.androidsdk;
      in
      {
        packages = {
          default = flash;
          docker  = dockerImage;
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            gotools
            scdoc

            androidSdk
            jdk21
            gradle
            kotlin
            android-tools
            nmap
            kotlin-language-server
            ktfmt
            lemminx

            sqlite

            just
            gnumake
            git
          ];

          env = {
            ANDROID_HOME = "${androidSdk}/libexec/android-sdk";
            ANDROID_SDK_ROOT = "${androidSdk}/libexec/android-sdk";
            JAVA_HOME = "${pkgs.jdk21}";

            GRADLE_OPTS = "-Dorg.gradle.daemon=true -Xmx4g -Dorg.gradle.jvmargs=-Xmx4g";
          };

          shellHook = ''
            export ANDROID_HOME="${androidSdk}/libexec/android-sdk"
            export ANDROID_SDK_ROOT="$ANDROID_HOME"
            export JAVA_HOME="${pkgs.jdk21}"
            export PATH="$ANDROID_HOME/platform-tools:$ANDROID_HOME/build-tools/35.0.0:$PATH"

            export AAPT2="$ANDROID_HOME/build-tools/35.0.0/aapt2"

            export FLASH_STORE_FILE="$HOME/.android/flash-release.jks"
            export FLASH_KEY_ALIAS="flash"
            export FLASH_STORE_PASSWORD="$(pass android/signing-key-password)"
            export FLASH_KEY_PASSWORD="$(pass android/signing-key-password)"

            cat > android/gradle.properties << EOF
            android.useAndroidX=true
            android.suppressUnsupportedCompileSdk=35
            org.gradle.jvmargs=-Xmx4g
            EOF
          '';
        };
      }
    );
}
