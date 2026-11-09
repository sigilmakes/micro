# micro-treesitter

An out-of-process tree-sitter highlighter for the micro fork's
`micro/syntax` provider API. The Go helper parses a document, runs a
tree-sitter highlight query, and returns single-line byte spans. The Lua
plugin owns the helper process and forwards spans to the editor.

This is the first provider extension for the fork: it keeps cgo, grammars,
queries, and capture names out of the editor core. Removing the plugin and
the helper removes tree-sitter entirely.

## Layout

| Path | Purpose |
|---|---|
| `cmd/micro-treesitter/` | helper binary: stdin/stdout protocol loop |
| `internal/protocol/` | frame reader/writer |
| `internal/tsprovider/` | grammars, queries, parse sessions, span flattening |
| `queries/go/highlights.scm` | Go highlight query, copied from tree-sitter-go v0.25.0 (MIT) |
| `runtime/plugins/treesitter/` | Lua plugin and `repo.json` |
| `testdata/` | Go sample, standalone helper driver, Lua harness |

## Build and test

The helper uses cgo through `github.com/tree-sitter/go-tree-sitter` and the
official Go grammar binding:

```sh
nix shell nixpkgs#go_1_27 -c bash -lc 'CGO_ENABLED=1 go build ./cmd/micro-treesitter'
nix shell nixpkgs#go_1_27 -c bash -lc 'CGO_ENABLED=1 go test ./...'
```

A C compiler must be available to cgo. On this machine the build was also
verified with `nix shell nixpkgs#go_1_27 nixpkgs#gcc`; `CGO_CFLAGS=-O2` is
optional and only affects parse throughput.

Drive the helper directly:

```sh
python3 testdata/demo.py ./micro-treesitter testdata/sample.go go
```

Check the Lua state machine without micro:

```sh
lua5_1 testdata/lua_harness.lua
```

## Grammar and query lookup

The helper reads `<grammar-dir>/<grammar>/highlights.scm`. The grammar
directory is resolved in this order:

1. `--grammar-dir` flag
2. `MICRO_TS_GRAMMAR_DIR`
3. `./queries`

`<grammar-dir>/<grammar>.scm` and `<grammar-dir>/highlights.scm` are accepted
as fallbacks, so a single-grammar directory also works. A missing or invalid
query becomes an `ERR` reply for that filetype; the helper keeps running.

Only the Go grammar is linked in this version. `filetype` is mapped to a
grammar through `internal/tsprovider.filetypeGrammar`; adding a language means
adding its Go module to `go.mod`, one `grammarLanguages` entry, one mapping
entry, a query directory, and a second Lua `syntax.register` call.

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
subgroups). Unknown captures produce no span. The main mappings:

| tree-sitter capture | micro group |
|---|---|
| `comment*` | `comment` |
| `string*`, `character*` | `constant.string` |
| `string.escape`, `escape` | `constant.specialChar` |
| `number*` | `constant.number` |
| `constant`, `constant.macro` | `constant` |
| `constant.builtin`, `boolean` | `constant.bool` |
| `keyword*` | `statement` (Go keywords are refined below) |
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

The Go grammar emits one generic `@keyword` capture for every keyword. The
helper refines the keyword text using the old `go.yaml` vocabulary: `package`,
`import`, `const`, `var`, `type`, `struct`, `func`, `interface`, `map`,
`chan`, `defer` become `preproc`; `break`, `case`, `continue`, `default`,
`go`, `goto`, `range`, `return`, `fallthrough` become `special`; `else`,
`for`, `if`, `switch`, `select` stay `statement`.

## Lua plugin

`runtime/plugins/treesitter/treesitter.lua` registers one provider:

```lua
{
    name = "treesitter-go",
    filetype = "go",
    filename = "\\.go$",
    priority = 100,
    request = request,   -- request(ctx, reply)
}
```

On `init` it spawns `$MICRO_TS` if set, otherwise `micro-treesitter` from
`PATH`. It logs once and disables highlighting when the helper exits before
serving requests. On `deinit` it stops the job and drops pending replies.

The plugin keeps one helper process for all buffers and routes replies by
`ctx.id`. A pending reply is stored per id with its generation; a `SPANS` or
`ERR` frame is delivered only when the generation matches. Chunk boundaries
from the process pipe are arbitrary; the plugin buffers stdout and reassembles
frames line by line.

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
0	0	7	preproc
2	0	42	comment
3	0	16	comment
4	0	17	comment
5	0	11	comment
6	0	4	preproc
6	5	11	type
6	12	18	preproc
7	1	4	identifier
7	6	12	type
7	13	35	constant.string
11	0	4	preproc
...
16	1	7	special
...
```

`Héllo` (6 bytes, 5 runes) is reported as byte span `[5,11)`, and both the
three-line block comment starting at line 3 and the raw string on lines 12-14
are split across their lines.

## Deliberate choices and deviations

- `ERR` replies with a matching generation call `reply({})` after logging, so
  a failed refresh cannot leave spans from a previous generation on screen.
- `ctx.id` is used as the numeric request key. If a core build omits it, the
  plugin derives a stable numeric id per buffer handle; that keeps the helper
  usable but is intended only as a compatibility fallback.
- A malformed request header is answered with `ERR` and the helper exits,
  because the unread payload length is unknown. The plugin never sends one.
- The helper reparses incrementally even though v1 permits a full reparse on
  every request. A differential test compares incremental spans with a fresh
  parse over random edits, including UTF-8 and invalid UTF-8 bytes.
- Request handling is sequential and has no timeout. A helper that hangs will
  eventually block `JobSend` in the editor. Process supervision beyond the
  exit callback (restart with backoff, watchdog) is not implemented.
- `queries/go/highlights.scm` and `queries/go/LICENSE` are copied from
  tree-sitter-go v0.25.0.

## Known gaps

- Only Go is linked. Other filetypes get an `ERR`, and the plugin only
  registers `go`.
- Queries run over the whole document; there is no windowed query, no
  debounce policy in the plugin (core owns the request cadence), and no
  injections or embedded languages.
- One helper serves all buffers. A helper crash disables highlighting until
  the plugin is reloaded.
- Spans are generated from the whole tree on every request; the previous tree
  only saves reparsing, not re-querying.
- No grammar manifest. Distribution must provide `queries/<grammar>/highlights.scm`
  and set `MICRO_TS_GRAMMAR_DIR`; there is no compiled default path.
