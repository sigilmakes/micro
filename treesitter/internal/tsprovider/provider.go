// Package tsprovider turns tree-sitter highlight queries into micro spans.
//
// The package owns one parser and one compiled query per filetype, plus a
// per-request-id parse session. Parse sessions are reused with a single
// contiguous InputEdit derived from the common prefix and suffix of the old
// and new document; a failed incremental parse falls back to a full parse.
package tsprovider

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unsafe"

	tslua "github.com/tree-sitter-grammars/tree-sitter-lua/bindings/go"
	tstoml "github.com/tree-sitter-grammars/tree-sitter-toml/bindings/go"
	tsyaml "github.com/tree-sitter-grammars/tree-sitter-yaml/bindings/go"
	sitter "github.com/tree-sitter/go-tree-sitter"
	tsbash "github.com/tree-sitter/tree-sitter-bash/bindings/go"
	tsc "github.com/tree-sitter/tree-sitter-c/bindings/go"
	tscpp "github.com/tree-sitter/tree-sitter-cpp/bindings/go"
	tscss "github.com/tree-sitter/tree-sitter-css/bindings/go"
	tsgo "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tshtml "github.com/tree-sitter/tree-sitter-html/bindings/go"
	tsjavascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tsjson "github.com/tree-sitter/tree-sitter-json/bindings/go"
	tspython "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tsrust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
	tstypescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

// Span is one half-open, single-line byte range. Line is 0-based; Start and
// End are byte offsets within that line, End exclusive.
type Span struct {
	Line  int
	Start int
	End   int
	Group string
}

// Definition describes one linked grammar. Queries names files under
// <grammarDir>/<ID>/ and is concatenated in order, so a dialect can layer its
// own patterns over a base language. Filetypes are the micro filetypes from
// pkg/filetype/table.go that this grammar serves.
type Definition struct {
	ID         string
	Filetypes  []string
	Queries    []string
	Injections []string
	Language   func() *sitter.Language
}

// definitions is the linked grammar registry. Adding a language means adding
// its module to go.mod, one entry here, and its query directory.
var definitions = []Definition{
	{ID: "go", Filetypes: []string{"go"}, Queries: []string{"highlights.scm"}, Language: language(tsgo.Language)},
	{ID: "python", Filetypes: []string{"python"}, Queries: []string{"highlights.scm"}, Language: language(tspython.Language)},
	{ID: "javascript", Filetypes: []string{"javascript"}, Queries: []string{"highlights.scm"}, Language: language(tsjavascript.Language)},
	// micro's "typescript" filetype covers both .ts and .tsx, so the TSX
	// grammar serves it. TSX queries are layered over the JavaScript base.
	{ID: "tsx", Filetypes: []string{"typescript"}, Queries: []string{"javascript.scm", "highlights.scm"}, Language: language(tstypescript.LanguageTSX)},
	{ID: "rust", Filetypes: []string{"rust"}, Queries: []string{"highlights.scm"}, Language: language(tsrust.Language)},
	{ID: "c", Filetypes: []string{"c"}, Queries: []string{"highlights.scm"}, Language: language(tsc.Language)},
	{ID: "cpp", Filetypes: []string{"c++"}, Queries: []string{"c.scm", "highlights.scm"}, Language: language(tscpp.Language)},
	{ID: "bash", Filetypes: []string{"shell"}, Queries: []string{"highlights.scm"}, Language: language(tsbash.Language)},
	{ID: "json", Filetypes: []string{"json"}, Queries: []string{"highlights.scm"}, Language: language(tsjson.Language)},
	{ID: "toml", Filetypes: []string{"toml"}, Queries: []string{"highlights.scm"}, Language: language(tstoml.Language)},
	{ID: "html", Filetypes: []string{"html"}, Queries: []string{"highlights.scm"}, Injections: []string{"injections.scm"}, Language: language(tshtml.Language)},
	{ID: "css", Filetypes: []string{"css"}, Queries: []string{"highlights.scm"}, Language: language(tscss.Language)},
	{ID: "lua", Filetypes: []string{"lua"}, Queries: []string{"highlights.scm"}, Language: language(tslua.Language)},
	{ID: "yaml", Filetypes: []string{"yaml"}, Queries: []string{"highlights.scm"}, Language: language(tsyaml.Language)},
}

func language(constructor func() unsafe.Pointer) func() *sitter.Language {
	return func() *sitter.Language { return sitter.NewLanguage(constructor()) }
}

func definitionForFiletype(filetype string) (Definition, bool) {
	for _, definition := range definitions {
		for _, candidate := range definition.Filetypes {
			if candidate == filetype {
				return definition, true
			}
		}
	}
	return Definition{}, false
}

// captureGroups maps a tree-sitter capture name to a micro highlight group.
// Names are matched exactly first, then by dropping the last dotted
// component. Only group names whose first component exists in micro
// colorschemes (comment, identifier, constant, statement, symbol, preproc,
// type, special, error, todo) are emitted; config.GetColor falls back to the
// default style for anything else.
var captureGroups = map[string]string{
	"comment":               "comment",
	"comment.documentation": "comment",
	"comment.error":         "error",
	"comment.warning":       "todo",
	"comment.todo":          "todo",
	"todo":                  "todo",

	"string":             "constant.string",
	"string.escape":      "constant.specialChar",
	"string.special":     "constant.specialChar",
	"string.special.key": "constant.string",
	"string.regexp":      "constant.string",
	"escape":             "constant.specialChar",
	"character":          "constant.string",
	"character.special":  "constant.specialChar",

	"number":         "constant.number",
	"number.float":   "constant.number",
	"number.integer": "constant.number",
	"float":          "constant.number",

	"boolean":          "constant.bool",
	"constant":         "constant",
	"constant.builtin": "constant.bool",
	"constant.bool":    "constant.bool",
	"constant.macro":   "constant",

	"keyword":             "statement",
	"keyword.return":      "statement",
	"keyword.conditional": "statement",
	"keyword.control":     "statement",
	"keyword.exception":   "statement",
	"keyword.repeat":      "statement",
	"keyword.coroutine":   "statement",
	"keyword.function":    "preproc",
	"keyword.type":        "preproc",
	"keyword.import":      "preproc",
	"keyword.package":     "preproc",
	"keyword.declaration": "preproc",
	"keyword.operator":    "symbol.operator",

	"function":         "identifier",
	"function.call":    "identifier",
	"function.method":  "identifier",
	"function.builtin": "preproc",
	"function.macro":   "preproc",

	"type":           "type",
	"type.builtin":   "type",
	"type.qualifier": "statement",
	"constructor":    "type",
	"module":         "type",
	"namespace":      "type",
	"attribute":      "type",
	"label":          "statement",

	"property":           "identifier",
	"variable":           "identifier",
	"variable.builtin":   "identifier",
	"variable.parameter": "identifier",
	"parameter":          "identifier",
	"identifier":         "identifier",

	"operator":              "symbol.operator",
	"punctuation":           "symbol",
	"punctuation.bracket":   "symbol.brackets",
	"punctuation.delimiter": "symbol",
	"punctuation.special":   "symbol",
	"symbol":                "symbol",
	"tag":                   "symbol.tag",
	"preproc":               "preproc",
	"error":                 "error",
}

// MapCapture resolves a capture name to a micro group, or "" when the
// capture is unknown. Longest dotted prefix wins.
func MapCapture(name string) string {
	for name != "" {
		if group, ok := captureGroups[name]; ok {
			return group
		}
		i := strings.LastIndexByte(name, '.')
		if i < 0 {
			break
		}
		name = name[:i]
	}
	return ""
}

type grammar struct {
	id                string
	parser            *sitter.Parser
	language          *sitter.Language
	query             *sitter.Query
	captures          []string
	injectionQuery    *sitter.Query
	injectionCaptures []string
}

func (g *grammar) close() {
	if g.injectionQuery != nil {
		g.injectionQuery.Close()
	}
	if g.query != nil {
		g.query.Close()
	}
	if g.parser != nil {
		g.parser.Close()
	}
}

type session struct {
	filetype string
	text     []byte
	tree     *sitter.Tree
	used     uint64
}

func (s *session) closeTree() {
	if s.tree != nil {
		s.tree.Close()
		s.tree = nil
	}
}

// Engine owns grammars and parse sessions. It is not safe for concurrent use.
type Engine struct {
	grammarDir  string
	grammars    map[string]*grammar
	failures    map[string]error
	sessions    map[int64]*session
	maxSessions int
	tick        uint64
}

// New returns an engine that loads each grammar's query files from
// <grammarDir>/<grammar>/<name>.
func New(grammarDir string) *Engine {
	return &Engine{
		grammarDir:  grammarDir,
		grammars:    make(map[string]*grammar),
		failures:    make(map[string]error),
		sessions:    make(map[int64]*session),
		maxSessions: 16,
	}
}

// Close releases all parsers, queries, and cached trees.
func (e *Engine) Close() {
	for _, g := range e.grammars {
		g.close()
	}
	for _, s := range e.sessions {
		s.closeTree()
	}
	e.grammars = nil
	e.sessions = nil
}

// Spans reparses text for the request id and returns flattened micro spans.
// id identifies the parse session (normally one buffer); filetype selects
// the grammar and query.
func (e *Engine) Spans(id int64, filetype string, text []byte) ([]Span, error) {
	g, err := e.grammarFor(filetype)
	if err != nil {
		return nil, err
	}
	s := e.touchSession(id)
	tree, err := e.parse(g, s, filetype, text)
	if err != nil {
		return nil, err
	}
	return e.withInjections(g, tree, text, g.spans(tree, text), 0), nil
}

// withInjections overlays embedded languages declared by a grammar's
// injections.scm. It is intentionally depth-limited: HTML -> JavaScript/CSS
// is useful; recursive template languages are not worth the risk yet.
func (e *Engine) withInjections(g *grammar, tree *sitter.Tree, text []byte, base []Span, depth int) []Span {
	if g.injectionQuery == nil || depth >= 2 {
		return base
	}

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	matches := cursor.Matches(g.injectionQuery, tree.RootNode(), text)

	var extra []Span
	for match := matches.Next(); match != nil; match = matches.Next() {
		language := ""
		for _, property := range g.injectionQuery.PropertySettings(match.PatternIndex) {
			if property.Key == "injection.language" && property.Value != nil {
				language = *property.Value
			}
		}

		var content *sitter.Node
		for i := range match.Captures {
			capture := &match.Captures[i]
			name := ""
			if int(capture.Index) < len(g.injectionCaptures) {
				name = g.injectionCaptures[capture.Index]
			}
			switch name {
			case "injection.content":
				node := capture.Node
				content = &node
			case "injection.language":
				if language == "" {
					language = capture.Node.Utf8Text(text)
				}
			}
		}
		if content == nil || language == "" {
			continue
		}

		target, err := e.grammarFor(normalizeInjectionLanguage(language))
		if err != nil || target == g {
			continue
		}

		childText := []byte(content.Utf8Text(text))
		childTree := target.parser.Parse(childText, nil)
		if childTree == nil {
			continue
		}
		childSpans := target.spans(childTree, childText)
		childSpans = e.withInjections(target, childTree, childText, childSpans, depth+1)
		childTree.Close()

		start := content.StartPosition()
		for i := range childSpans {
			if childSpans[i].Line == 0 {
				childSpans[i].Start += int(start.Column)
				childSpans[i].End += int(start.Column)
			}
			childSpans[i].Line += int(start.Row)
		}
		extra = append(extra, childSpans...)
	}
	return mergeSpans(base, extra)
}

func normalizeInjectionLanguage(language string) string {
	switch language {
	case "js", "jsx":
		return "javascript"
	case "ts":
		return "typescript"
	case "sh":
		return "shell"
	case "cpp":
		return "c++"
	}
	return language
}

// mergeSpans overlays extra spans on top of base spans, line by line, so an
// embedded language can replace the host language's colouring.
func mergeSpans(base, extra []Span) []Span {
	if len(extra) == 0 {
		return base
	}
	baseByLine := make(map[int][]Span)
	extraByLine := make(map[int][]Span)
	for _, span := range base {
		baseByLine[span.Line] = append(baseByLine[span.Line], span)
	}
	for _, span := range extra {
		extraByLine[span.Line] = append(extraByLine[span.Line], span)
	}
	lineSet := make(map[int]bool, len(baseByLine)+len(extraByLine))
	for line := range baseByLine {
		lineSet[line] = true
	}
	for line := range extraByLine {
		lineSet[line] = true
	}
	lines := make([]int, 0, len(lineSet))
	for line := range lineSet {
		lines = append(lines, line)
	}
	sort.Ints(lines)

	var out []Span
	for _, line := range lines {
		painted := append([]Span(nil), baseByLine[line]...)
		sort.SliceStable(painted, func(i, j int) bool { return painted[i].Start < painted[j].Start })
		for _, span := range extraByLine[line] {
			painted = paint(painted, span)
		}
		out = append(out, painted...)
	}
	return out
}

func (e *Engine) grammarFor(filetype string) (*grammar, error) {
	if g, ok := e.grammars[filetype]; ok {
		return g, nil
	}
	if err, ok := e.failures[filetype]; ok {
		return nil, err
	}
	g, err := e.loadGrammar(filetype)
	if err != nil {
		e.failures[filetype] = err
		return nil, err
	}
	e.grammars[filetype] = g
	return g, nil
}

func (e *Engine) loadGrammar(filetype string) (*grammar, error) {
	definition, ok := definitionForFiletype(filetype)
	if !ok {
		return nil, fmt.Errorf("unsupported filetype %q: no linked tree-sitter grammar", filetype)
	}
	language := definition.Language()
	if language == nil {
		return nil, fmt.Errorf("grammar %q returned a nil language", definition.ID)
	}
	var sources []string
	for _, name := range definition.Queries {
		queryPath, err := e.findQuery(definition.ID, name)
		if err != nil {
			return nil, err
		}
		querySource, err := os.ReadFile(queryPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", queryPath, err)
		}
		sources = append(sources, string(querySource))
	}
	query, queryErr := sitter.NewQuery(language, strings.Join(sources, "\n"))
	if queryErr != nil {
		return nil, fmt.Errorf("grammar %q query: %s", definition.ID, queryErr.Message)
	}
	var injectionQuery *sitter.Query
	var injectionSources []string
	for _, name := range definition.Injections {
		queryPath, err := e.findQuery(definition.ID, name)
		if err != nil {
			query.Close()
			return nil, err
		}
		querySource, err := os.ReadFile(queryPath)
		if err != nil {
			query.Close()
			return nil, fmt.Errorf("read %s: %w", queryPath, err)
		}
		injectionSources = append(injectionSources, string(querySource))
	}
	if len(injectionSources) > 0 {
		injectionQuery, queryErr = sitter.NewQuery(language, strings.Join(injectionSources, "\n"))
		if queryErr != nil {
			query.Close()
			return nil, fmt.Errorf("grammar %q injection query: %s", definition.ID, queryErr.Message)
		}
	}
	parser := sitter.NewParser()
	if err := parser.SetLanguage(language); err != nil {
		if injectionQuery != nil {
			injectionQuery.Close()
		}
		query.Close()
		parser.Close()
		return nil, fmt.Errorf("set language %q: %w", definition.ID, err)
	}
	g := &grammar{
		id:       definition.ID,
		parser:   parser,
		language: language,
		query:    query,
		captures: query.CaptureNames(),
	}
	if injectionQuery != nil {
		g.injectionQuery = injectionQuery
		g.injectionCaptures = injectionQuery.CaptureNames()
	}
	return g, nil
}

func (e *Engine) findQuery(grammarID, name string) (string, error) {
	candidates := []string{filepath.Join(e.grammarDir, grammarID, name)}
	if name == "highlights.scm" {
		candidates = append(candidates,
			filepath.Join(e.grammarDir, grammarID+".scm"),
			filepath.Join(e.grammarDir, "highlights.scm"),
		)
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("no %s for grammar %q under %q", name, grammarID, e.grammarDir)
}

func (e *Engine) touchSession(id int64) *session {
	e.tick++
	s := e.sessions[id]
	if s == nil {
		s = &session{}
		e.sessions[id] = s
	}
	s.used = e.tick
	e.evictSessions()
	return s
}

func (e *Engine) evictSessions() {
	for len(e.sessions) > e.maxSessions {
		var oldestID int64
		oldestUse := ^uint64(0)
		for id, s := range e.sessions {
			if s.used < oldestUse {
				oldestID, oldestUse = id, s.used
			}
		}
		e.sessions[oldestID].closeTree()
		delete(e.sessions, oldestID)
	}
}

func (e *Engine) parse(g *grammar, s *session, filetype string, text []byte) (*sitter.Tree, error) {
	if s.tree != nil && s.filetype == filetype {
		oldTree := s.tree
		if tree := incrementalParse(g.parser, oldTree, s.text, text); tree != nil {
			oldTree.Close()
			s.tree = tree
			s.text = text
			s.filetype = filetype
			return tree, nil
		}
		oldTree.Close()
		s.tree = nil
	}
	s.closeTree()
	tree := g.parser.Parse(text, nil)
	if tree == nil {
		s.text = text
		s.filetype = filetype
		return nil, errors.New("tree-sitter parse failed")
	}
	s.tree = tree
	s.text = text
	s.filetype = filetype
	return tree, nil
}

// incrementalParse mutates oldTree with one InputEdit covering the changed
// region and reparses. The edit is always a single contiguous replacement in
// the old and new document, so it is valid even when several distant regions
// changed. Returns nil when tree-sitter declines to produce a tree.
func incrementalParse(parser *sitter.Parser, oldTree *sitter.Tree, oldText, newText []byte) *sitter.Tree {
	prefix, suffix := commonAffix(oldText, newText)
	if prefix == len(oldText) && prefix == len(newText) {
		// Unchanged document: reparse from the existing tree without an edit.
		return parser.Parse(newText, oldTree)
	}
	oldEnd := len(oldText) - suffix
	newEnd := len(newText) - suffix
	oldTree.Edit(&sitter.InputEdit{
		StartByte:      uint(prefix),
		OldEndByte:     uint(oldEnd),
		NewEndByte:     uint(newEnd),
		StartPosition:  pointAt(oldText, prefix),
		OldEndPosition: pointAt(oldText, oldEnd),
		NewEndPosition: pointAt(newText, newEnd),
	})
	return parser.Parse(newText, oldTree)
}

func commonAffix(a, b []byte) (prefix, suffix int) {
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	for prefix < limit && a[prefix] == b[prefix] {
		prefix++
	}
	for suffix < limit-prefix && a[len(a)-1-suffix] == b[len(b)-1-suffix] {
		suffix++
	}
	return prefix, suffix
}

func pointAt(text []byte, offset int) sitter.Point {
	if offset > len(text) {
		offset = len(text)
	}
	row, lineStart := 0, 0
	for i := 0; i < offset; i++ {
		if text[i] == '\n' {
			row++
			lineStart = i + 1
		}
	}
	return sitter.Point{Row: uint(row), Column: uint(offset - lineStart)}
}

type lineIndex struct {
	starts  []int
	lengths []int
}

func newLineIndex(text []byte) lineIndex {
	starts := []int{0}
	for i, b := range text {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	lengths := make([]int, len(starts))
	for i, start := range starts {
		end := len(text)
		if i+1 < len(starts) {
			end = starts[i+1] - 1
		}
		if end < start {
			end = start
		}
		lengths[i] = end - start
	}
	return lineIndex{starts: starts, lengths: lengths}
}

type rawSpan struct {
	Span
	seq  int
	spec int
}

func (g *grammar) spans(tree *sitter.Tree, text []byte) []Span {
	lines := newLineIndex(text)
	root := tree.RootNode()
	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	matches := cursor.Matches(g.query, root, text)
	var raw []rawSpan
	seq := 0
	for match := matches.Next(); match != nil; match = matches.Next() {
		for _, capture := range match.Captures {
			name := ""
			if int(capture.Index) < len(g.captures) {
				name = g.captures[capture.Index]
			}
			group := MapCapture(name)
			if group == "" {
				continue
			}
			node := capture.Node
			start, end := node.StartPosition(), node.EndPosition()
			spec := strings.Count(name, ".") + 1
			for _, span := range splitNode(lines, int(start.Row), int(start.Column), int(end.Row), int(end.Column)) {
				raw = append(raw, rawSpan{Span: Span{
					Line:  span.Line,
					Start: span.Start,
					End:   span.End,
					Group: group,
				}, seq: seq, spec: spec})
			}
			seq++
		}
	}
	return resolve(raw)
}

func splitNode(lines lineIndex, startRow, startColumn, endRow, endColumn int) []Span {
	if startRow < 0 {
		startRow = 0
	}
	if endRow < startRow {
		return nil
	}
	last := len(lines.starts) - 1
	if startRow > last {
		return nil
	}
	if endRow > last {
		endRow = last
		endColumn = lines.lengths[last]
	}
	clamp := func(row, column int) int {
		if column < 0 {
			return 0
		}
		if column > lines.lengths[row] {
			return lines.lengths[row]
		}
		return column
	}
	var out []Span
	if startRow == endRow {
		start, end := clamp(startRow, startColumn), clamp(endRow, endColumn)
		if end > start {
			out = append(out, Span{Line: startRow, Start: start, End: end})
		}
		return out
	}
	if start := clamp(startRow, startColumn); lines.lengths[startRow] > start {
		out = append(out, Span{Line: startRow, Start: start, End: lines.lengths[startRow]})
	}
	for row := startRow + 1; row < endRow; row++ {
		if lines.lengths[row] > 0 {
			out = append(out, Span{Line: row, Start: 0, End: lines.lengths[row]})
		}
	}
	if end := clamp(endRow, endColumn); end > 0 {
		out = append(out, Span{Line: endRow, Start: 0, End: end})
	}
	return out
}

func resolve(raw []rawSpan) []Span {
	if len(raw) == 0 {
		return nil
	}
	byLine := make(map[int][]rawSpan)
	for _, span := range raw {
		byLine[span.Line] = append(byLine[span.Line], span)
	}
	lineNumbers := make([]int, 0, len(byLine))
	for line := range byLine {
		lineNumbers = append(lineNumbers, line)
	}
	sort.Ints(lineNumbers)

	var out []Span
	for _, line := range lineNumbers {
		candidates := byLine[line]
		// Identical ranges: the most specific capture name wins; on a tie the
		// earlier pattern wins (e.g. @function over a later @variable).
		type spanKey struct{ start, end int }
		best := make(map[spanKey]rawSpan, len(candidates))
		for _, candidate := range candidates {
			key := spanKey{candidate.Start, candidate.End}
			current, seen := best[key]
			if !seen || candidate.spec > current.spec ||
				(candidate.spec == current.spec && candidate.seq < current.seq) {
				best[key] = candidate
			}
		}
		list := make([]rawSpan, 0, len(best))
		for _, candidate := range best {
			list = append(list, candidate)
		}
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].spec != list[j].spec {
				return list[i].spec < list[j].spec
			}
			return list[i].seq < list[j].seq
		})

		var painted []Span
		for _, candidate := range list {
			painted = paint(painted, Span{
				Line:  line,
				Start: candidate.Start,
				End:   candidate.End,
				Group: candidate.Group,
			})
		}
		for _, span := range painted {
			if n := len(out); n > 0 && out[n-1].Line == span.Line &&
				out[n-1].End == span.Start && out[n-1].Group == span.Group {
				out[n-1].End = span.End
				continue
			}
			out = append(out, span)
		}
	}
	return out
}

// paint overwrites [span.Start, span.End) in a sorted, non-overlapping list.
// Later spans win; the caller establishes precedence by paint order.
func paint(list []Span, span Span) []Span {
	out := list[:0]
	for _, existing := range list {
		if existing.End <= span.Start || existing.Start >= span.End {
			out = append(out, existing)
			continue
		}
		if existing.Start < span.Start {
			out = append(out, Span{Line: existing.Line, Start: existing.Start, End: span.Start, Group: existing.Group})
		}
		if existing.End > span.End {
			out = append(out, Span{Line: existing.Line, Start: span.End, End: existing.End, Group: existing.Group})
		}
	}
	out = append(out, span)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}
