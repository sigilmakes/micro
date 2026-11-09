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

	sitter "github.com/tree-sitter/go-tree-sitter"
	tsgo "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// Span is one half-open, single-line byte range. Line is 0-based; Start and
// End are byte offsets within that line, End exclusive.
type Span struct {
	Line  int
	Start int
	End   int
	Group string
}

// filetypeGrammar names the tree-sitter grammar used for a micro filetype.
var filetypeGrammar = map[string]string{
	"go": "go",
}

// grammarLanguages links a grammar id to its compiled language. Adding a
// grammar means adding a module to go.mod and one entry here.
var grammarLanguages = map[string]func() *sitter.Language{
	"go": func() *sitter.Language {
		return sitter.NewLanguage(tsgo.Language())
	},
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

// goKeywordGroups refines the grammar's single @keyword capture into the
// vocabulary the old go.yaml used: declarations are preproc, control-flow
// keywords are special, and the rest stay statement.
var goKeywordGroups = map[string]string{
	"package": "preproc", "import": "preproc", "const": "preproc", "var": "preproc",
	"type": "preproc", "struct": "preproc", "func": "preproc", "interface": "preproc",
	"map": "preproc", "chan": "preproc", "defer": "preproc",
	"break": "special", "case": "special", "continue": "special", "default": "special",
	"go": "special", "goto": "special", "range": "special", "return": "special",
	"fallthrough": "special",
	"else":        "statement", "for": "statement", "if": "statement",
	"switch": "statement", "select": "statement",
}

type grammar struct {
	id       string
	parser   *sitter.Parser
	language *sitter.Language
	query    *sitter.Query
	captures []string
}

func (g *grammar) close() {
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

// New returns an engine that looks for <grammarDir>/<grammar>/highlights.scm.
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
	return g.spans(tree, text), nil
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
	grammarID, ok := filetypeGrammar[filetype]
	if !ok {
		return nil, fmt.Errorf("unsupported filetype %q: no linked tree-sitter grammar", filetype)
	}
	newLanguage, ok := grammarLanguages[grammarID]
	if !ok {
		return nil, fmt.Errorf("filetype %q maps to unknown grammar %q", filetype, grammarID)
	}
	language := newLanguage()
	if language == nil {
		return nil, fmt.Errorf("grammar %q returned a nil language", grammarID)
	}
	queryPath, err := e.findQuery(grammarID)
	if err != nil {
		return nil, err
	}
	querySource, err := os.ReadFile(queryPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", queryPath, err)
	}
	query, queryErr := sitter.NewQuery(language, string(querySource))
	if queryErr != nil {
		return nil, fmt.Errorf("%s: %s", queryPath, queryErr.Message)
	}
	parser := sitter.NewParser()
	if err := parser.SetLanguage(language); err != nil {
		query.Close()
		parser.Close()
		return nil, fmt.Errorf("set language %q: %w", grammarID, err)
	}
	return &grammar{
		id:       grammarID,
		parser:   parser,
		language: language,
		query:    query,
		captures: query.CaptureNames(),
	}, nil
}

func (e *Engine) findQuery(grammarID string) (string, error) {
	candidates := []string{
		filepath.Join(e.grammarDir, grammarID, "highlights.scm"),
		filepath.Join(e.grammarDir, grammarID+".scm"),
		filepath.Join(e.grammarDir, "highlights.scm"),
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("no highlights.scm for grammar %q under %q", grammarID, e.grammarDir)
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
			if g.id == "go" && name == "keyword" {
				if refined, ok := goKeywordGroups[nodeText(text, &node)]; ok {
					group = refined
				}
			}
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

func nodeText(text []byte, node *sitter.Node) string {
	start, end := node.StartByte(), node.EndByte()
	if start > end || end > uint(len(text)) {
		return ""
	}
	return string(text[start:end])
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
