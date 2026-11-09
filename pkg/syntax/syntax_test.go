package syntax

import (
	"regexp"
	"testing"
)

func TestIntern(t *testing.T) {
	Clear()
	a := Intern("comment")
	b := Intern("comment")
	if a == NoGroup || a != b {
		t.Fatalf("expected stable non-zero group, got %v and %v", a, b)
	}
	if got := a.String(); got != "comment" {
		t.Fatalf("String() = %q, want comment", got)
	}
	if NoGroup.String() != "" {
		t.Fatalf("NoGroup.String() = %q", NoGroup.String())
	}
	if !Defined("comment") || Defined("missing") {
		t.Fatal("Defined mismatch")
	}
}

func TestBuildMatchCombiningMarks(t *testing.T) {
	line := []byte("ae\u0301b")
	g := Intern("constant.string")
	m := BuildMatch(line, []Span{{StartByte: 1, EndByte: 4, Group: g}})
	if len(m) != 3 {
		t.Fatalf("match = %v, want 3 boundaries", m)
	}
	if m[0] != NoGroup || m[1] != g || m[2] != NoGroup {
		t.Fatalf("match = %v, want 0:none 1:string 2:nothing", m)
	}
}

func TestBuildMatchEmojiAndOverlap(t *testing.T) {
	line := []byte("\U0001F4DAx")
	a := Intern("a")
	b := Intern("b")
	m := BuildMatch(line, []Span{
		{StartByte: 0, EndByte: 4, Group: a},
		{StartByte: 0, EndByte: 5, Group: b},
	})
	if m[0] != b || len(m) != 1 {
		t.Fatalf("later span should win over the whole line: %v", m)
	}

	m = BuildMatch(line, []Span{
		{StartByte: 0, EndByte: 5, Group: a},
		{StartByte: 4, EndByte: 5, Group: b},
	})
	if m[0] != a || m[1] != b {
		t.Fatalf("match = %v, want 0:a 1:b", m)
	}
}

func TestBuildMatchEmptyGroupClears(t *testing.T) {
	line := []byte("abc")
	g := Intern("comment")
	m := BuildMatch(line, []Span{
		{StartByte: 0, EndByte: 3, Group: g},
		{StartByte: 1, EndByte: 2, Group: NoGroup},
	})
	if m[0] != g || m[1] != NoGroup || m[2] != g {
		t.Fatalf("match = %v", m)
	}
}

type testSource struct {
	lines [][]byte
}

func (s testSource) LinesNum() int { return len(s.lines) }
func (s testSource) LineBytes(n int) []byte {
	if n < 0 || n >= len(s.lines) {
		return nil
	}
	return s.lines[n]
}

func TestLookupPriorityAndSignature(t *testing.T) {
	defer Clear()
	low := &Provider{Name: "low", Match: Matcher{FileType: "foo"}, Priority: 1}
	high := &Provider{Name: "high", Match: Matcher{FileType: "foo"}, Priority: 10}
	Register(low)
	Register(high)

	src := testSource{lines: [][]byte{[]byte("x")}}
	if got := Lookup("f", "foo", src, 100); got == nil || got.Name != "high" {
		t.Fatalf("Lookup = %v, want high", got)
	}

	Clear()
	fn := &Provider{Name: "fn", Match: Matcher{
		FileName:  regexp.MustCompile(`\.h$`),
		Signature: regexp.MustCompile(`objc`),
	}}
	hdr := &Provider{Name: "hdr", Match: Matcher{Header: regexp.MustCompile(`^package`)}}
	Register(fn)
	Register(hdr)
	src = testSource{lines: [][]byte{[]byte("objc stuff")}}
	if got := Lookup("x.h", "unknown", src, 100); got == nil || got.Name != "fn" {
		t.Fatalf("filename matcher = %v, want fn", got)
	}
	src = testSource{lines: [][]byte{[]byte("package main")}}
	if got := Lookup("noext", "unknown", src, 100); got == nil || got.Name != "hdr" {
		t.Fatalf("header matcher = %v, want hdr", got)
	}
}

func TestReportErrorDisablesProvider(t *testing.T) {
	defer Clear()
	p := &Provider{Name: "bad", Match: Matcher{FileType: "bad"}, Request: func(Request) {}}
	Register(p)
	if ReportError("bad") || ReportError("bad") {
		t.Fatal("provider disabled too early")
	}
	if !ReportError("bad") {
		t.Fatal("provider should be disabled on third failure")
	}
	if len(Providers()) != 0 {
		t.Fatal("provider still registered")
	}
}
