package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/micro-editor/micro/v2/internal/buffer"
	"github.com/micro-editor/micro/v2/pkg/syntax"
)

func TestLuaSyntaxProviderEndToEnd(t *testing.T) {
	found := false
	for _, name := range syntax.ProviderNames() {
		if name == "gosyntax.go-hardcoded" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("demo provider not registered: %v", syntax.ProviderNames())
	}

	// The embedded tree-sitter plugin also claims Go. This test exercises the
	// Lua provider bridge, so suspend any other Go provider while it runs.
	var suspended []*syntax.Provider
	for _, p := range syntax.Providers() {
		if p.Name != "gosyntax.go-hardcoded" && p.Match.FileType == "go" {
			suspended = append(suspended, p)
			syntax.Unregister(p.Name)
		}
	}
	t.Cleanup(func() {
		for _, p := range suspended {
			syntax.Register(p)
		}
	})

	path := filepath.Join(t.TempDir(), "demo.go")
	content := "package main\n\nfunc main() {\n\tprintln(\"hi\", 42)\n}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	work := make(chan func(), 4)
	oldMain, oldDebounce := buffer.MainThread, buffer.HighlightDebounce
	buffer.MainThread = func(f func()) { work <- f }
	buffer.HighlightDebounce = time.Millisecond
	defer func() {
		buffer.MainThread = oldMain
		buffer.HighlightDebounce = oldDebounce
	}()

	b, err := buffer.NewBufferFromFile(path, buffer.BTDefault)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	b.InvalidateSyntax()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case f := <-work:
			f()
		default:
		}
		for _, g := range b.Match(3) {
			if g != syntax.NoGroup && g.String() == "constant.string" {
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("demo provider did not produce a Go string span")
}
