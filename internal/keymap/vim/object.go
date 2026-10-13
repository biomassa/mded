package vim

import (
	"sort"
	"strings"
	"unicode"
)

// region is a part of the text that an operator works on. A char region goes from (r1, c1)
// to (r2, c2), and the end is not in it. A linewise region covers the lines r1 to r2.
type region struct {
	r1, c1, r2, c2 int
	linewise       bool
}

// object finds a text object. name is i or a, then the object char. n is the count.
func (v *Vim) object(name string, n int) (region, bool) {
	inner := name[0] == 'i'
	switch ch := name[1]; ch {
	case 'w', 'W':
		return v.wordObj(inner, ch == 'W', n)
	case 's':
		return v.sentenceObj(inner, n)
	case 'p':
		return v.paraObj(inner, n)
	case '"', '\'', '`':
		return v.quoteObj(rune(ch), inner)
	case '(', ')', 'b':
		return v.bracketObj('(', ')', inner, n)
	case '[', ']':
		return v.bracketObj('[', ']', inner, n)
	case '{', '}', 'B':
		return v.bracketObj('{', '}', inner, n)
	case '<', '>':
		return v.bracketObj('<', '>', inner, n)
	}
	return region{}, false
}

// wordObj finds iw, aw, iW, or aW in the cursor line.
func (v *Vim) wordObj(inner, big bool, n int) (region, bool) {
	b := v.b
	l := b.Lines[b.Row]
	row := b.Row
	if len(l) == 0 {
		return region{r1: row, r2: row}, true
	}
	col := min(b.Col, len(l)-1)
	runEnd := func(i int) int {
		c := cls(l[i], big)
		for i < len(l) && cls(l[i], big) == c {
			i++
		}
		return i
	}
	runStart := func(i int) int {
		c := cls(l[i], big)
		for i > 0 && cls(l[i-1], big) == c {
			i--
		}
		return i
	}
	a, z := runStart(col), runEnd(col)
	if inner {
		for k := 1; k < n && z < len(l); k++ {
			z = runEnd(z)
		}
		return region{r1: row, c1: a, r2: row, c2: z}, true
	}
	if cls(l[col], big) == 0 { // on blanks: the blanks and the next word
		if z < len(l) {
			z = runEnd(z)
		}
	} else if z < len(l) && cls(l[z], big) == 0 { // a word and the blanks after it
		z = runEnd(z)
	} else { // no blanks after the word: take the blanks before it
		for a > 0 && cls(l[a-1], big) == 0 {
			a--
		}
	}
	for k := 1; k < n && z < len(l); k++ {
		z = runEnd(z)
		if z < len(l) && cls(l[z], big) == 0 {
			z = runEnd(z)
		}
	}
	return region{r1: row, c1: a, r2: row, c2: z}, true
}

// sentPiece is one part of the text for the sentence objects. The parts do not overlap. The
// text between two parts (a line break before empty lines) belongs to no part.
type sentPiece struct {
	s, e  int  // offsets in the text with line breaks, e is not in the part
	white bool // blanks, not a sentence
	trail bool // white that as takes after a sentence
	lead  bool // white that as takes before a sentence
	blank bool // the white has a space or a tab (a line break alone does not count)
	brk   bool // the empty lines between two paragraphs
	nl    bool // the line break before the empty lines
}

// sentPieces cuts text into sentences and white parts. A sentence ends at ., !, or ? with
// closing chars ) ] " ' after it, if a blank or the end of the text follows. A paragraph
// boundary (an empty line) ends a sentence too.
func sentPieces(t []rune) []sentPiece {
	var ps []sentPiece
	for i := 0; i < len(t); {
		if unicode.IsSpace(t[i]) {
			j := i
			for j < len(t) && unicode.IsSpace(t[j]) {
				j++
			}
			ps = appendWhite(ps, t, i, j)
			i = j
			continue
		}
		j := i
		for j < len(t) {
			if t[j] == '\n' && j+1 < len(t) && t[j+1] == '\n' {
				break
			}
			if strings.ContainsRune(".!?", t[j]) {
				k := j + 1
				for k < len(t) && strings.ContainsRune(")]\"'", t[k]) {
					k++
				}
				j = k
				if k >= len(t) || unicode.IsSpace(t[k]) {
					break
				}
				continue
			}
			j++
		}
		e := j
		for e > i && unicode.IsSpace(t[e-1]) {
			e--
		}
		ps = append(ps, sentPiece{s: i, e: e})
		i = e
	}
	return ps
}

// appendWhite adds the blanks t[i:j] as one part. Empty lines in them split the blanks into
// the end of the line before, the empty lines, and the indent of the line after.
func appendWhite(ps []sentPiece, t []rune, i, j int) []sentPiece {
	p := -1
	for k := i; k+1 < j; k++ {
		if t[k] == '\n' && t[k+1] == '\n' {
			p = k
			break
		}
	}
	blank := func(a, z int) bool {
		for _, r := range t[a:z] {
			if r == ' ' || r == '\t' {
				return true
			}
		}
		return false
	}
	if p < 0 {
		return append(ps, sentPiece{s: i, e: j, white: true, trail: true, lead: true, blank: blank(i, j)})
	}
	q := j - 1
	for t[q] != '\n' {
		q--
	}
	start := p + 1
	if i == 0 { // empty lines at the start of the text
		start = 0
	}
	if p > i {
		ps = append(ps, sentPiece{s: i, e: p, white: true, trail: true, blank: blank(i, p)})
	}
	if i > 0 {
		ps = append(ps, sentPiece{s: p, e: p + 1, white: true, nl: true})
	}
	ps = append(ps, sentPiece{s: start, e: q + 1, white: true, brk: true})
	if q+1 < j {
		ps = append(ps, sentPiece{s: q + 1, e: j, white: true, lead: true, blank: blank(q+1, j)})
	}
	return ps
}

// sentenceObj finds is or as, over line breaks. is takes the sentence, or the white part,
// at the cursor, and n-1 parts after it. as takes a sentence with the blanks after it, or
// before it if no blanks follow. On blanks, as takes the blanks and the next sentence.
func (v *Vim) sentenceObj(inner bool, n int) (region, bool) {
	b := v.b
	var t []rune
	starts := make([]int, len(b.Lines))
	for r, l := range b.Lines {
		starts[r] = len(t)
		t = append(t, l...)
		if r < len(b.Lines)-1 {
			t = append(t, '\n')
		}
	}
	ps := sentPieces(t)
	if len(ps) == 0 {
		return region{}, false
	}
	off := starts[b.Row] + min(b.Col, len(b.Lines[b.Row]))
	idx := -1
	for k, p := range ps {
		if p.s <= off {
			idx = k // the last part that starts at or before the cursor
		}
	}
	idx = max(idx, 0)
	s, e := ps[idx].s, ps[idx].e
	if ps[idx].brk && n == 1 { // one empty line
		row := max(0, min(len(starts)-1, sort.Search(len(starts), func(i int) bool { return starts[i] > off })-1))
		if starts[row] >= s && starts[row] < e {
			s, e = starts[row], min(starts[row]+1, e)
		}
		return v.pieceRegion(starts, s, e), true
	}
	j := idx + 1
	if inner {
		e = ps[min(idx+n-1, len(ps)-1)].e
	} else {
		trailing := func() bool { return j < len(ps) && ps[j].white && ps[j].trail && ps[j].blank }
		blanks := ps[idx].white
		j = idx
		if blanks { // blanks: they and the next sentence
			j++
		}
		lastTrail := true
		if !ps[idx].brk { // empty lines: only they
			for k := 0; k < n; k++ {
				if j < len(ps) && ps[j].nl { // a paragraph boundary counts as a sentence
					e = ps[j].e
					j++
					if j < len(ps) && ps[j].brk {
						e = ps[j].e
						j++
					}
					lastTrail = true
					continue
				}
				if k > 0 && j+1 < len(ps) && ps[j].white && ps[j].trail && !ps[j+1].white {
					j++ // a line break between two sentences
				}
				if j >= len(ps) || ps[j].white {
					break
				}
				e = ps[j].e
				j++
				lastTrail = false
				if (!blanks || k < n-1) && trailing() {
					e = ps[j].e
					j++
					lastTrail = true
				}
			}
		}
		if !blanks && !lastTrail && idx > 0 && ps[idx-1].white && ps[idx-1].lead {
			// No blanks follow: take the blanks before the sentence, but not a line break.
			w := ps[idx-1]
			s = w.s
			for i := w.e; i > w.s; i-- {
				if t[i-1] == '\n' {
					s = i
					break
				}
			}
		}
	}
	return v.pieceRegion(starts, s, e), true
}

// pieceRegion turns the offsets s and e in the text with line breaks into a region.
// starts has the offset of each line.
func (v *Vim) pieceRegion(starts []int, s, e int) region {
	at := func(o int) (int, int) {
		r := sort.Search(len(starts), func(i int) bool { return starts[i] > o }) - 1
		return r, o - starts[r]
	}
	r1, c1 := at(s)
	r2, c2 := at(e)
	return region{r1: r1, c1: c1, r2: r2, c2: c2}
}

// sentenceRegion changes the region of a sentence object for an operator, as vim does for an
// exclusive motion: a region that ends at a line end ends in column 0 of the next line, and
// then the end goes back to the end of the line before. If only blanks come before the start,
// the region takes whole lines.
func (v *Vim) sentenceRegion(rg region) region {
	b := v.b
	if rg.linewise || (rg.r1 == rg.r2 && rg.c1 == rg.c2) {
		return rg
	}
	if rg.r2 < len(b.Lines)-1 && rg.c2 == len(b.Lines[rg.r2]) {
		rg.r2, rg.c2 = rg.r2+1, 0
	}
	if rg.c2 == 0 && rg.r2 > rg.r1 {
		rg.r2--
		rg.c2 = len(b.Lines[rg.r2])
		if strings.TrimSpace(string(b.Lines[rg.r1][:rg.c1])) == "" {
			return region{r1: rg.r1, r2: rg.r2, linewise: true}
		}
	}
	return rg
}

// blankLine reports whether line row has no text except blanks.
func (v *Vim) blankLine(row int) bool {
	return strings.TrimSpace(string(v.b.Lines[row])) == ""
}

// paraObj finds ip or ap: whole lines.
func (v *Vim) paraObj(inner bool, n int) (region, bool) {
	last := len(v.b.Lines) - 1
	row := v.b.Row
	runEnd := func(r int) int { // last line of the run that starts at r
		bl := v.blankLine(r)
		for r < last && v.blankLine(r+1) == bl {
			r++
		}
		return r
	}
	r1 := row
	for r1 > 0 && v.blankLine(r1-1) == v.blankLine(row) {
		r1--
	}
	r2 := runEnd(row)
	if inner {
		for k := 1; k < n && r2 < last; k++ {
			r2 = runEnd(r2 + 1)
		}
		return region{r1: r1, r2: r2, linewise: true}, true
	}
	onBlank := v.blankLine(row)
	units := n
	if onBlank {
		if r2 >= last {
			return region{}, false
		}
		r2 = runEnd(r2 + 1)
		units--
	} else if r2 < last {
		r2 = runEnd(r2 + 1) // the blank lines after the paragraph
	} else {
		for r1 > 0 && v.blankLine(r1-1) { // none after it: take the blank lines before it
			r1--
		}
	}
	for k := 1; k <= units-1 && r2 < last; k++ {
		r2 = runEnd(r2 + 1)
		if r2 < last && v.blankLine(r2) != v.blankLine(r2+1) {
			r2 = runEnd(r2 + 1)
		}
	}
	return region{r1: r1, r2: r2, linewise: true}, true
}

// quoteObj finds i" a" and the same with ' and ` in the cursor line.
func (v *Vim) quoteObj(q rune, inner bool) (region, bool) {
	b := v.b
	l := b.Lines[b.Row]
	row := b.Row
	var qs []int
	for i := 0; i < len(l); i++ {
		switch l[i] {
		case '\\':
			i++
		case q:
			qs = append(qs, i)
		}
	}
	a, z := -1, -1
	for k := 0; k+1 < len(qs); k += 2 {
		if qs[k] <= b.Col && b.Col <= qs[k+1] {
			a, z = qs[k], qs[k+1]
			break
		}
		if qs[k] > b.Col {
			a, z = qs[k], qs[k+1]
			break
		}
	}
	if a < 0 {
		return region{}, false
	}
	if inner {
		return region{r1: row, c1: a + 1, r2: row, c2: z}, true
	}
	e, s := z+1, a
	if e < len(l) && unicode.IsSpace(l[e]) {
		for e < len(l) && unicode.IsSpace(l[e]) {
			e++
		}
	} else {
		for s > 0 && unicode.IsSpace(l[s-1]) {
			s--
		}
	}
	return region{r1: row, c1: s, r2: row, c2: e}, true
}

// bracketObj finds i( a( and the same for [, {, and <. The cursor must be inside the pair or
// on one of its brackets. n picks the n-th enclosing pair.
func (v *Vim) bracketObj(open, closing rune, inner bool, n int) (region, bool) {
	b := v.b
	p := pos{b.Row, min(b.Col, max(0, len(b.Lines[b.Row])-1))}
	var o pos
	found := false
	for k := 0; k < n; k++ {
		l := b.Lines[p.row]
		switch {
		case k == 0 && p.col < len(l) && l[p.col] == open:
			o, found = p, true
		case k == 0 && p.col < len(l) && l[p.col] == closing:
			o, found = scanOpen(b, p, open, closing)
		default:
			o, found = v.enclosing(p, open, closing)
		}
		if !found {
			return region{}, false
		}
		p = o
	}
	cl, ok := scanClose(b, o, open, closing)
	if !ok {
		return region{}, false
	}
	if !inner {
		return region{r1: o.row, c1: o.col, r2: cl.row, c2: cl.col + 1}, true
	}
	// An open bracket at the line end and a close bracket after blanks only: whole lines.
	if o.col == len(b.Lines[o.row])-1 && cl.row > o.row &&
		strings.TrimSpace(string(b.Lines[cl.row][:cl.col])) == "" {
		return region{r1: o.row + 1, r2: cl.row - 1, linewise: true}, cl.row-1 >= o.row+1
	}
	return region{r1: o.row, c1: o.col + 1, r2: cl.row, c2: cl.col}, true
}

// enclosing finds the open bracket before p that has no close bracket for it before p.
func (v *Vim) enclosing(p pos, open, closing rune) (pos, bool) {
	b := v.b
	depth := 0
	for row := p.row; row >= 0; row-- {
		l := b.Lines[row]
		start := len(l) - 1
		if row == p.row {
			start = p.col - 1
		}
		for i := start; i >= 0; i-- {
			switch l[i] {
			case closing:
				depth++
			case open:
				if depth == 0 {
					return pos{row, i}, true
				}
				depth--
			}
		}
	}
	return p, false
}
