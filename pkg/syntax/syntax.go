// Package syntax defines the provider-facing highlighting contract.
//
// It contains no highlighting engine: providers produce spans, the buffer
// stores them, and the display resolves group names through the colorscheme.
// The package imports only the Go standard library.
package syntax

import (
	"regexp"
	"sort"
	"sync"
)

// Group is an interned highlight-group name. Group 0 means "no group".
type Group uint16

// NoGroup is the zero group and clears highlighting for its range.
const NoGroup Group = 0

var (
	groupsMu   sync.RWMutex
	groupNames []string
	groupIDs   = map[string]Group{}
)

// Intern returns the stable id for name, assigning one on first use.
func Intern(name string) Group {
	if name == "" {
		return NoGroup
	}

	groupsMu.RLock()
	id, ok := groupIDs[name]
	groupsMu.RUnlock()
	if ok {
		return id
	}

	groupsMu.Lock()
	defer groupsMu.Unlock()
	if id, ok := groupIDs[name]; ok {
		return id
	}
	if len(groupNames) == 0 {
		groupNames = append(groupNames, "")
	}
	if len(groupNames) > 65535 {
		return NoGroup
	}
	id = Group(len(groupNames))
	groupNames = append(groupNames, name)
	groupIDs[name] = id
	return id
}

// String returns the interned name, or "" for NoGroup.
func (g Group) String() string {
	groupsMu.RLock()
	defer groupsMu.RUnlock()
	if int(g) >= len(groupNames) {
		return ""
	}
	return groupNames[g]
}

// Defined reports whether name has already been interned.
func Defined(name string) bool {
	groupsMu.RLock()
	defer groupsMu.RUnlock()
	_, ok := groupIDs[name]
	return ok
}

// Edit is one buffer mutation. Start and OldEnd are pre-edit coordinates;
// NewEnd is post-edit. Columns are character indices (rune plus combining
// marks), the same unit micro uses for cursors.
type Edit struct {
	StartLine  int
	StartCol   int
	OldEndLine int
	OldEndCol  int
	NewEndLine int
	NewEndCol  int
}

// Span is a half-open byte range on one line.
type Span struct {
	Line      int
	StartByte int
	EndByte   int
	Group     Group
}

// LineMatch maps a character (rune plus combining marks) index to a group.
// A key is present at every style change point, including index 0.
type LineMatch map[int]Group

// Matcher selects a provider by filetype or by path/content.
type Matcher struct {
	FileType  string
	FileName  *regexp.Regexp
	Header    *regexp.Regexp
	Signature *regexp.Regexp
}

// Matches reports whether the matcher claims a buffer. An explicit filetype
// wins; otherwise filename, first line, and signature are considered.
func (m *Matcher) Matches(path, filetype string, firstLine []byte) bool {
	if filetype != "" && filetype != "unknown" {
		return m.FileType == filetype
	}
	if m.FileName != nil && m.FileName.MatchString(path) {
		return true
	}
	return m.Header != nil && m.Header.Match(firstLine)
}

// Cause describes why a provider was asked for spans.
type Cause uint8

const (
	CauseOpen Cause = iota
	CauseEdit
	CauseManual
	CauseReload
)

// Request is one provider invocation. Reply may be called after Request
// returns, but always on the editor's main goroutine. A reply replaces every
// span in the buffer; lines it does not mention are cleared.
type Request struct {
	Buf        any
	BufID      uint64
	Path       string
	FileType   string
	Text       string
	First      int
	Last       int
	Generation uint64
	Cause      Cause
	Edits      []Edit
	Reply      func([]Span)
	// Fail lets a provider report that it could not produce spans for this
	// request. The buffer clears its in-flight state; the provider registry
	// handles repeated failures separately.
	Fail func(error)
}

// Provider is a registered highlighting backend.
type Provider struct {
	Name     string
	Owner    string
	Match    Matcher
	Priority int
	Request  func(Request)

	failures int
}

// Source is the read-only line view used for content-based matching.
type Source interface {
	LinesNum() int
	LineBytes(int) []byte
}

var (
	providersMu sync.RWMutex
	providers   []*Provider
)

// Register adds p, replacing an existing provider with the same name.
func Register(p *Provider) (unregister func()) {
	providersMu.Lock()
	for i, old := range providers {
		if old.Name == p.Name {
			providers = append(providers[:i], providers[i+1:]...)
			break
		}
	}
	providers = append(providers, p)
	providersMu.Unlock()
	return func() { Unregister(p.Name) }
}

// Unregister removes the named provider.
func Unregister(name string) {
	providersMu.Lock()
	defer providersMu.Unlock()
	for i, p := range providers {
		if p.Name == name {
			providers = append(providers[:i], providers[i+1:]...)
			return
		}
	}
}

// Clear removes every provider. Interned groups stay allocated because
// buffers may still hold their ids.
func Clear() {
	providersMu.Lock()
	providers = nil
	providersMu.Unlock()
}

// Providers returns a snapshot of the registry.
func Providers() []*Provider {
	providersMu.RLock()
	defer providersMu.RUnlock()
	out := make([]*Provider, len(providers))
	copy(out, providers)
	return out
}

// ProviderNames returns the registered provider names in registration order.
func ProviderNames() []string {
	providersMu.RLock()
	defer providersMu.RUnlock()
	out := make([]string, len(providers))
	for i, p := range providers {
		out[i] = p.Name
	}
	return out
}

// ReportSuccess resets a provider's consecutive-failure count.
func ReportSuccess(name string) {
	providersMu.Lock()
	defer providersMu.Unlock()
	for _, p := range providers {
		if p.Name == name {
			p.failures = 0
			return
		}
	}
}

// ReportError records one provider failure and returns true when the
// provider has failed three consecutive times and was removed.
func ReportError(name string) bool {
	providersMu.Lock()
	defer providersMu.Unlock()
	for i, p := range providers {
		if p.Name != name {
			continue
		}
		p.failures++
		if p.failures < 3 {
			return false
		}
		providers = append(providers[:i], providers[i+1:]...)
		return true
	}
	return false
}

// Lookup selects the provider for a buffer. When filetype is known, only
// providers for that filetype are considered. Otherwise filename matches
// take precedence over first-line matches, signatures break ties between
// several candidates, then priority and registration order decide.
func Lookup(path, filetype string, src Source, detectLimit int) *Provider {
	providersMu.RLock()
	candidates := make([]*Provider, len(providers))
	copy(candidates, providers)
	providersMu.RUnlock()

	if len(candidates) == 0 {
		return nil
	}

	if filetype != "" && filetype != "unknown" {
		var best *Provider
		for _, p := range candidates {
			if p.Match.FileType != filetype {
				continue
			}
			if best == nil || p.Priority > best.Priority {
				best = p
			}
		}
		if best != nil {
			return best
		}
		// A provider without a filetype can still claim a known path.
		return matchByPath(candidates, path, src, detectLimit, true)
	}

	return matchByPath(candidates, path, src, detectLimit, false)
}

func matchByPath(candidates []*Provider, path string, src Source, detectLimit int, skipTyped bool) *Provider {
	var firstLine []byte
	lines := 0
	if src != nil {
		lines = src.LinesNum()
		if lines > 0 {
			firstLine = src.LineBytes(0)
		}
	}

	var filenameMatches []*Provider
	var headerMatches []*Provider
	for _, p := range candidates {
		if skipTyped && p.Match.FileType != "" && p.Match.FileType != "unknown" {
			continue
		}
		if p.Match.FileName != nil && p.Match.FileName.MatchString(path) {
			filenameMatches = append(filenameMatches, p)
		}
		if len(filenameMatches) == 0 && p.Match.Header != nil && p.Match.Header.Match(firstLine) {
			headerMatches = append(headerMatches, p)
		}
	}

	matches := filenameMatches
	if len(matches) == 0 {
		matches = headerMatches
	}
	if len(matches) == 0 {
		return nil
	}

	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].Priority > matches[j].Priority
	})

	if len(matches) > 1 {
		limit := detectLimit
		if limit <= 0 || limit > lines {
			limit = lines
		}
		for _, p := range matches {
			if p.Match.Signature == nil {
				continue
			}
			for i := 0; i < limit; i++ {
				if p.Match.Signature.Match(src.LineBytes(i)) {
					return p
				}
			}
		}
	}

	return matches[0]
}

// FileTypes returns the sorted filetypes claimed by registered providers.
func FileTypes() []string {
	providersMu.RLock()
	seen := make(map[string]bool, len(providers))
	for _, p := range providers {
		if p.Match.FileType != "" && p.Match.FileType != "unknown" && p.Match.FileType != "off" {
			seen[p.Match.FileType] = true
		}
	}
	providersMu.RUnlock()

	out := make([]string, 0, len(seen))
	for ft := range seen {
		out = append(out, ft)
	}
	sort.Strings(out)
	return out
}
