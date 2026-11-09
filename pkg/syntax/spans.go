package syntax

import (
	"sort"
	"unicode"
	"unicode/utf8"
)

// BuildMatch converts byte spans for one line into the display boundary map.
// Overlapping spans are painted in reply order, so a later span wins. The
// returned map always has an entry at index 0 when the line is non-empty.
func BuildMatch(line []byte, spans []Span) LineMatch {
	if len(spans) == 0 {
		return nil
	}

	ordered := make([]Span, 0, len(spans))
	offsets := make([]int, 0, len(spans)*2)
	for _, s := range spans {
		start, end := s.StartByte, s.EndByte
		if start < 0 {
			start = 0
		}
		if end > len(line) {
			end = len(line)
		}
		if end <= start {
			continue
		}
		s.StartByte = start
		s.EndByte = end
		ordered = append(ordered, s)
		offsets = append(offsets, start, end)
	}
	if len(ordered) == 0 {
		return nil
	}

	sort.Ints(offsets)

	positions := make(map[int]int, len(offsets))
	pos, chars := 0, 0
	for _, off := range offsets {
		if off < pos {
			continue
		}
		for pos < off {
			r, size := utf8.DecodeRune(line[pos:])
			if size == 0 {
				break
			}
			if !isMark(r) {
				chars++
			}
			pos += size
		}
		positions[off] = chars
	}

	charCount := CharacterCount(line)
	if charCount == 0 {
		return nil
	}
	painted := make([]Group, charCount)
	for _, s := range ordered {
		start := positions[s.StartByte]
		end := positions[s.EndByte]
		if end > charCount {
			end = charCount
		}
		for i := start; i < end; i++ {
			painted[i] = s.Group
		}
	}

	match := make(LineMatch)
	cur := painted[0]
	match[0] = cur
	for i := 1; i < len(painted); i++ {
		if painted[i] != cur {
			cur = painted[i]
			match[i] = cur
		}
	}
	return match
}

// CharacterCount counts base runes plus their combining marks as one
// character, matching micro's cursor and display indexing.
func CharacterCount(b []byte) int {
	n := 0
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if size == 0 {
			break
		}
		if !isMark(r) {
			n++
		}
		b = b[size:]
	}
	return n
}

func isMark(r rune) bool {
	return unicode.In(r, unicode.Mark)
}
