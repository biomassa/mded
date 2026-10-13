package buffer

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// SubFlags are the flags of :s.
type SubFlags struct {
	Global     bool // g: all matches in a line, not only the first
	IgnoreCase bool // i
	MatchCase  bool // I
	Confirm    bool // c: the caller asks for each match. Substitute ignores it.
}

// ParseSubFlags reads the flags of :s from a string such as "gi". It returns an error for an
// unknown flag.
func ParseSubFlags(s string) (SubFlags, error) {
	var f SubFlags
	for _, r := range s {
		switch r {
		case 'g':
			f.Global = true
		case 'i':
			f.IgnoreCase = true
		case 'I':
			f.MatchCase = true
		case 'c':
			f.Confirm = true
		default:
			return f, fmt.Errorf("trailing characters: %s", s)
		}
	}
	return f, nil
}

// SubResult is the result of :s.
type SubResult struct {
	Count int // substitutions
	Lines int // lines that changed
}

// VimToGo translates a vim pattern to Go syntax. It changes \( \) to groups, \| to
// alternation, \< and \> to \b, and \{n,m} to {n,m}. Other text passes through.
func VimToGo(p string) string {
	rs := []rune(p)
	var sb strings.Builder
	for i := 0; i < len(rs); i++ {
		if rs[i] != '\\' || i+1 >= len(rs) {
			sb.WriteRune(rs[i])
			continue
		}
		i++
		switch n := rs[i]; n {
		case '(', ')', '|':
			sb.WriteRune(n)
		case '<', '>':
			sb.WriteString(`\b`)
		case '{':
			j := i + 1
			for j < len(rs) && rs[j] != '}' {
				j++
			}
			body := string(rs[i+1 : j])
			if j >= len(rs) || strings.HasPrefix(body, "-") { // not a plain count: keep as is
				sb.WriteString(`\{`)
				continue
			}
			sb.WriteString("{" + strings.TrimSuffix(body, `\`) + "}")
			i = j
		default:
			sb.WriteRune('\\')
			sb.WriteRune(n)
		}
	}
	return sb.String()
}

// repTok is one part of a replacement: text, a group (n >= 0), or a line break.
type repTok struct {
	text string
	n    int // group number, or -1 for text
	nl   bool
}

// parseReplacement reads a :s replacement. & and \0 are the whole match, \1 to \9 are
// groups, \& is a literal &, \n and \r are a line break, \t is a tab, and \\ is a backslash.
func parseReplacement(rep string) []repTok {
	var out []repTok
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			out = append(out, repTok{text: lit.String(), n: -1})
			lit.Reset()
		}
	}
	rs := []rune(rep)
	for i := 0; i < len(rs); i++ {
		switch c := rs[i]; {
		case c == '&':
			flush()
			out = append(out, repTok{n: 0})
		case c == '\\' && i+1 < len(rs):
			i++
			switch d := rs[i]; {
			case d >= '0' && d <= '9':
				flush()
				out = append(out, repTok{n: int(d - '0')})
			case d == 'n' || d == 'r':
				flush()
				out = append(out, repTok{nl: true, n: -1})
			case d == 't':
				lit.WriteRune('\t')
			default: // \& \\ \/ and any other escaped rune stand for themselves
				lit.WriteRune(d)
			}
		default:
			lit.WriteRune(c)
		}
	}
	flush()
	return out
}

// expand builds the replacement text for one match. A line break is "\n".
func expand(toks []repTok, groups []string) string {
	var sb strings.Builder
	for _, t := range toks {
		switch {
		case t.nl:
			sb.WriteByte('\n')
		case t.n >= 0:
			if t.n < len(groups) {
				sb.WriteString(groups[t.n])
			}
		default:
			sb.WriteString(t.text)
		}
	}
	return sb.String()
}

// subMatch is a match in a line of the original text, with its groups.
type subMatch struct {
	a, b   int // runes
	groups []string
}

// SubJob is one run of :s. It goes through the matches one by one, so that the caller can
// ask the user (flag c). Call Next, then Replace or Skip, until Next reports false. Call
// Finish at the end. The whole run is one undo step.
type SubJob struct {
	b        *Buffer
	re       *regexp.Regexp
	toks     []repTok
	rep      string
	global   bool
	next     int // next line to scan, in current line numbers
	end      int // last line of the range, in current line numbers
	origRow  int // line of the matches in ms, as it was before changes
	ms       []subMatch
	idx      int
	rowOff   int // lines that earlier replacements added in this line
	colDelta int // shift of the original columns on the current line
	touched  bool
	have     bool
	done     bool
	res      SubResult
	lastRow  int
}

// NewSubstitute starts a :s run over lines r1 to r2. The pattern is a vim pattern (see
// VimToGo). An empty pattern uses the last search pattern. It opens an undo group, which
// Finish closes.
func (b *Buffer) NewSubstitute(r1, r2 int, pat, rep string, fl SubFlags) (*SubJob, error) {
	var expr string
	switch {
	case pat == "" && b.Search.Pattern == "":
		return nil, fmt.Errorf("no previous regular expression")
	case pat == "":
		expr = b.Search.Pattern // the last search pattern is Go syntax already
	default:
		expr = VimToGo(pat)
	}
	switch {
	case fl.IgnoreCase:
		expr = "(?i)" + expr
	case fl.MatchCase:
	case pat == "":
		expr = smartcase(expr, b.Search.Pattern)
	default:
		expr = smartcase(expr, pat)
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, err
	}
	r1, r2 = b.rowSpan(r1, r2)
	b.BeginChange()
	return &SubJob{b: b, re: re, toks: parseReplacement(rep), rep: rep, global: fl.Global,
		next: r1, end: r2, lastRow: -1}, nil
}

// Rep returns the replacement text as the user typed it, for the prompt.
func (j *SubJob) Rep() string { return j.rep }

// loadRow finds the matches of the next line. It reports false at the end of the range.
func (j *SubJob) loadRow() bool {
	b := j.b
	if j.next > j.end || j.next >= len(b.Lines) {
		return false
	}
	j.origRow = j.next
	j.next++
	j.ms, j.idx, j.rowOff, j.colDelta, j.touched = nil, 0, 0, 0, false
	s := string(b.Lines[j.origRow])
	n := -1
	if !j.global {
		n = 1
	}
	pos, runes := 0, 0
	conv := func(off int) int {
		runes += utf8.RuneCountInString(s[pos:off])
		pos = off
		return runes
	}
	for _, loc := range j.re.FindAllStringSubmatchIndex(s, n) {
		m := subMatch{a: conv(loc[0]), b: conv(loc[1])}
		for g := 0; g < len(loc)/2; g++ {
			if loc[2*g] >= 0 {
				m.groups = append(m.groups, s[loc[2*g]:loc[2*g+1]])
			} else {
				m.groups = append(m.groups, "")
			}
		}
		j.ms = append(j.ms, m)
	}
	return true
}

// Next finds the next match and puts the cursor at its start. It returns false when no
// match is left. A match that Next returned stays current until Replace or Skip.
func (j *SubJob) Next() (Match, bool) {
	if j.done {
		return Match{}, false
	}
	for j.idx >= len(j.ms) {
		if !j.loadRow() {
			return Match{}, false
		}
	}
	m := j.ms[j.idx]
	j.have = true
	cur := Match{j.origRow + j.rowOff, m.a + j.colDelta, m.b + j.colDelta}
	j.b.Row, j.b.Col, j.b.Want = cur.Row, cur.A, -1
	return cur, true
}

// Skip leaves the current match as it is.
func (j *SubJob) Skip() {
	if j.have {
		j.idx++
		j.have = false
	}
}

// Replace replaces the current match.
func (j *SubJob) Replace() {
	if !j.have {
		return
	}
	b := j.b
	m := j.ms[j.idx]
	row := j.origRow + j.rowOff
	a, z := m.a+j.colDelta, m.b+j.colDelta
	parts := strings.Split(expand(j.toks, m.groups), "\n")
	line := b.Lines[row]
	if len(parts) == 1 {
		ins := []rune(parts[0])
		b.Lines[row] = splice(line, a, z, ins)
		j.colDelta += len(ins) - (z - a)
		j.lastRow = row
	} else {
		k := len(parts) - 1
		rows := make([][]rune, 0, len(parts))
		rows = append(rows, splice(line[:a:a], a, a, []rune(parts[0])))
		for _, p := range parts[1 : len(parts)-1] {
			rows = append(rows, []rune(p))
		}
		tail := []rune(parts[k])
		rows = append(rows, splice(tail, len(tail), len(tail), line[z:]))
		b.Lines = append(b.Lines[:row], append(rows, b.Lines[row+1:]...)...)
		b.shift(Shift{At: row + 1, N: k})
		j.rowOff += k
		j.next += k
		j.end += k
		j.colDelta = len(tail) - m.b
		j.lastRow = row + k
	}
	b.touch()
	j.res.Count++
	if !j.touched {
		j.touched = true
		j.res.Lines++
	}
	j.idx++
	j.have = false
}

// ReplaceRest replaces the current match and all matches after it (the answer "a").
func (j *SubJob) ReplaceRest() {
	for {
		if !j.have {
			if _, ok := j.Next(); !ok {
				return
			}
		}
		j.Replace()
	}
}

// Result returns the counts so far.
func (j *SubJob) Result() SubResult { return j.res }

// Finish ends the run and closes the undo group. The cursor goes to the start of the last
// changed line. It returns the counts. A second call does nothing.
func (j *SubJob) Finish() SubResult {
	if !j.done {
		j.done, j.have = true, false
		if j.lastRow >= 0 {
			j.b.Row, j.b.Col, j.b.Want = j.lastRow, 0, -1
		}
		j.b.EndChange()
		j.b.Refresh()
	}
	return j.res
}

// Substitute runs :s over lines r1 to r2 with no questions. It is one undo step. Use
// NewSubstitute for the flag c.
func (b *Buffer) Substitute(r1, r2 int, pat, rep string, fl SubFlags) (SubResult, error) {
	j, err := b.NewSubstitute(r1, r2, pat, rep, fl)
	if err != nil {
		return SubResult{}, err
	}
	j.ReplaceRest()
	return j.Finish(), nil
}
