package vim

import "strings"

// The sentence objects is and as follow vim: this file is a port of findsent, current_sent,
// and the helpers of vim's textobject.c, with the same scan over the text. A sentence ends at
// ., !, or ? with closing chars ) ] " ' after it, if a blank or a line end follows. An empty
// line is a paragraph boundary: it ends a sentence, and it is a sentence of its own, so a run
// of empty lines counts as one unit for the scan.

// sentScan is a scan over the lines. cur is the cursor of the scan.
type sentScan struct {
	l   [][]rune
	cur pos
}

// at returns the char at p, or 0 at the line end (the char that vim calls NUL).
func (s *sentScan) at(p pos) rune {
	if p.row < 0 || p.row >= len(s.l) || p.col < 0 || p.col >= len(s.l[p.row]) {
		return 0
	}
	return s.l[p.row][p.col]
}

// inc moves p one position on, to the line end after the last char. It returns 0 if p is on a
// char, 2 if p is at the line end now, 1 if p is on the next line now, and -1 at the text end.
func (s *sentScan) inc(p *pos) int {
	n := len(s.l[p.row])
	if p.col < n {
		p.col++
		if p.col < n {
			return 0
		}
		return 2
	}
	if p.row < len(s.l)-1 {
		p.row++
		p.col = 0
		return 1
	}
	return -1
}

// incl is inc, but it does not stop at the line end of a line that has chars.
func (s *sentScan) incl(p *pos) int {
	r := s.inc(p)
	if r >= 1 && p.col > 0 {
		r = s.inc(p)
	}
	return r
}

// dec moves p one position back. From the start of a line it goes to the line end of the
// line before. It returns 0 on the same line, 1 on the line before, and -1 at the text start.
func (s *sentScan) dec(p *pos) int {
	if p.col > 0 {
		p.col--
		return 0
	}
	if p.row > 0 {
		p.row--
		p.col = len(s.l[p.row])
		return 1
	}
	return -1
}

// decl is dec, but it does not stop at the line end of a line that has chars.
func (s *sentScan) decl(p *pos) int {
	r := s.dec(p)
	if r == 1 && p.col > 0 {
		r = s.dec(p)
	}
	return r
}

// startPS reports whether line row starts a paragraph: it is empty, or it starts with a form
// feed.
func (s *sentScan) startPS(row int) bool {
	return len(s.l[row]) == 0 || s.l[row][0] == '\f'
}

func isBlank(c rune) bool { return c == ' ' || c == '\t' }

// anyOf reports whether c is one of the chars of set. The line end (0) is in no set.
func anyOf(set string, c rune) bool { return c != 0 && strings.ContainsRune(set, c) }

// findsent moves the cursor of the scan to the start of the next sentence (fwd) or of the
// sentence before (not fwd), count times. It reports false if it cannot, and then the cursor
// stays.
func (s *sentScan) findsent(fwd bool, count int) bool {
	p := s.cur
	step := func(q *pos) int {
		if fwd {
			return s.incl(q)
		}
		return s.decl(q)
	}
	noskip := false
	for count > 0 {
		count--
		found := false
		switch {
		case s.at(p) == 0: // on an empty line: go to a line that has chars
			for step(&p) != -1 && s.at(p) == 0 {
			}
			found = fwd
		case fwd && p.col == 0 && s.startPS(p.row):
			if p.row == len(s.l)-1 {
				return false
			}
			p.row++
			found = true
		case !fwd:
			s.decl(&p)
		}
		if !found {
			// Go back to the last char that is not a blank and not a sentence end.
			foundDot := false
			for {
				c := s.at(p)
				if !isBlank(c) && !anyOf(".!?)]\"'", c) {
					break
				}
				t := p
				if s.decl(&t) == -1 || (len(s.l[t.row]) == 0 && fwd) {
					break
				}
				if foundDot {
					break
				}
				if anyOf(".!?", c) {
					foundDot = true
				}
				if anyOf(")]\"'", c) && !anyOf(".!?)]\"'", s.at(t)) {
					break
				}
				s.decl(&p)
			}
			startRow := p.row
			for { // find the end of the sentence
				c := s.at(p)
				if c == 0 || (p.col == 0 && s.startPS(p.row)) {
					if !fwd && p.row != startRow {
						p.row++
					}
					break
				}
				if anyOf(".!?", c) {
					t := p
					var d rune
					for {
						if s.inc(&t) == -1 {
							d = -1
							break
						}
						if d = s.at(t); !anyOf(")]\"'", d) {
							break
						}
					}
					if d == -1 || d == ' ' || d == '\t' || d == 0 {
						p = t
						if s.at(p) == 0 { // the line end: go to the next line
							s.inc(&p)
						}
						break
					}
				}
				if step(&p) == -1 {
					if count > 0 {
						return false
					}
					noskip = true
					break
				}
			}
		}
		for !noskip && isBlank(s.at(p)) { // skip the blanks
			if s.incl(&p) == -1 {
				break
			}
		}
	}
	s.cur = p
	return true
}

// findFirstBlank moves p back over the blanks before it, to the first of them.
func (s *sentScan) findFirstBlank(p *pos) {
	for s.decl(p) != -1 {
		if !isBlank(s.at(*p)) {
			s.incl(p)
			break
		}
	}
}

// findsentForward moves the cursor over count units: sentences and the white parts between
// them. The cursor is at the start of a sentence if atStart.
func (s *sentScan) findsentForward(count int, atStart bool) {
	for count > 0 {
		count--
		s.findsent(true, 1)
		if atStart {
			s.findFirstBlank(&s.cur)
		}
		if count == 0 || atStart {
			s.decl(&s.cur)
		}
		atStart = !atStart
	}
}

// sentRes is the result of a sentence object: the object goes from start to end. incl tells
// that the end is in the object (for an operator). In visual mode with extend set, only the
// cursor moves to end, and the anchor stays.
type sentRes struct {
	start, end pos
	incl       bool
	extend     bool
}

// sentenceObj finds is or as for the cursor, with count n. vis is the anchor of the visual
// selection, or nil if there is no selection of more than one char.
func (v *Vim) sentenceObj(inner bool, n int, visual bool, anchor pos) sentRes {
	b := v.b
	s := &sentScan{l: b.Lines}
	cur := pos{b.Row, min(b.Col, len(b.Lines[b.Row]))}
	s.cur = cur
	startPos := cur
	p := startPos
	s.findsent(true, 1) // the start of the next sentence

	// extend moves the cursor of a visual selection that has more than one char.
	extend := func() sentRes {
		count := n
		if startPos.row < anchor.row || (startPos.row == anchor.row && startPos.col < anchor.col) {
			// The cursor is at the start of the selection. Find out where it is: in the
			// white part before a sentence, in a sentence, or at the start of a sentence.
			atStartSent := true
			s.decl(&p)
			for p.row < s.cur.row || (p.row == s.cur.row && p.col < s.cur.col) {
				if !isBlank(s.at(p)) {
					atStartSent = false
					break
				}
				s.incl(&p)
			}
			if !atStartSent {
				s.findsent(false, 1)
				if s.cur == startPos {
					atStartSent = true
				} else {
					s.findsent(true, 1)
				}
			}
			if !inner {
				count *= 2
			}
			for ; count > 0; count-- {
				if atStartSent {
					s.findFirstBlank(&s.cur)
				}
				if !atStartSent || (inner && !isBlank(s.at(s.cur))) {
					s.findsent(false, 1)
				}
				atStartSent = !atStartSent
			}
			return sentRes{end: s.cur, extend: true}
		}
		// The cursor is at the end of the selection: just before a sentence, in or before the
		// white part before a sentence, or in a sentence.
		s.incl(&p)
		atStartSent := true
		if p != s.cur {
			atStartSent = false
			for p.row < s.cur.row || (p.row == s.cur.row && p.col < s.cur.col) {
				if !isBlank(s.at(p)) {
					atStartSent = true
					break
				}
				s.incl(&p)
			}
			if atStartSent {
				s.findsent(false, 1)
			} else {
				s.cur = startPos
			}
		}
		if !inner {
			count *= 2
		}
		s.findsentForward(count, atStartSent)
		return sentRes{end: s.cur, extend: true}
	}

	if visual && startPos != anchor { // a selection of more than one char: extend it
		return extend()
	}

	// If the cursor started on a blank, check if it is just before the start of the next
	// sentence.
	for isBlank(s.at(p)) {
		s.incl(&p)
	}
	startBlank := p == s.cur
	if startBlank {
		s.findFirstBlank(&startPos) // go back to the first blank
	} else {
		s.findsent(false, 1)
		startPos = s.cur
	}
	ncount := n
	if !inner {
		ncount = n * 2
	} else if startBlank {
		ncount--
	}
	if ncount > 0 {
		s.findsentForward(ncount, true)
	} else {
		s.decl(&s.cur)
	}
	if !inner {
		// If the blank in front of the sentence is in the object, leave out the blanks at its
		// end. If no blanks follow, take the blanks before the sentence.
		if startBlank {
			s.findFirstBlank(&s.cur)
			if isBlank(s.at(s.cur)) {
				s.decl(&s.cur)
			}
		} else if !isBlank(s.at(s.cur)) {
			s.findFirstBlank(&startPos)
		}
	}
	if visual {
		if startPos == s.cur { // do not stay on one blank before a sentence
			return extend()
		}
		return sentRes{start: startPos, end: s.cur}
	}
	// Take the line break after the sentence, if there is one.
	r := sentRes{start: startPos}
	r.incl = s.incl(&s.cur) == -1
	r.end = s.cur
	return r
}

// inIndent reports whether only blanks come before column col of line row.
func (v *Vim) inIndent(row, col int) bool {
	for _, r := range v.b.Lines[row] {
		if !isBlank(r) {
			break
		}
		if col <= 0 {
			return true
		}
		col--
	}
	return col <= 0
}

// sentenceRegion makes the region of a sentence object for operator op, as vim does. The end
// of an exclusive motion in column 0 goes back to the end of the line before, and if only
// blanks come before the start, the region takes whole lines. A delete of more than one line
// takes whole lines too if only blanks follow the end and only blanks come before the start.
func (v *Vim) sentenceRegion(r sentRes, op string) region {
	b := v.b
	st, e, incl := r.start, r.end, r.incl
	if !incl && e.col == 0 && e.row > st.row {
		e.row--
		if v.inIndent(st.row, st.col) {
			return region{r1: st.row, r2: e.row, linewise: true}
		}
		e.col = len(b.Lines[e.row])
		if e.col > 0 {
			e.col--
			incl = true
		}
	}
	if op == "d" && e.row > st.row {
		l := b.Lines[e.row]
		i := min(e.col, len(l))
		if i < len(l) && incl {
			i++
		}
		if strings.TrimLeft(string(l[min(i, len(l)):]), " \t") == "" && v.inIndent(st.row, st.col) {
			return region{r1: st.row, r2: e.row, linewise: true}
		}
	}
	end := e.col
	if incl {
		end++
	}
	return region{r1: st.row, c1: st.col, r2: e.row, c2: min(end, len(b.Lines[e.row]))}
}
