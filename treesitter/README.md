# micro-treesitter

An out-of-process tree-sitter highlighter for the micro fork's
`micro/syntax` provider API. The Go helper parses a document, runs one or
more tree-sitter highlight queries, and returns single-line byte spans. The
Lua plugin owns the helper process and forwards spans to the editor.

Fifteen micro filetypes are linked: Go, Python, JavaScript, TypeScript,
Rust, C, C++, shell, JSON, TOML, HTML, CSS, Lua, YAML, and Nix. Grammars and
queries are data in a registry, so adding a language does not change the
parse, span, or protocol code.

This is the first provider extension for the fork: it keeps cgo, grammars,
queries, and capture names out of the editor core. Removing the plugin and
the helper removes tree-sitter entirely.

## Layout

| Path | Purpose |
|---|---|
| `cmd/micro-treesitter/` | helper binary: stdin/stdout protocol loop |
| `internal/protocol/` | frame reader/writer |
| `internal/tsprovider/` | grammar registry, parse sessions, span flattening |
| `internal/grammars/nix/` | cgo shim for the vendored Nix parser |
| `third_party/tree-sitter-nix/` | Nix parser C source and license |
| `queries/<grammar>/` | upstream `highlights.scm` files and licenses |
| `runtime/plugins/treesitter/` | Lua plugin and `repo.json` |
| `testdata/` | samples, standalone helper driver, Lua harness |

## Build and test

The helper uses cgo through `github.com/tree-sitter/go-tree-sitter` and the
grammars' Go bindings:

```sh
nix shell nixpkgs#go_1_27 -c bash -lc 'CGO_ENABLED=1 go build ./...'
nix shell nixpkgs#go_1_27 -c bash -lc 'CGO_ENABLED=1 go test ./...'
nix shell nixpkgs#go_1_27 -c bash -lc 'CGO_ENABLED=1 go vet ./...'
```

A C compiler must be available to cgo. On this machine the build was also
verified with `nix shell nixpkgs#go_1_27 nixpkgs#gcc`; `CGO_CFLAGS=-O2` is
optional and only affects parse throughput.

Build the helper binary at the module root:

```sh
nix shell nixpkgs#go_1_27 -c bash -lc 'CGO_ENABLED=1 go build -o micro-treesitter ./cmd/micro-treesitter'
```

Drive the helper directly:

```sh
python3 testdata/demo.py ./micro-treesitter testdata/sample.go go
```

Check the Lua state machine without micro:

```sh
lua5_1 testdata/lua_harness.lua
```

## Grammar registry

`internal/tsprovider.definitions` is the registry. Each `Definition` carries
a grammar id, the micro filetypes it serves, its query file names, and a
language constructor. A language is linked by adding its Go module to
`go.mod` (or vendoring its C source), one definition, and a query directory.

| micro filetype | grammar id | module | query layers |
|---|---|---|---|
| `go` | `go` | `tree-sitter-go` | `highlights.scm` |
| `python` | `python` | `tree-sitter-python` | `highlights.scm` |
| `javascript` | `javascript` | `tree-sitter-javascript` | `highlights.scm` |
| `typescript` | `tsx` | `tree-sitter-typescript` | `javascript.scm`, `highlights.scm` |
| `rust` | `rust` | `tree-sitter-rust` | `highlights.scm` |
| `c` | `c` | `tree-sitter-c` | `highlights.scm` |
| `c++` | `cpp` | `tree-sitter-cpp` | `c.scm`, `highlights.scm` |
| `shell` | `bash` | `tree-sitter-bash` | `highlights.scm` |
| `json` | `json` | `tree-sitter-json` | `highlights.scm` |
| `toml` | `toml` | `tree-sitter-toml` | `highlights.scm` |
| `html` | `html` | `tree-sitter-html` | `highlights.scm` |
| `css` | `css` | `tree-sitter-css` | `highlights.scm` |
| `lua` | `lua` | `tree-sitter-lua` | `highlights.scm` |
| `yaml` | `yaml` | `tree-sitter-yaml` | `highlights.scm` |
| `nix` | `nix` | `tree-sitter-nix` (vendored C) | `highlights.scm` |

Micro's `typescript` filetype matches both `.ts` and `.tsx`, so the TSX
grammar serves it. TSX parses plain TypeScript too, and one grammar keeps
buffer switching from reloading a second parser. The TypeScript query only
covers TypeScript-specific patterns, so the registry runs the JavaScript
query first; the C++ registry entry does the same with the C query. This
mirrors nvim-treesitter's inheritance without a query preprocessor.
`queries/README.md` records every upstream repository, tag, and license.

## Query lookup

The helper reads the files listed by the definition from
`<grammar-dir>/<grammar>/<name>` and concatenates them in order. The grammar
directory is resolved in this order:

1. `--grammar-dir` flag
2. `MICRO_TS_GRAMMAR_DIR`
3. `./queries`

For `highlights.scm`, `<grammar-dir>/<grammar>.scm` and
`<grammar-dir>/highlights.scm` are accepted as fallbacks, so a
single-grammar directory also works. A missing or invalid query becomes an
`ERR` reply for that filetype; the helper keeps running.

## Wire protocol

Requests and replies are newline-framed headers followed by an exact byte
payload or a known number of lines. All integers are decimal. Fields are
separated by one tab. Text is UTF-8 but is never decoded into runes.

Request:

```text
REQ\t<id>\t<gen>\t<filetype>\t<byte_len>\n
<exactly byte_len bytes of document text>
```

Reply:

```text
SPANS\t<id>\t<gen>\t<count>\n
<line>\t<startByte>\t<endByte>\t<group>\n      (count lines)
```

Error:

```text
ERR\t<id>\t<gen>\t<message>\n
```

`line` is 0-based. `startByte` and `endByte` are 0-based byte offsets within
that line, end exclusive. Lines are `\n`-separated in the payload and contain
no line terminator.

The helper keeps parse state per `id`: the previous text and tree are reused
through a single contiguous `InputEdit` computed from the common prefix and
suffix, then reparsed. If tree-sitter declines to produce a tree, the helper
falls back to a full parse. Sessions are evicted least-recently-used beyond
16 ids. Parse sessions are not shared across ids, and requests are processed
sequentially.

## Span construction

- Multi-line nodes are split per line. The first line runs from the node's
  start byte to the end of the line; middle lines are covered fully; the last
  line runs from byte 0 to the node's end column. A node that ends at column 0
  emits nothing on that line.
- Empty and zero-width spans are dropped.
- Overlapping captures are painted in precedence order. Later captures win,
  except for identical ranges: the more specific capture name wins, then the
  earlier pattern. This keeps `@function.builtin` over `@function`, and
  `@function` over a later `@variable` capture of the same node.
- Adjacent spans with the same group are merged.
- Output is sorted by line, then start byte.

Capture names are resolved by longest dotted prefix through
`internal/tsprovider.captureGroups`. The groups emitted are limited to names
that micro colorschemes define (`comment`, `identifier`, `constant`,
`statement`, `symbol`, `preproc`, `type`, `special`, `error`, `todo` and their
subgroups). Unknown captures produce no span. The mapping is shared by every
language; there are no per-language keyword tables. The main mappings:

| tree-sitter capture | micro group |
|---|---|
| `comment*` | `comment` |
| `string*`, `character*` | `constant.string` |
| `string.escape`, `escape` | `constant.specialChar` |
| `number*` | `constant.number` |
| `constant`, `constant.macro` | `constant` |
| `constant.builtin`, `boolean` | `constant.bool` |
| `keyword*` | `statement` |
| `keyword.function/type/import/package/declaration` | `preproc` |
| `function*`, `variable*`, `parameter`, `property`, `identifier` | `identifier` |
| `function.builtin`, `function.macro` | `preproc` |
| `type*`, `constructor`, `module`, `namespace`, `attribute` | `type` |
| `operator` | `symbol.operator` |
| `punctuation.bracket` | `symbol.brackets` |
| `punctuation*` | `symbol` |
| `preproc` | `preproc` |
| `tag` | `symbol.tag` |
| `error` | `error` |
| `todo`, `comment.todo`, `comment.warning` | `todo` |

## Lua plugin

`runtime/plugins/treesitter/treesitter.lua` registers one provider per
filetype from a table:

```lua
{ name = "treesitter-go", filetype = "go", filename = [[\.go$]], priority = 100, request = request }
```

All providers share one request function and one helper process. The C++
entry also carries a signature regexp so content-based detection still
picks C++ over C for ambiguous headers. A filetype the helper does not link
answers `ERR`; the plugin logs that filetype once and clears its spans.

On `init` the plugin spawns `$MICRO_TS` if set, otherwise `micro-treesitter`
from `PATH`. It logs once and disables highlighting when the helper exits
before serving requests. On `deinit` it stops the job and drops pending
replies.

The plugin routes replies by `ctx.id`. A pending reply is stored per id with
its generation; a `SPANS` or `ERR` frame is delivered only when the
generation matches. Chunk boundaries from the process pipe are arbitrary;
the plugin buffers stdout and reassembles frames line by line.

Install the helper on `PATH` through your package manager, or set:

```sh
export MICRO_TS=/path/to/micro-treesitter
export MICRO_TS_GRAMMAR_DIR=/path/to/grammar-bundle
```

Copy `runtime/plugins/treesitter/` into `$MICRO_CONFIG_HOME/plug/` (or ship it
in the editor's embedded runtime plugins). The plugin directory must contain
only `treesitter.lua` and `repo.json`; micro executes every `.lua` file it
finds there.

The plugin assumes the core provides `micro/syntax` with
`register{...}` and `request(ctx, reply)`, and that `ctx` carries `id`,
`generation`, `text`, `filetype`, `buf`, `path`, `first`, `last`, and `cause`.
`ctx.id` must be an integer that is stable for a buffer: the helper parses it
as the decimal request key. `MICRO_TS` is one executable path; arguments are
not split, so use a wrapper script if extra flags are needed.

## Tested sample

`testdata/sample.go` exercises a multi-line block comment, a backtick raw
string, a struct tag, and UTF-8 identifiers. The helper's reply begins:

```text
SPANS	1	1	39
0	0	7	statement
2	0	42	comment
3	0	16	comment
4	0	17	comment
5	0	11	comment
6	0	4	statement
6	5	11	type
6	12	18	statement
7	1	4	identifier
7	6	12	type
7	13	35	constant.string
...
```

`Héllo` (6 bytes, 5 runes) is reported as byte span `[5,11)`, and both the
three-line block comment starting at line 3 and the raw string on lines 12-14
are split across their lines.

## Deliberate choices and deviations

- `typescript` uses the TSX grammar because micro does not distinguish `.ts`
  from `.tsx`. `c++` uses `cpp` and `shell` uses `bash`, matching the
  filetype table.
- The Go keyword vocabulary was removed. The old `go.yaml` split declarations
  from control flow; the generic `keyword*` mapping now paints all upstream
  `@keyword` captures as `statement`. Per-language keyword tables would have
  to be maintained for fourteen grammars.
- Query inheritance is expressed as ordered query files rather than a
  `; inherits:` preprocessor. The copied files remain verbatim.
- `ERR` replies with a matching generation call `reply({})` after logging
  once per filetype, so a failed refresh cannot leave spans from a previous
  generation on screen.
- `ctx.id` is used as the numeric request key. If a core build omits it, the
  plugin derives a stable numeric id per buffer handle; that keeps the helper
  usable but is intended only as a compatibility fallback.
- A malformed request header is answered with `ERR` and the helper exits,
  because the unread payload length is unknown. The plugin never sends one.
- The helper reparses incrementally even though v1 permits a full reparse on
  every request. Differential tests compare incremental spans with a fresh
  parse over random edits for Go and Python, including UTF-8 and invalid
  UTF-8 bytes.
- Request handling is sequential and has no timeout. A helper that hangs will
  eventually block `JobSend` in the editor. Process supervision beyond the
  exit callback (restart with backoff, watchdog) is not implemented.

## Injections

The HTML grammar ships upstream `injections.scm`. The helper parses
`<script>` content with the JavaScript grammar and `<style>` content with the
CSS grammar, then overlays those spans on the HTML tree. Injection depth is
limited to two so a malformed grammar cannot recurse forever.

## Known gaps

- Only HTML injections are wired. Markdown fenced code blocks, JavaScript
  template substitutions, and other embedded-language hosts are not parsed
  with a second grammar yet.
- Grammars are linked at build time. There is no dynamic parser loading,
  plugin grammar registry, or runtime download.
- Queries run over the whole document; there is no viewport or windowed
  query, and the previous tree only saves reparsing, not re-querying.
- The plugin has no debounce policy (core owns the request cadence) and no
  per-provider failure recovery beyond the helper's process exit callback.
- One helper serves all buffers. A helper crash disables highlighting until
  the plugin is reloaded.
- There is no runtime grammar manifest. The flake bakes a default grammar
  directory into the helper; a plain `go build` falls back to `./queries`, and
  `MICRO_TS_GRAMMAR_DIR` overrides both.
