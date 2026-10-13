package buffer

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Match is a match in the text: line Row, runes [A, B).
type Match struct{ Row, A, B int }

// Search is the search state. View mode, conventional edit, and vim share it. Matches are
// inside one line. Matches with no length do not count.
type Search struct {
	Pattern   string
	Re        *regexp.Regexp // nil if the pattern is empty or bad
	Matches   []Match        // in text order. Valid while Highlight is on.
	Highlight bool           // the UI shows the matches
	Cur       int            // index of the current match, -1 for none
	Backward  bool           // direction of n
}

// CompilePattern compiles a Go regular expression with smartcase. A pattern with no
// uppercase letter matches without regard to case. A letter after a backslash does not
// count, so \S and \W keep the search case-insensitive.
func CompilePattern(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile(smartcase(pattern, pattern))
}

// smartcase puts (?i) before expr if src has no uppercase letter.
func smartcase(expr, src string) string {
	rs := []rune(src)
	for i := 0; i < len(rs); i++ {
		if rs[i] == '\\' {
			i++
			continue
		}
		if unicode.IsUpper(rs[i]) {
			return expr
		}
	}
	return "(?i)" + expr
}

// matchesOf returns the matches of re in lines, with rune offsets.
func matchesOf(re *regexp.Regexp, lines [][]rune) []Match {
	var out []Match
	for row, l := range lines {
		s := string(l)
		locs := re.FindAllStringIndex(s, -1)
		if len(locs) == 0 {
			continue
		}
		ascii := len(s) == len(l)
		pos, runes := 0, 0 // byte position and rune count at that position
		conv := func(off int) int {
			if ascii {
				return off
			}
			runes += utf8.RuneCountInString(s[pos:off])
			pos = off
			return runes
		}
		for _, loc := range locs {
			if loc[0] == loc[1] {
				continue
			}
			a := conv(loc[0])
			out = append(out, Match{row, a, conv(loc[1])})
		}
	}
	return out
}

// SetSearch compiles pattern and finds all matches. It turns the highlight on. With a bad
// pattern, it clears the matches and the highlight and returns the error. An empty pattern
// clears the search.
func (b *Buffer) SetSearch(pattern string, backward bool) error {
	s := &b.Search
	s.Pattern, s.Backward, s.Cur, s.Matches = pattern, backward, -1, nil
	s.Re, s.Highlight = nil, false
	if pattern == "" {
		return nil
	}
	re, err := CompilePattern(pattern)
	if err != nil {
		return err
	}
	s.Re, s.Highlight = re, true
	s.Matches = matchesOf(re, b.Lines)
	return nil
}

// ClearHighlight turns the highlight off. The pattern stays for n and N.
func (b *Buffer) ClearHighlight() {
	b.Search.Highlight, b.Search.Matches, b.Search.Cur = false, nil, -1
}

// Refresh finds the matches again after a change of the text, if the highlight is on.
func (b *Buffer) Refresh() {
	s := &b.Search
	if s.Re == nil || !s.Highlight {
		return
	}
	s.Matches = matchesOf(s.Re, b.Lines)
	s.Cur = -1
	for i, m := range s.Matches {
		if m.Row == b.Row && m.A == b.Col {
			s.Cur = i
			break
		}
	}
}

// Find returns the index of the match after position (row, col) in the given direction. With
// inclusive, a match at the position counts. If it finds none before the end of the text, it
// wraps and sets wrapped. It returns -1 if there are no matches.
func (s *Search) Find(row, col int, backward, inclusive bool) (idx int, wrapped bool) {
	n := len(s.Matches)
	if n == 0 {
		return -1, false
	}
	if !backward {
		for i, m := range s.Matches {
			if before(row, col, m.Row, m.A) || (inclusive && m.Row == row && m.A == col) {
				return i, false
			}
		}
		return 0, true
	}
	for i := n - 1; i >= 0; i-- {
		m := s.Matches[i]
		if before(m.Row, m.A, row, col) || (inclusive && m.Row == row && m.A == col) {
			return i, false
		}
	}
	return n - 1, true
}

// SearchAt moves the cursor to the first match at or after (row, col) in the search
// direction. The UI uses it while the user types a pattern. ok is false with no match.
func (b *Buffer) SearchAt(row, col int) (wrapped, ok bool) {
	return b.searchGo(row, col, b.Search.Backward, true)
}

// SearchJump moves the cursor to the next match in the search direction (reverse: the other
// direction), from the cursor. It finds the matches again and turns the highlight on. ok is
// false with no match.
func (b *Buffer) SearchJump(reverse bool) (wrapped, ok bool) {
	s := &b.Search
	if s.Re == nil {
		return false, false
	}
	s.Highlight = true
	s.Matches = matchesOf(s.Re, b.Lines)
	return b.searchGo(b.Row, b.Col, s.Backward != reverse, false)
}

func (b *Buffer) searchGo(row, col int, backward, inclusive bool) (wrapped, ok bool) {
	s := &b.Search
	idx, wrapped := s.Find(row, col, backward, inclusive)
	if idx < 0 {
		s.Cur = -1
		return false, false
	}
	s.Cur = idx
	m := s.Matches[idx]
	b.Row, b.Col, b.Want = m.Row, m.A, -1
	return wrapped, true
}

// ---- literal find for the find box ----

func runesEqual(a, b rune, matchCase bool) bool {
	if matchCase {
		return a == b
	}
	return unicode.ToLower(a) == unicode.ToLower(b)
}

// FindLiteral returns all matches of q, without overlaps, in text order.
func (b *Buffer) FindLiteral(q string, matchCase bool) []Match {
	qr := []rune(q)
	if len(qr) == 0 {
		return nil
	}
	var out []Match
	for row, l := range b.Lines {
		for i := 0; i+len(qr) <= len(l); i++ {
			ok := true
			for j := range qr {
				if !runesEqual(l[i+j], qr[j], matchCase) {
					ok = false
					break
				}
			}
			if ok {
				out = append(out, Match{row, i, i + len(qr)})
				i += len(qr) - 1
			}
		}
	}
	return out
}

// ReplaceMatch replaces match m with rep. The cursor goes after the new text.
func (b *Buffer) ReplaceMatch(m Match, rep string) {
	if m.Row < 0 || m.Row >= len(b.Lines) || m.A < 0 || m.B > len(b.Lines[m.Row]) || m.A > m.B {
		return
	}
	r := []rune(strings.ReplaceAll(rep, "\n", " "))
	b.Lines[m.Row] = splice(b.Lines[m.Row], m.A, m.B, r)
	b.Row, b.Col, b.Want = m.Row, m.A+len(r), -1
	b.touch()
}

// ReplaceAll replaces every literal match of q with rep and returns the number of changes.
func (b *Buffer) ReplaceAll(q, rep string, matchCase bool) int {
	ms := b.FindLiteral(q, matchCase)
	for i := len(ms) - 1; i >= 0; i-- { // from the end, so that earlier positions stay correct
		b.ReplaceMatch(ms[i], rep)
	}
	return len(ms)
}
