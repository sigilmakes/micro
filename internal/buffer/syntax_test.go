package buffer

import (
	"testing"
	"time"

	"github.com/micro-editor/micro/v2/pkg/syntax"
)

func TestNoProviderKeepsWorking(t *testing.T) {
	syntax.Clear()
	b := NewBufferFromString("hello\nworld", "file.definitely-not-a-known-ext", BTDefault)
	defer b.Close()

	if b.hl != nil {
		t.Fatal("unexpected provider session")
	}
	b.UpdateProvider()
	if b.Match(0) != nil || b.Match(1) != nil {
		t.Fatal("unexpected highlight match without a provider")
	}
}

func TestApplySpansDropsStaleGeneration(t *testing.T) {
	syntax.Clear()
	defer syntax.Clear()
	syntax.Register(&syntax.Provider{
		Name:    "test-provider",
		Match:   syntax.Matcher{FileType: "go"},
		Request: func(syntax.Request) {},
	})

	old := HighlightDebounce
	HighlightDebounce = time.Hour
	defer func() { HighlightDebounce = old }()

	b := NewBufferFromString("hello", "x.go", BTDefault)
	defer b.Close()
	if b.hl == nil {
		t.Fatal("provider did not attach")
	}
	if b.hl.timer != nil {
		b.hl.timer.Stop()
		b.hl.timer = nil
	}

	gen := b.hlGen
	g := syntax.Intern("comment")
	spans := []syntax.Span{{Line: 0, StartByte: 0, EndByte: 5, Group: g}}

	if b.ApplySpans(gen-1, spans) {
		t.Fatal("stale generation was applied")
	}
	if b.Match(0) != nil {
		t.Fatal("stale generation changed matches")
	}
	if !b.ApplySpans(gen, spans) {
		t.Fatal("current generation was rejected")
	}
	if got := b.Match(0)[0]; got != g {
		t.Fatalf("match[0] = %v, want %v", got, g)
	}

	b.MarkModified(0, 0)
	if b.ApplySpans(gen, spans) {
		t.Fatal("reply from before an edit was applied")
	}
	if b.hl.timer != nil {
		b.hl.timer.Stop()
		b.hl.timer = nil
	}
}

func TestApplySpansClearsUnmentionedLines(t *testing.T) {
	syntax.Clear()
	defer syntax.Clear()
	syntax.Register(&syntax.Provider{
		Name:    "test-provider",
		Match:   syntax.Matcher{FileType: "go"},
		Request: func(syntax.Request) {},
	})

	old := HighlightDebounce
	HighlightDebounce = time.Hour
	defer func() { HighlightDebounce = old }()

	b := NewBufferFromString("one\ntwo", "x.go", BTDefault)
	defer b.Close()
	if b.hl.timer != nil {
		b.hl.timer.Stop()
		b.hl.timer = nil
	}

	g := syntax.Intern("comment")
	if !b.ApplySpans(b.hlGen, []syntax.Span{{Line: 0, StartByte: 0, EndByte: 3, Group: g}}) {
		t.Fatal("spans rejected")
	}
	if b.Match(0)[0] != g {
		t.Fatal("line 0 not highlighted")
	}
	if b.Match(1) != nil {
		t.Fatal("unmentioned line was not cleared")
	}
}

func TestSharedViewsShareProviderID(t *testing.T) {
	syntax.Clear()
	first := NewBufferFromString("x", "same.go", BTDefault)
	defer first.Close()
	second := NewBufferFromString("x", "same.go", BTDefault)
	defer second.Close()

	if first.hlID == 0 || first.hlID != second.hlID {
		t.Fatalf("ids = %d and %d", first.hlID, second.hlID)
	}
}

func TestSyntaxOffStopsProvider(t *testing.T) {
	syntax.Clear()
	defer syntax.Clear()
	syntax.Register(&syntax.Provider{
		Name:    "test-provider",
		Match:   syntax.Matcher{FileType: "go"},
		Request: func(syntax.Request) {},
	})

	old := HighlightDebounce
	HighlightDebounce = time.Hour
	defer func() { HighlightDebounce = old }()

	b := NewBufferFromString("package main", "x.go", BTDefault)
	defer b.Close()
	if b.hl == nil {
		t.Fatal("provider did not attach")
	}
	if b.hl.timer != nil {
		b.hl.timer.Stop()
		b.hl.timer = nil
	}

	b.Settings["syntax"] = false
	b.UpdateProvider()
	if b.hl != nil {
		t.Fatal("provider was not detached")
	}
	if b.FileType() != "go" {
		t.Fatalf("filetype = %q, want go", b.FileType())
	}

	b.Settings["syntax"] = true
	b.UpdateProvider()
	if b.hl == nil {
		t.Fatal("provider was not attached again")
	}
	if b.hl.timer != nil {
		b.hl.timer.Stop()
		b.hl.timer = nil
	}
}
