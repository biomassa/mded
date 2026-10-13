package vim

import (
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
