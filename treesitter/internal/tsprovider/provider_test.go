package tsprovider

import (
	"bytes"
	"math/rand"
	"os"
	"testing"
)

func loadSample(t *testing.T) []byte {
	t.Helper()
	text, err := os.ReadFile("../../testdata/sample.go")
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func splitLines(text []byte) [][]byte {
	return bytes.Split(text, []byte("\n"))
}

func groupAt(spans []Span, line, column int) string {
	for _, span := range spans {
		if span.Line == line && column >= span.Start && column < span.End {
			return span.Group
		}
	}
	return ""
}

func haveSpan(spans []Span, line, start, end int, group string) bool {
	for _, span := range spans {
		if span.Line == line && span.Start == start && span.End == end && span.Group == group {
			return true
		}
	}
	return false
}

func TestGoSampleSpans(t *testing.T) {
	engine := New("../../queries")
	defer engine.Close()
	text := loadSample(t)
	spans, err := engine.Spans(1, "go", text)
	if err != nil {
		t.Fatal(err)
	}
	lines := splitLines(text)

	// Go keyword refinement: declarations are preproc, control flow is
	// special, and conditional keywords stay statement.
	if got := groupAt(spans, 0, 0); got != "preproc" {
		t.Errorf("package keyword: got %q", got)
	}
	if got := groupAt(spans, 11, 0); got != "preproc" {
		t.Errorf("func keyword: got %q", got)
	}
	if got := groupAt(spans, 16, 1); got != "special" {
		t.Errorf("return keyword: got %q", got)
	}

	// Multi-line block comment: first line from the opener, middle line fully
	// covered, last line through the closer.
	if got := groupAt(spans, 3, 0); got != "comment" {
		t.Errorf("block comment opener: got %q", got)
	}
	if got := groupAt(spans, 4, 0); got != "comment" {
		t.Errorf("block comment middle line column 0: got %q", got)
	}
	if got := groupAt(spans, 4, len(lines[4])-1); got != "comment" {
		t.Errorf("block comment middle line last column: got %q", got)
	}
	if got := groupAt(spans, 5, len(lines[5])-1); got != "comment" {
		t.Errorf("block comment closer: got %q", got)
	}

	// Raw string with backticks spanning three lines.
	rawStart := bytes.IndexByte(lines[12], '`')
	if !haveSpan(spans, 12, rawStart, len(lines[12]), "constant.string") {
		t.Errorf("raw string first line: want constant.string [%d,%d), spans %v", rawStart, len(lines[12]), lineSpans(spans, 12))
	}
	if !haveSpan(spans, 13, 0, len(lines[13]), "constant.string") {
		t.Errorf("raw string middle line: spans %v", lineSpans(spans, 13))
	}
	if !haveSpan(spans, 14, 0, len(lines[14]), "constant.string") {
		t.Errorf("raw string last line: spans %v", lineSpans(spans, 14))
	}

	// Struct tag: the raw string literal is a single span including both
	// backticks.
	tagStart := bytes.IndexByte(lines[7], '`')
	tagEnd := bytes.LastIndexByte(lines[7], '`') + 1
	if !haveSpan(spans, 7, tagStart, tagEnd, "constant.string") {
		t.Errorf("struct tag: want constant.string [%d,%d), spans %v", tagStart, tagEnd, lineSpans(spans, 7))
	}

	// UTF-8 identifier: byte columns, not rune columns.
	name := []byte("Héllo")
	for _, line := range []int{6, 19} {
		start := bytes.Index(lines[line], name)
		if start < 0 {
			t.Fatalf("Héllo missing on line %d", line)
		}
		if !haveSpan(spans, line, start, start+len(name), "type") {
			t.Errorf("UTF-8 type identifier on line %d: want type [%d,%d), spans %v", line, start, start+len(name), lineSpans(spans, line))
		}
	}
	greet := []byte("Gréet")
	start := bytes.Index(lines[11], greet)
	if !haveSpan(spans, 11, start, start+len(greet), "identifier") {
		t.Errorf("UTF-8 function name: want identifier [%d,%d), spans %v", start, start+len(greet), lineSpans(spans, 11))
	}

	// Every span must be a non-empty, in-bounds byte range on its line.
	for _, span := range spans {
		if span.Line < 0 || span.Line >= len(lines) {
			t.Fatalf("span line out of range: %+v", span)
		}
		if span.Start < 0 || span.End > len(lines[span.Line]) || span.End <= span.Start {
			t.Fatalf("span out of bounds: %+v (line length %d)", span, len(lines[span.Line]))
		}
	}
}

func lineSpans(spans []Span, line int) []Span {
	var out []Span
	for _, span := range spans {
		if span.Line == line {
			out = append(out, span)
		}
	}
	return out
}

func TestIncrementalMatchesFullParse(t *testing.T) {
	engine := New("../../queries")
	defer engine.Close()
	text := append([]byte(nil), loadSample(t)...)

	insertions := []string{
		"x", "\n", "// note\n", "func f() {}\n", "é", "`", "/*", "\tvar _ = 1\n",
		"日本語", "\r\n", "string", "}", "package p\n",
	}
	random := rand.New(rand.NewSource(7))
	for step := 0; step < 40; step++ {
		switch random.Intn(3) {
		case 0:
			at := random.Intn(len(text) + 1)
			insert := insertions[random.Intn(len(insertions))]
			text = append(text[:at:at], append([]byte(insert), text[at:]...)...)
		case 1:
			if len(text) > 1 {
				at := random.Intn(len(text))
				n := 1 + random.Intn(min(5, len(text)-at))
				text = append(text[:at:at], text[at+n:]...)
			}
		default:
			if len(text) > 1 {
				at := random.Intn(len(text))
				n := 1 + random.Intn(min(5, len(text)-at))
				text = append(text[:at:at], append([]byte(insertions[random.Intn(len(insertions))]), text[at+n:]...)...)
			}
		}

		got, err := engine.Spans(1, "go", text)
		if err != nil {
			t.Fatalf("step %d: incremental parse: %v", step, err)
		}
		fresh := New("../../queries")
		want, err := fresh.Spans(1, "go", text)
		fresh.Close()
		if err != nil {
			t.Fatalf("step %d: full parse: %v", step, err)
		}
		if !equalSpans(got, want) {
			t.Fatalf("step %d: incremental spans differ from full parse\ngot  %v\nwant %v\ntext:\n%s", step, got, want, text)
		}
	}
}

func equalSpans(a, b []Span) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSplitNode(t *testing.T) {
	lines := newLineIndex([]byte("aa\n\nbbbb\ncc"))
	got := splitNode(lines, 0, 1, 2, 1)
	want := []Span{
		{Line: 0, Start: 1, End: 2},
		{Line: 2, Start: 0, End: 1},
	}
	if !equalSpans(got, want) {
		t.Fatalf("split: got %v want %v", got, want)
	}
	// A node ending at column 0 of the next line must not emit a zero-width
	// span on that line.
	got = splitNode(lines, 0, 0, 1, 0)
	if len(got) != 1 || got[0] != (Span{Line: 0, Start: 0, End: 2}) {
		t.Fatalf("end at line start: got %v", got)
	}
	// A node covering an empty line emits nothing for it.
	got = splitNode(lines, 1, 0, 2, 0)
	if len(got) != 0 {
		t.Fatalf("empty middle/end line: got %v", got)
	}
}

func TestCapturePrecedence(t *testing.T) {
	engine := New("../../queries")
	defer engine.Close()
	text := []byte("package p\n\nvar s = \"a\\tb\"\n\nfunc f() { foo(len(s)) }\n")
	spans, err := engine.Spans(1, "go", text)
	if err != nil {
		t.Fatal(err)
	}
	lines := splitLines(text)

	// An escape capture is narrower and later than its enclosing string, so
	// it must override the string group inside the quoted literal.
	escape := bytes.Index(lines[2], []byte("\\t"))
	if escape < 0 {
		t.Fatalf("escape sequence missing from %q", lines[2])
	}
	if got := groupAt(spans, 2, escape); got != "constant.specialChar" {
		t.Errorf("escape group: got %q, spans %v", got, lineSpans(spans, 2))
	}
	quote := bytes.IndexByte(lines[2], '"')
	if got := groupAt(spans, 2, quote); got != "constant.string" {
		t.Errorf("string group: got %q", got)
	}

	// @function and @variable cover the same node for a call name. The
	// earlier, more specific pattern must win; @function.builtin wins over
	// @function by capture-name specificity.
	foo := bytes.Index(lines[4], []byte("foo"))
	if got := groupAt(spans, 4, foo); got != "identifier" {
		t.Errorf("call name group: got %q, spans %v", got, lineSpans(spans, 4))
	}
	builtin := bytes.Index(lines[4], []byte("len"))
	if got := groupAt(spans, 4, builtin); got != "preproc" {
		t.Errorf("builtin call group: got %q, spans %v", got, lineSpans(spans, 4))
	}
}

func TestSessionsAreBounded(t *testing.T) {
	engine := New("../../queries")
	defer engine.Close()
	text := []byte("package p\n")
	for id := int64(1); id <= 20; id++ {
		if _, err := engine.Spans(id, "go", text); err != nil {
			t.Fatal(err)
		}
	}
	if len(engine.sessions) != engine.maxSessions {
		t.Fatalf("session count: got %d want %d", len(engine.sessions), engine.maxSessions)
	}
	if engine.sessions[20] == nil {
		t.Fatal("most recently used session was evicted")
	}
}

func TestUnsupportedFiletype(t *testing.T) {
	engine := New("../../queries")
	defer engine.Close()
	if _, err := engine.Spans(1, "python", []byte("print(1)\n")); err == nil {
		t.Fatal("expected an error for an unlinked filetype")
	}
}

func TestMapCapture(t *testing.T) {
	cases := map[string]string{
		"comment":               "comment",
		"comment.documentation": "comment",
		"string":                "constant.string",
		"escape":                "constant.specialChar",
		"number":                "constant.number",
		"keyword":               "statement",
		"keyword.function":      "preproc",
		"function":              "identifier",
		"function.builtin":      "preproc",
		"type":                  "type",
		"constant.builtin":      "constant.bool",
		"operator":              "symbol.operator",
		"punctuation.bracket":   "symbol.brackets",
		"preproc":               "preproc",
		"unknown.capture":       "",
	}
	for capture, want := range cases {
		if got := MapCapture(capture); got != want {
			t.Errorf("MapCapture(%q) = %q, want %q", capture, got, want)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
