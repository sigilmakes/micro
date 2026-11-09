// Code generated from runtime/syntax/*.yaml; DO NOT EDIT.

package filetype

var table = []Spec{
	{Filetype: "powershell", Filename: "\\.ps(1|m1|d1)$"},                                                                   // PowerShell.yaml
	{Filetype: "ada", Filename: "(\\.ads$|\\.adb$|\\.ada$)"},                                                                // ada.yaml
	{Filetype: "apacheconf", Filename: "httpd\\.conf|mime\\.types|vhosts\\.d\\\\*|\\.htaccess"},                             // apacheconf.yaml
	{Filetype: "ino", Filename: "\\.ino$"},                                                                                  // arduino.yaml
	{Filetype: "asciidoc", Filename: "\\.(asc|asciidoc|adoc)$"},                                                             // asciidoc.yaml
	{Filetype: "asm", Filename: "\\.(S|s|asm)$"},                                                                            // asm.yaml
	{Filetype: "ATS", Filename: "\\.(d|h|s)ats$"},                                                                           // ats.yaml
	{Filetype: "awk", Filename: "\\.awk$", Header: "^#!.*bin/(env +)?awk( |$)"},                                             // awk.yaml
	{Filetype: "B", Filename: "\\.b$", Signature: "\\b(if|else|while|switch|case|default|break|return|goto|extrn|auto)\\b"}, // b.yaml
	{Filetype: "batch", Filename: "(\\.bat$|\\.cmd$)"},                                                                      // bat.yaml
	{Filetype: "c", Filename: "(\\.(c|C)$|\\.(h|H)$|\\.ii?$|\\.(def)$)"},                                                    // c.yaml
	{Filetype: "caddyfile", Filename: "Caddyfile"},                                                                          // caddyfile.yaml
	{Filetype: "cake", Filename: "\\.cake$"},                                                                                // cake.yaml
	{Filetype: "clojure", Filename: "\\.(clj[sc]?|edn)$"},                                                                   // clojure.yaml
	{Filetype: "cmake", Filename: "(CMakeLists\\.txt|\\.cmake)$"},                                                           // cmake.yaml
	{Filetype: "coffeescript", Filename: "\\.coffee$"},                                                                      // coffeescript.yaml
	{Filetype: "colortest", Filename: "ColorTest$"},                                                                         // colortest.yaml
	{Filetype: "conky", Filename: "(\\.*conkyrc.*$|conky.conf)"},                                                            // conky.yaml
	{Filetype: "c++", Filename: "(\\.c(c|pp|xx)$|\\.h(h|pp|xx)?$|\\.ii?$|\\.(def)$)", Signature: "\\b(namespace|class|public|protected|private|template|constexpr|noexcept|nullptr|throw)\\b"}, // cpp.yaml
	{Filetype: "crontab", Filename: "crontab$|/tmp/crontab\\.\\w+$", Header: "^#.*?/etc/crontab"},                                                                                              // crontab.yaml
	{Filetype: "crystal", Filename: "\\.cr$"},                                                                              // crystal.yaml
	{Filetype: "csharp", Filename: "\\.cs$"},                                                                               // csharp.yaml
	{Filetype: "css", Filename: "\\.(css|scss)$"},                                                                          // css.yaml
	{Filetype: "csharp-script", Filename: "\\.csx$", Header: "^#!.*/(env +)?dotnet-script( |$)"},                           // csx.yaml
	{Filetype: "cuda", Filename: "(\\.cu[h]?$)"},                                                                           // cuda.yaml
	{Filetype: "cython", Filename: "\\.pyx$|\\.pxd$|\\.pyi$"},                                                              // cython.yaml
	{Filetype: "d", Filename: "\\.(d(i|d)?)$"},                                                                             // d.yaml
	{Filetype: "dart", Filename: "\\.dart$"},                                                                               // dart.yaml
	{Filetype: "dockerfile", Filename: "((Docker|Container)file[^/]*$|\\.(docker|container)file$)"},                        // dockerfile.yaml
	{Filetype: "dot", Filename: "\\.(dot|gv)$"},                                                                            // dot.yaml
	{Filetype: "elixir", Filename: "\\.ex$|\\.exs$"},                                                                       // elixir.yaml
	{Filetype: "elm", Filename: "\\.elm$"},                                                                                 // elm.yaml
	{Filetype: "erb", Filename: "\\.erb$|\\.rhtml$"},                                                                       // erb.yaml
	{Filetype: "erlang", Filename: "\\.erl$"},                                                                              // erlang.yaml
	{Filetype: "fish", Filename: "\\.fish$", Header: "^#!.*/(env +)?fish( |$)"},                                            // fish.yaml
	{Filetype: "forth", Filename: "\\.(forth|4th|fs|fs8|ft|fth|frt)$"},                                                     // forth.yaml
	{Filetype: "fortran", Filename: "\\.([Ff]|[Ff]90|[Ff]95|[Ff][Oo][Rr])$"},                                               // fortran.yaml
	{Filetype: "freebsd-kernel", Filename: "GENERIC$"},                                                                     // freebsd-kernel.yaml
	{Filetype: "fsharp", Filename: "\\.fs?$"},                                                                              // fsharp.yaml
	{Filetype: "gdscript", Filename: "\\.gd$"},                                                                             // gdscript.yaml
	{Filetype: "gemini", Filename: "\\.(gmi|gemini)$"},                                                                     // gemini.yaml
	{Filetype: "ebuild", Filename: "\\.e(build|class)$"},                                                                   // gentoo-ebuild.yaml
	{Filetype: "etc-portage", Filename: "\\.(keywords|mask|unmask|use)(/.+)?$"},                                            // gentoo-etc-portage.yaml
	{Filetype: "git-commit", Filename: "^(.*[\\\\/])?(COMMIT_EDITMSG|TAG_EDITMSG|MERGE_MSG)$"},                             // git-commit.yaml
	{Filetype: "git-config", Filename: "git(config|modules)$|^(.*[\\\\/])?\\.?git[\\\\/]config$"},                          // git-config.yaml
	{Filetype: "git-rebase-todo", Filename: "^(.*[\\\\/])?git\\-rebase\\-todo$"},                                           // git-rebase-todo.yaml
	{Filetype: "gleam", Filename: "\\.gleam$"},                                                                             // gleam.yaml
	{Filetype: "glsl", Filename: "\\.(frag|vert|fp|vp|glsl)$"},                                                             // glsl.yaml
	{Filetype: "gnuplot", Filename: "\\.(gnu|gpi|plt|gp)$"},                                                                // gnuplot.yaml
	{Filetype: "go", Filename: "\\.go$"},                                                                                   // go.yaml
	{Filetype: "godoc", Filename: "\\.godoc$", Header: "package.*import"},                                                  // godoc.yaml
	{Filetype: "golo", Filename: "\\.golo$"},                                                                               // golo.yaml
	{Filetype: "gomod", Filename: "go.mod"},                                                                                // gomod.yaml
	{Filetype: "graphql", Filename: "\\.(gql|graphql)$"},                                                                   // graphql.yaml
	{Filetype: "groff", Filename: "\\.m[ems]$|\\.rof|\\.tmac$|^tmac."},                                                     // groff.yaml
	{Filetype: "groovy", Filename: "(\\.(groovy|gy|gvy|gsh|gradle)$|^[Jj]enkinsfile$)", Header: "^#!.*/(env +)?groovy *$"}, // groovy.yaml
	{Filetype: "haml", Filename: "\\.haml$"},                                                                               // haml.yaml
	{Filetype: "hare", Filename: "\\.ha$"},                                                                                 // hare.yaml
	{Filetype: "haskell", Filename: "\\.hs$"},                                                                              // haskell.yaml
	{Filetype: "hc", Filename: "(\\.(hc|HC)$|\\.(hh|HH)$|\\.ii?$|\\.(def)$)"},                                              // hc.yaml
	{Filetype: "html", Filename: "\\.htm[l]?$"},                                                                            // html.yaml
	{Filetype: "html4", Filename: "\\.htm[l]?4$", Header: "<!DOCTYPE HTML PUBLIC \"-//W3C//DTD HTML 4.01//EN|http://www.w3.org/TR/html4/strict.dtd\">"}, // html4.yaml
	{Filetype: "html5", Filename: "\\.htm[l]?5$", Header: "<!DOCTYPE html5>"},                                                                           // html5.yaml
	{Filetype: "ini", Filename: "\\.(ini|desktop|lfl|override|tscn|tres)$|(mimeapps\\.list|pinforc|setup\\.cfg|project\\.godot)$|weechat/.+\\.conf$"},   // ini.yaml
	{Filetype: "inputrc", Filename: "inputrc$"},                                                                                                // inputrc.yaml
	{Filetype: "java", Filename: "\\.java$"},                                                                                                   // java.yaml
	{Filetype: "javascript", Filename: "(\\.(m|c)?js$|\\.es[5678]?$)", Header: "^#!.*/(env +)?node( |$)"},                                      // javascript.yaml
	{Filetype: "jinja2", Filename: "\\.(j2|jinja2?)$"},                                                                                         // jinja2.yaml
	{Filetype: "json", Filename: "\\.jsonc?$", Header: "^\\{$"},                                                                                // json.yaml
	{Filetype: "jsonnet", Filename: "\\.(jsonnet|libsonnet)$"},                                                                                 // jsonnet.yaml
	{Filetype: "julia", Filename: "\\.jl$", Header: "^#!.*/(env +)?julia( |$)"},                                                                // julia.yaml
	{Filetype: "justfile", Filename: "(\\.?[Jj]ustfile|\\.just)$", Header: "^#!.*/(env +)?[bg]?just --justfile"},                               // justfile.yaml
	{Filetype: "keymap", Filename: "\\.(k|key)?map$|Xmodmap$"},                                                                                 // keymap.yaml
	{Filetype: "kickstart", Filename: "\\.ks$|\\.kickstart$"},                                                                                  // kickstart.yaml
	{Filetype: "kotlin", Filename: "\\.kts?$"},                                                                                                 // kotlin.yaml
	{Filetype: "kvlang", Filename: "\\.kv$"},                                                                                                   // kvlang.yaml
	{Filetype: "ledger", Filename: "(^|\\.|/)(ledger|ldgr|beancount|bnct)$"},                                                                   // ledger.yaml
	{Filetype: "lfe", Filename: "lfe$|\\.lfe$"},                                                                                                // lfe.yaml
	{Filetype: "lilypond", Filename: "\\.ly$|\\.ily$|\\.lly$"},                                                                                 // lilypond.yaml
	{Filetype: "lisp", Filename: "(emacs|zile)$|\\.(el|li?sp|scm|ss|rkt)$"},                                                                    // lisp.yaml
	{Filetype: "log", Filename: "(\\.log|log\\.txt)$"},                                                                                         // log.yaml
	{Filetype: "lua", Filename: "\\.lua$"},                                                                                                     // lua.yaml
	{Filetype: "mail", Filename: "(.*/mutt-.*|\\.eml)$", Header: "^From .* \\d+:\\d+:\\d+ \\d+"},                                               // mail.yaml
	{Filetype: "makefile", Filename: "([Mm]akefile|\\.ma?k)$", Header: "^#!.*/(env +)?[bg]?make( |$)"},                                         // makefile.yaml
	{Filetype: "man", Filename: "\\.[1-9]x?$"},                                                                                                 // man.yaml
	{Filetype: "markdown", Filename: "\\.(livemd|md|mkd|mkdn|markdown)$"},                                                                      // markdown.yaml
	{Filetype: "mc", Filename: "\\.mc$"},                                                                                                       // mc.yaml
	{Filetype: "meson", Filename: "(meson\\.build|meson_options\\.txt|meson\\.options)"},                                                       // meson.yaml
	{Filetype: "micro", Filename: "\\.(micro)$"},                                                                                               // micro.yaml
	{Filetype: "mpd", Filename: "mpd\\.conf$"},                                                                                                 // mpdconf.yaml
	{Filetype: "msbuild", Filename: "\\.(.*proj|props|targets|tasks)$"},                                                                        // msbuild.yaml
	{Filetype: "nanorc", Filename: "\\.?nanorc$"},                                                                                              // nanorc.yaml
	{Filetype: "nftables", Filename: "(nftables\\.(conf|rules)$|nftables(\\.rules)?\\.d/)", Header: "^(#!.*/(env +)?nft( |$)|flush +ruleset)"}, // nftables.yaml
	{Filetype: "nginx", Filename: "nginx.*\\.conf$|\\.nginx$|\\.sub(domain|folder)\\.conf$", Header: "^(server|upstream)[a-z ]*\\{$"},          // nginx.yaml
	{Filetype: "nim", Filename: "\\.nims?$|nim.cfg"},                                                                                           // nim.yaml
	{Filetype: "nix", Filename: "\\.nix$"},                                                                                                     // nix.yaml
	{Filetype: "nu", Filename: "\\.nu$"},                                                                                                       // nu.yaml
	{Filetype: "objective-c", Filename: "\\.(m|mm|h)$", Signature: "(obj|objective)-c|#import|@(encode|end|interface|implementation|selector|protocol|synchronized|try|catch|finally|property|optional|required|import|autoreleasepool)"}, // objc.yaml
	{Filetype: "ocaml", Filename: "\\.mli?$"},                                           // ocaml.yaml
	{Filetype: "octave", Filename: "\\.m$"},                                             // octave.yaml
	{Filetype: "odin", Filename: "\\.odin$"},                                            // odin.yaml
	{Filetype: "pascal", Filename: "\\.pas$"},                                           // pascal.yaml
	{Filetype: "patch", Filename: "\\.(patch|diff)$", Header: "^diff"},                  // patch.yaml
	{Filetype: "peg", Filename: "\\.l?peg$"},                                            // peg.yaml
	{Filetype: "perl", Filename: "\\.p[lmp]$", Header: "^#!.*/(env +)?perl( |$)"},       // perl.yaml
	{Filetype: "php", Filename: "\\.php[2345s~]?$"},                                     // php.yaml
	{Filetype: "pc", Filename: "\\.pc$"},                                                // pkg-config.yaml
	{Filetype: "po", Filename: "\\.pot?$"},                                              // po.yaml
	{Filetype: "pony", Filename: "\\.pony$"},                                            // pony.yaml
	{Filetype: "pov", Filename: "\\.(pov|POV|povray|POVRAY)$"},                          // pov.yaml
	{Filetype: "privoxy-action", Filename: "\\.action$"},                                // privoxy-action.yaml
	{Filetype: "privoxy-config", Filename: "privoxy/config$"},                           // privoxy-config.yaml
	{Filetype: "privoxy-filter", Filename: "\\.filter$"},                                // privoxy-filter.yaml
	{Filetype: "proto", Filename: "(\\.(proto)$$)"},                                     // proto.yaml
	{Filetype: "prql", Filename: "\\.prql$"},                                            // prql.yaml
	{Filetype: "puppet", Filename: "\\.pp$"},                                            // puppet.yaml
	{Filetype: "python2", Filename: "\\.py2$", Header: "^#!.*/(env +)?python2$"},        // python2.yaml
	{Filetype: "python", Filename: "\\.py(3|w)?$", Header: "^#!.*/(env +)?python(3)?$"}, // python3.yaml
	{Filetype: "r", Filename: "\\.(r|R)$"},                                              // r.yaml
	{Filetype: "raku", Filename: "\\.(p(l|m|od)?6|raku(mod|doc|test)?|nqp)$"},           // raku.yaml
	{Filetype: "rst", Filename: "\\.rest$|\\.rst$"},                                     // reST.yaml
	{Filetype: "renpy", Filename: "\\.rpy$"},                                            // renpy.yaml
	{Filetype: "rpmspec", Filename: "\\.spec$|\\.rpmspec$"},                             // rpmspec.yaml
	{Filetype: "ruby", Filename: "\\.(rb|rake|gemspec)$|^(.*[\\\\/])?(Gemfile|config\\.ru|Rakefile|Capfile|Vagrantfile|Guardfile|Appfile|Fastfile|Pluginfile|Podfile|\\.?[Bb]rewfile)$", Header: "^#!.*/(env +)?ruby( |$)"}, // ruby.yaml
	{Filetype: "rust", Filename: "\\.rs$"},                                      // rust.yaml
	{Filetype: "sage", Filename: "\\.sage$", Header: "^#!.*/(env +)?sage( |$)"}, // sage.yaml
	{Filetype: "OpenSCAD", Filename: "\\.scad$"},                                // scad.yaml
	{Filetype: "scala", Filename: "\\.sc(ala)?$|\\.sbt$"},                       // scala.yaml
	{Filetype: "sed", Filename: "\\.sed$", Header: "^#!.*bin/(env +)?sed( |$)"}, // sed.yaml
	{Filetype: "shell", Filename: "(\\.(sh|bash|ash|ebuild)$|(\\.bash(rc|_aliases|_functions|_profile)|\\.?profile|Pkgfile|pkgmk\\.conf|rc\\.conf|PKGBUILD|APKBUILD)$|bash-fc\\.)", Header: "^#!.*/(env +)?(ba)?(a)?(mk)?sh( |$)"}, // sh.yaml
	{Filetype: "salt", Filename: "\\.sls$"},                                               // sls.yaml
	{Filetype: "smalltalk", Filename: "\\.(st|sources|changes)$"},                         // smalltalk.yaml
	{Filetype: "solidity", Filename: "\\.sol$"},                                           // solidity.yaml
	{Filetype: "sql", Filename: "\\.sql$|sqliterc$"},                                      // sql.yaml
	{Filetype: "stata", Filename: "\\.a?do$"},                                             // stata.yaml
	{Filetype: "svelte", Filename: "\\.svelte$"},                                          // svelte.yaml
	{Filetype: "swift", Filename: "\\.swift$", Header: "^#!.*bin/(env +)?swift( |$)"},     // swift.yaml
	{Filetype: "systemd", Filename: "\\.(service|socket|timer)$", Header: "^\\[Unit\\]$"}, // systemd.yaml
	{Filetype: "tcl", Filename: "\\.tcl$", Header: "^#!.*/(env +)?tclsh( |$)"},            // tcl.yaml
	{Filetype: "terraform", Filename: "\\.tf$|\\.hcl$"},                                   // terraform.yaml
	{Filetype: "tex", Filename: "\\.tex$|\\.bib$|\\.cls$"},                                // tex.yaml
	{Filetype: "toml", Filename: "\\.toml"},                                               // toml.yaml
	{Filetype: "twig", Filename: "\\.twig$"},                                              // twig.yaml
	{Filetype: "typescript", Filename: "\\.tsx?$"},                                        // typescript.yaml
	{Filetype: "v"},                                                                       // v.yaml
	{Filetype: "vala", Filename: "\\.vala$"},                                              // vala.yaml
	{Filetype: "verilog", Filename: "\\.(v|vh|sv|svh)$"},                                  // verilog.yaml
	{Filetype: "vhdl", Filename: "\\.vhdl?$"},                                             // vhdl.yaml
	{Filetype: "vi", Filename: "(^|/|\\.)(ex|vim)rc$|\\.vim"},                             // vi.yaml
	{Filetype: "vue", Filename: "\\.vue$"},                                                // vue.yaml
	{Filetype: "xml", Filename: "\\.(xml|sgml?|rng|svg|plist)$", Header: "<\\?xml.*\\?>"}, // xml.yaml
	{Filetype: "xresources", Filename: "X(defaults|resources)$"},                          // xresources.yaml
	{Filetype: "yaml", Filename: "\\.ya?ml$", Header: "%YAML"},                            // yaml.yaml
	{Filetype: "yum", Filename: "\\.repo$|yum.*\\.conf$"},                                 // yum.yaml
	{Filetype: "zig", Filename: "\\.z(ig|on)$"},                                           // zig.yaml
	{Filetype: "zscript", Filename: "(?i)\\.z(c|sc)$"},                                    // zscript.yaml
	{Filetype: "zsh", Filename: "(\\.zsh$|\\.?(zshenv|zprofile|zshrc|zlogin|zlogout)$)", Header: "^#!.*/(env +)?zsh( |$)"}, // zsh.yaml
}
