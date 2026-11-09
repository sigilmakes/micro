# Colors

This help page covers colorschemes. Syntax highlighting is provided by
extensions; see `> help syntax` for the provider API.

## Colorschemes

To change your colorscheme, press `Ctrl-e` in micro to bring up the command
prompt, and type:

```
set colorscheme twilight
```

(or whichever colorscheme you choose).

Micro comes with a number of colorschemes by default. The colorschemes that you
can display will depend on what kind of color support your terminal has.

Omit color-link default "[fg color],[bg color]" will make the background color match the terminal's, and transparency if set.

Modern terminals tend to have a palette of 16 user-configurable colors (these
colors can often be configured in the terminal preferences), and additional
color support comes in three flavors.

* 16-color: A colorscheme that uses the 16 default colors will always work but
  will only look good if the 16 default colors have been configured to the
  user's liking. Using a colorscheme that only uses the 16 colors from the
  terminal palette will also preserve the terminal's theme from other
  applications since the terminal will often use those same colors for other
  applications. Default colorschemes of this type include `simple` and
  `solarized`.

* 256-color: Almost all terminals support displaying an additional 240 colors
  on top of the 16 user-configurable colors (creating 256 colors total).
  Colorschemes which use 256-color are portable because they will look the
  same regardless of the configured 16-color palette. However, the color
  range is fairly limited due to the small number of colors available.
  Default 256-color colorschemes include `monokai`, `twilight`, `zenburn`,
  `darcula` and more.

* true-color: Some terminals support displaying "true color" with 16 million
  colors using standard RGB values. This mode will be able to support
  displaying any colorscheme, but it should be noted that the user-configured
  16-color palette is ignored when using true-color mode (this means the
  colors while using the terminal emulator will be slightly off). Not all
  terminals support true color but at this point most do (see below).
  True-color colorschemes in micro typically end with `-tc`, such as
  `solarized-tc`, `atom-dark`, `material-tc`, etc... If true color is not
  enabled but a true color colorscheme is used, micro will do its best to
  approximate the colors to the available 256 colors.

Here is the list of colorschemes:

### 256 color

These should work and look nice in most terminals. I recommend these
themes the most.

* `monokai` (also the `default` colorscheme)
* `zenburn`
* `gruvbox`
* `darcula`
* `twilight`
* `railscast`
* `bubblegum` (light theme)

### 16 color

These may vary widely based on the 16 colors selected for your terminal.

* `simple`
* `solarized` (must have the solarized color palette in your terminal to use
   this colorscheme properly)
* `cmc-16`
* `geany`

### True color

Micro enables true color support by default as long as it detects that the
terminal supports it (which is usually indicated by the environment variable
`COLORTERM` being set to `truecolor`, `24bit` or `24-bit`). You can also force
enabling it unconditionally by setting the option `truecolor` to `on` (or
alternatively by setting the environment variable `MICRO_TRUECOLOR` to 1, which
is supported for backward compatibility).

* `solarized-tc`: this is the solarized colorscheme for true color.
* `atom-dark`: this colorscheme is based off of Atom's "dark" colorscheme.
* `cmc-tc`: A true colour variant of the cmc theme.  It requires true color to
   look its best. Use cmc-16 if your terminal doesn't support true color.
* `gruvbox-tc`: The true color version of the gruvbox colorscheme
* `material-tc`: Colorscheme based off of Google's Material Design palette

## Creating a Colorscheme

Micro's colorschemes are also extremely simple to create. The default ones can
be found
[here](https://github.com/micro-editor/micro/tree/master/runtime/colorschemes).

Custom colorschemes should be placed in the `~/.config/micro/colorschemes`
directory.

A number of custom directives are placed in a `.micro` file. Colorschemes are
typically only 18-30 lines in total.

To create the colorscheme you need to link highlight groups with
actual colors. This is done using the `color-link` command.

For example, to highlight all comments in green, you would use the command:

```
color-link comment "green"
```

Background colors can also be specified with a comma:

```
color-link comment "green,blue"
```

This will give the comments a blue background.

If you would like no foreground you can just use a comma with nothing in front:

```
color-link comment ",blue"
```

You can also put bold, blink, dim, italic, reverse, strikethrough and underline
in front of the color:

```
color-link comment "bold red"
```

And you can even combine different styles:

```
color-link line-number "italic underline black,yellow"
```

---

There are three different ways to specify the color.

Color terminals usually have 16 colors that are preset by the user. This means
that you cannot depend on those colors always being the same. You can use those
colors with the names `black, red, green, yellow, blue, magenta, cyan, white`
and the bright variants of each one (brightblack, brightred...).

Then you can use the terminals 256 colors by using their numbers 1-256 (numbers
1-16 will refer to the named colors).

If the user's terminal supports true color, then you can also specify colors
exactly using their hex codes. If the terminal is not true color but micro is
told to use a true color colorscheme it will attempt to map the colors to the
available 256 colors.

Generally colorschemes which require true color terminals to look good are
marked with a `-tc` suffix.

---

Here is a list of the colorscheme groups that you can use:

* default (color of the background and foreground for unhighlighted text)
* comment
* identifier
* constant
* statement
* symbol
* preproc
* type
* special
* underlined
* error
* todo
* selection (Color of the text selection)
* statusline (Color of the statusline)
* statusline.inactive (Color of the statusline of inactive split panes)
* statusline.suggestions (Color of the autocomplete suggestions menu)
* tabbar (Color of the tabbar that lists open files)
* tabbar.active (Color of the active tab in the tabbar)
* indent-char (Color of the character which indicates tabs if the option is
  enabled)
* line-number
* gutter-info
* gutter-error
* gutter-warning
* diff-added
* diff-modified
* diff-deleted
* cursor-line
* current-line-number
* color-column
* ignore
* scrollbar
* divider (Color of the divider between vertical splits)
* message (Color of messages in the bottom line of the screen)
* error-message (Color of error messages in the bottom line of the screen)
* match-brace (Color of matching brackets when `matchbracestyle` is set to `highlight`)
* hlsearch (Color of highlighted search results when `hlsearch` is enabled)
* tab-error (Color of tab vs space errors when `hltaberrors` is enabled)
* trailingws (Color of trailing whitespaces when `hltrailingws` is enabled)

Colorschemes must be placed in the `~/.config/micro/colorschemes` directory to
be used.

---

In addition to the main colorscheme groups, there are subgroups that you can
specify by adding `.subgroup` to the group. Syntax providers can use their own
subgroups.

If micro can't match the subgroup, it'll default to the root group, so it's
safe and recommended to use subgroups in providers.

For example if `constant.string` is found in your colorscheme, micro will us
that for highlighting strings. If it's not found, it will use constant instead.
Micro tries to match the largest set of groups it can find in the colorscheme
definitions, so if, for example `constant.bool.true` is found then micro will
use that. If `constant.bool.true` is not found but `constant.bool` is found
micro will use `constant.bool`. If not, it uses `constant`.

Here's a list of subgroups commonly used by syntax providers.

* comment.bright (Some filetypes have distinctions between types of comments)
* constant.bool
* constant.bool.true
* constant.bool.false
* constant.number
* constant.specialChar
* constant.string
* constant.string.url
* identifier.class (Also used for functions)
* identifier.macro
* identifier.var
* preproc.shebang (The #! at the beginning of a file that tells the os what
  script interpreter to use)
* symbol.brackets (`{}()[]` and sometimes `<>`)
* symbol.operator (Color operator symbols differently)
* symbol.tag (For html tags, among other things)
* type.keyword (If you want a special highlight for keywords like `private`)

In the future, plugins may also be able to use color groups for styling.

---

Last but not least it's even possible to use `include` followed by the
colorscheme name as string to include a different colorscheme within a new one.
Additionally the groups can then be extended or overwritten. The `default.micro`
theme can be seen as an example, which links to the chosen default colorscheme.

## Syntax highlighting

Syntax highlighting is supplied by extensions through the `micro/syntax` Lua
module. See `> help syntax` for the provider API, span units, matching rules,
and the asynchronous reply contract.

Micro still detects filetypes without any provider, so `filetype` settings,
`ft:` options, the status line, and plugins such as `comment` and `linter`
continue to work. A file with no provider simply has no highlighting.
