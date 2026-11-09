{
    description = "micro editor with tree-sitter support";

    inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

    outputs =
        { self, nixpkgs }:
        let
            systems = [
                "x86_64-linux"
                "aarch64-linux"
            ];
            forSystems = nixpkgs.lib.genAttrs systems;
            version = "2.0.15";
        in
        {
            packages = forSystems (
                system:
                let
                    pkgs = nixpkgs.legacyPackages.${system};
                    micro = pkgs.buildGoModule {
                        pname = "micro";
                        inherit version;
                        src = pkgs.lib.cleanSourceWith {
                            src = ./.;
                            filter =
                                path: _:
                                !(builtins.elem (baseNameOf path) [
                                    ".git"
                                    "result"
                                ]);
                        };
                        vendorHash = "sha256-bkPd6zB9e4q6N20wbKS8n8zGGITOoScajdPYv7Race0=";
                        proxyVendor = true;
                        nativeBuildInputs = [ pkgs.installShellFiles ];
                        outputs = [
                            "out"
                            "man"
                        ];
                        subPackages = [ "cmd/micro" ];
                        ldflags = [
                            "-s"
                            "-w"
                            "-X github.com/micro-editor/micro/v2/internal/util.Version=${version}"
                            "-X github.com/micro-editor/micro/v2/internal/util.CommitHash=${
                                self.shortRev or (self.dirtyShortRev or "unknown")
                            }"
                        ];
                        postInstall = ''
                            installManPage assets/packaging/micro.1
                            install -Dm444 assets/packaging/micro.desktop $out/share/applications/micro.desktop
                            install -Dm644 assets/micro-logo-mark.svg $out/share/icons/hicolor/scalable/apps/micro.svg
                        '';
                        meta = {
                            description = "Modern and intuitive terminal-based text editor";
                            homepage = "https://micro-editor.github.io";
                            license = pkgs.lib.licenses.mit;
                            mainProgram = "micro";
                            platforms = pkgs.lib.platforms.linux;
                        };
                    };
                    micro-treesitter = (pkgs.buildGoModule.override { go = pkgs.go_1_27; }) {
                        pname = "micro-treesitter";
                        inherit version;
                        src = ./treesitter;
                        vendorHash = "sha256-NLIDNdtFiBE/vx21lrwAkAozP0mk5drVDDrZYnTcX70=";
                        proxyVendor = true;
                        subPackages = [ "cmd/micro-treesitter" ];
                        env.CGO_ENABLED = "1";
                        env.CGO_CFLAGS = "-O2";
                        ldflags = [
                            "-s"
                            "-w"
                            "-X main.defaultGrammarDir=${placeholder "out"}/share/micro/treesitter/grammars"
                        ];
                        postInstall = ''
                            mkdir -p $out/share/micro/treesitter/grammars
                            cp -r queries/*/ $out/share/micro/treesitter/grammars/
                        '';
                        meta = {
                            description = "Tree-sitter highlight helper for micro";
                            homepage = "https://github.com/micro-editor/micro";
                            license = pkgs.lib.licenses.mit;
                            mainProgram = "micro-treesitter";
                            platforms = pkgs.lib.platforms.linux;
                        };
                    };
                in
                {
                    default = micro;
                    micro = micro;
                    micro-treesitter = micro-treesitter;
                }
            );

            formatter = forSystems (system: nixpkgs.legacyPackages.${system}.nixfmt-rfc-style);
        };
}
