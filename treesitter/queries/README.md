# Tree-sitter queries

Each directory holds the upstream `queries/highlights.scm` for one linked
grammar, copied verbatim, plus the upstream license. `tsx/` and `cpp/` also
carry the base language they are layered over, because the grammars' own
queries only cover dialect-specific patterns.

| Directory | Source | Tag | License | Notes |
|---|---|---|---|---|
| `go/` | [tree-sitter-go](https://github.com/tree-sitter/tree-sitter-go) | `v0.25.0` | MIT | |
| `python/` | [tree-sitter-python](https://github.com/tree-sitter/tree-sitter-python) | `v0.25.0` | MIT | |
| `javascript/` | [tree-sitter-javascript](https://github.com/tree-sitter/tree-sitter-javascript) | `v0.25.0` | MIT | |
| `tsx/` | [tree-sitter-typescript](https://github.com/tree-sitter/tree-sitter-typescript) | `v0.23.2` | MIT | `highlights.scm` is the TypeScript query. `javascript.scm` and `LICENSE.javascript` are copied from tree-sitter-javascript `v0.25.0`; the helper runs them before the TypeScript query. |
| `rust/` | [tree-sitter-rust](https://github.com/tree-sitter/tree-sitter-rust) | `v0.24.2` | MIT | |
| `c/` | [tree-sitter-c](https://github.com/tree-sitter/tree-sitter-c) | `v0.24.2` | MIT | |
| `cpp/` | [tree-sitter-cpp](https://github.com/tree-sitter/tree-sitter-cpp) | `v0.23.4` | MIT | `highlights.scm` is the C++ query. `c.scm` and `LICENSE.c` are copied from tree-sitter-c `v0.24.2`; the helper runs them before the C++ query. |
| `bash/` | [tree-sitter-bash](https://github.com/tree-sitter/tree-sitter-bash) | `v0.25.1` | MIT | |
| `json/` | [tree-sitter-json](https://github.com/tree-sitter/tree-sitter-json) | `v0.24.8` | MIT | |
| `toml/` | [tree-sitter-toml](https://github.com/tree-sitter-grammars/tree-sitter-toml) | `v0.7.0` | MIT | Community fork with Go bindings. |
| `html/` | [tree-sitter-html](https://github.com/tree-sitter/tree-sitter-html) | `v0.23.2` | MIT | |
| `css/` | [tree-sitter-css](https://github.com/tree-sitter/tree-sitter-css) | `v0.25.0` | MIT | |
| `lua/` | [tree-sitter-lua](https://github.com/tree-sitter-grammars/tree-sitter-lua) | `v0.5.0` | MIT | Upstream license file is `LICENSE.md`; copied as `LICENSE`. |
| `yaml/` | [tree-sitter-yaml](https://github.com/tree-sitter-grammars/tree-sitter-yaml) | `v0.7.2` | MIT | |

The query files are unmodified. Update a directory and its module version
together; the Go module pins live in `../go.mod`.
