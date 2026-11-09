# Syntax highlighting

Micro no longer ships a built-in regex engine. Syntax highlighting is
provided by extensions through the `micro/syntax` Lua module. A file with no
provider is not an error: it opens normally with its filetype detected and no
colors.

## Provider contract

A provider registers a matcher and a `request(ctx, reply)` function:

```lua
local syntax = import("micro/syntax")

syntax.register {
    name = "my-provider",          -- required
    filetype = "go",               -- exact match against the buffer filetype
    filename = "\\.go$",           -- optional Go regexp against the full path
    header = "^package ",          -- optional Go regexp against line 0
    signature = "^// Code generated", -- optional Go regexp for tie-breaking
    priority = 10,                 -- higher wins; default 0
    request = function(ctx, reply)
        -- reply takes a flat array of line, startByte, endByte, group
        reply {
            0, 2, 7, "preproc",
            1, 4, 9, "constant.string",
        }
    end,
}
```

`syntax.unregister(name)` removes a provider. `syntax.invalidate(buf)` asks for
fresh spans. `syntax.has_color(name)` reports whether the current colorscheme
defines a group or one of its dotted prefixes. `syntax.providers()` returns the
registered names.

## Context

`ctx` contains:

| field | meaning |
|---|---|
| `buf` | the `Buffer` |
| `id` | stable numeric id shared by split views of one buffer |
| `text` | the whole buffer as UTF-8, lines joined with `\n` |
| `path` | the buffer path |
| `filetype` | the detected or explicitly set filetype |
| `first`, `last` | inclusive line range; currently the whole buffer |
| `generation` | monotonic counter for stale-reply checks |
| `cause` | `"open"`, `"edit"`, `"manual"`, or `"reload"` |

## Spans

A span is four values: zero-based line, zero-based **byte** start, exclusive
byte end, and a group name. Spans must stay on one line. Convert multi-line
constructs by emitting one span per line. Later spans win over earlier ones;
an empty group name clears its range. Offsets are bytes, not rune or display
columns, so UTF-8 and combining marks map correctly.

A reply replaces every span in the buffer. Lines the reply does not mention
are cleared. Groups are looked up by `config.GetColor`, which already handles
dotted prefixes such as `constant.string` falling back to `constant`.

## Matching

An explicit `filetype` setting wins. Otherwise micro detects the filetype from
its generated filename/header/signature table. A provider for that filetype is
chosen by priority. If the filetype is unknown, providers are matched by
filename first, then first line, with signatures breaking ties, then priority.
Detection still works with no provider registered.

## Async replies

`request` and `reply` run on the main goroutine. A provider may keep `reply`
and call it later, for example from `micro.After` or a `micro/shell` job
callback. Edits are debounced (~25 ms), and at most one request is in flight
per buffer. Replies for an older generation are dropped. A provider that
errors three times is disabled.
