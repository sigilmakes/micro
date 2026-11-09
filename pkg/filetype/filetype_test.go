package filetype

import "testing"

type source struct {
	lines []string
}

func (s source) LinesNum() int { return len(s.lines) }
func (s source) LineBytes(n int) []byte {
	if n < 0 || n >= len(s.lines) {
		return nil
	}
	return []byte(s.lines[n])
}

func TestMatchFilename(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"main.go", "go"},
		{"Makefile", "makefile"},
		{"script.sh", "shell"},
		{"x.yaml", "yaml"},
		{"x.unknown-ext", ""},
	}
	for _, c := range cases {
		if got := Match(c.path, source{[]string{""}}, 100); got != c.want {
			t.Errorf("Match(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestMatchHeaderAndSignature(t *testing.T) {
	if got := Match("noext", source{[]string{"#!/usr/bin/env python3"}}, 100); got != "python" {
		t.Errorf("python header detection = %q", got)
	}
	if got := Match("x.h", source{[]string{"class Foo {};"}}, 100); got != "c++" {
		t.Errorf("c++ signature detection = %q", got)
	}
	if got := Match("x.h", source{[]string{"#import <Foundation/Foundation.h>"}}, 100); got != "objective-c" {
		t.Errorf("objc signature detection = %q", got)
	}
}

func TestFiletypes(t *testing.T) {
	fts := Filetypes()
	if len(fts) < 100 {
		t.Fatalf("only %d filetypes", len(fts))
	}
	seen := make(map[string]bool)
	for _, ft := range fts {
		if ft == "" || ft == "unknown" || ft == "off" {
			t.Fatalf("unexpected filetype %q", ft)
		}
		if seen[ft] {
			t.Fatalf("duplicate filetype %q", ft)
		}
		seen[ft] = true
	}
	if !seen["go"] || !seen["c++"] {
		t.Fatalf("missing common filetypes: go=%v c++=%v", seen["go"], seen["c++"])
	}
}
