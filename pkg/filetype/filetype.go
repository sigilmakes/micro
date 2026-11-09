// Package filetype assigns a deterministic filetype identity to a path.
//
// The built-in table is generated from the old YAML syntax metadata. It uses
// filename, first-line, and signature regular expressions, so detection is
// independent of whether any highlighting provider is registered.
package filetype

import (
	"log"
	"regexp"
	"sort"
)

// Source is the read-only line view used for header and signature matching.
type Source interface {
	LinesNum() int
	LineBytes(int) []byte
}

// Spec is one generated detection entry.
type Spec struct {
	Filetype  string
	Filename  string
	Header    string
	Signature string
}

// Detector is a compiled detection entry.
type Detector struct {
	Filetype  string
	Filename  *regexp.Regexp
	Header    *regexp.Regexp
	Signature *regexp.Regexp
}

var detectors []Detector

func init() {
	detectors = make([]Detector, 0, len(table))
	for _, spec := range table {
		d := Detector{Filetype: spec.Filetype}
		var err error
		if spec.Filename != "" {
			d.Filename, err = regexp.Compile(spec.Filename)
		}
		if err == nil && spec.Header != "" {
			d.Header, err = regexp.Compile(spec.Header)
		}
		if err == nil && spec.Signature != "" {
			d.Signature, err = regexp.Compile(spec.Signature)
		}
		if err != nil {
			log.Printf("filetype: skipping invalid detector for %q: %v", spec.Filetype, err)
			continue
		}
		detectors = append(detectors, d)
	}
}

// Detectors returns a copy of the compiled table.
func Detectors() []Detector {
	out := make([]Detector, len(detectors))
	copy(out, detectors)
	return out
}

// Match returns the detected filetype, or "" when nothing matches.
//
// This mirrors the old syntax-file resolution: filename matches take
// precedence over first-line matches, and when several candidates match a
// signature regex scanned over the first limit lines breaks the tie. Then the
// first registration (alphabetical YAML name) wins.
func Match(path string, src Source, limit int) string {
	var firstLine []byte
	lines := 0
	if src != nil {
		lines = src.LinesNum()
		if lines > 0 {
			firstLine = src.LineBytes(0)
		}
	}

	var filenameMatches []*Detector
	var headerMatches []*Detector
	for i := range detectors {
		d := &detectors[i]
		if d.Filename != nil && d.Filename.MatchString(path) {
			filenameMatches = append(filenameMatches, d)
		}
		if len(filenameMatches) == 0 && d.Header != nil && d.Header.Match(firstLine) {
			headerMatches = append(headerMatches, d)
		}
	}

	matches := filenameMatches
	if len(matches) == 0 {
		matches = headerMatches
	}
	if len(matches) == 0 {
		return ""
	}

	if len(matches) > 1 && src != nil {
		n := limit
		if n <= 0 || n > lines {
			n = lines
		}
		for _, d := range matches {
			if d.Signature == nil {
				continue
			}
			for i := 0; i < n; i++ {
				if d.Signature.Match(src.LineBytes(i)) {
					return d.Filetype
				}
			}
		}
	}

	return matches[0].Filetype
}

// Filetypes returns the sorted filetypes in the generated table.
func Filetypes() []string {
	seen := make(map[string]bool, len(detectors))
	for _, d := range detectors {
		seen[d.Filetype] = true
	}
	out := make([]string, 0, len(seen))
	for ft := range seen {
		out = append(out, ft)
	}
	sort.Strings(out)
	return out
}
