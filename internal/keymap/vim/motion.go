package vim

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/biomassa/mded/internal/buffer"
)

// mres is the result of a motion. The motion does not move the cursor.
type mres struct {
	row, col int
	linewise bool // the motion covers whole lines
	incl     bool // the char at the target belongs to the range
	fail     bool
	jump     bool // the motion sets the previous-position mark
	keepWant bool // the motion sets the wanted column to want
	want     int
	prompt   bool // the motion waits for a search prompt
}

// cls returns the class of a char for word motions: 0 blank, 1 punctuation, 2 word char.
// With big, all chars that are not blank are in class 1.
func cls(r rune, big bool) int {
	switch {
	case r == ' ' || r == '\t' || r == 0 || unicode.IsSpace(r):
		return 0
	case big || !buffer.IsWordRune(r):
		return 1
	}
	return 2
}

// fwdWord finds the start of the next word. With stopEOL, it stops at the end of the line
// instead of it moving to the next line.
func fwdWord(b *buffer.Buffer, p pos, big, stopEOL bool) pos {
	row, col := p.row, p.col
	l := b.Lines[row]
	if col < len(l) {
		if sc := cls(l[col], big); sc != 0 {
			for col < len(l) && cls(l[col], big) == sc {
				col++
			}
		}
	}
	for {
		l = b.Lines[row]
		if col >= len(l) {
			if stopEOL || row == len(b.Lines)-1 {
				return pos{row, len(l)}
			}
			row++
			col = 0
			if len(b.Lines[row]) == 0 {
				return pos{row, 0}
			}
			continue
		}
		if cls(l[col], big) != 0 {
			return pos{row, col}
		}
		col++
	}
}

// bckWord finds the start of the word before p. ok is false at the start of the text.
func bckWord(b *buffer.Buffer, p pos, big bool) (pos, bool) {
	row, col := p.row, p.col
	if row == 0 && col == 0 {
		return p, false
	}
	for {
		switch {
		case col > 0:
			col--
		case row == 0:
			return pos{0, 0}, true
		default:
			row--
			col = len(b.Lines[row])
			if col == 0 {
				return pos{row, 0}, true
			}
			col--
		}
		if col < len(b.Lines[row]) && cls(b.Lines[row][col], big) != 0 {
			break
		}
	}
	l := b.Lines[row]
	sc := cls(l[col], big)
	for col > 0 && cls(l[col-1], big) == sc {
		col--
	}
	return pos{row, col}, true
}

// endWord finds the end of the next word. ok is false if there is none.
func endWord(b *buffer.Buffer, p pos, big bool) (pos, bool) {
	row, col := p.row, p.col+1
	for {
		l := b.Lines[row]
		if col >= len(l) {
			if row == len(b.Lines)-1 {
				return p, false
			}
			row++
			col = 0
			continue
		}
		if cls(l[col], big) != 0 {
			break
		}
		col++
	}
	l := b.Lines[row]
	sc := cls(l[col], big)
	for col+1 < len(l) && cls(l[col+1], big) == sc {
		col++
	}
	return pos{row, col}, true
}

// bckEnd finds the end of the word before p. ok is false at the start of the text.
func bckEnd(b *buffer.Buffer, p pos, big bool) (pos, bool) {
	row, col := p.row, p.col
	if l := b.Lines[row]; col < len(l) && cls(l[col], big) != 0 {
		sc := cls(l[col], big)
		for col > 0 && cls(l[col-1], big) == sc {
			col--
		}
	}
	for {
		switch {
		case col > 0:
			col--
		case row == 0:
			return p, false
		default:
			row--
			col = len(b.Lines[row])
			if col == 0 {
				return pos{row, 0}, true
			}
			col--
		}
		if col < len(b.Lines[row]) && cls(b.Lines[row][col], big) != 0 {
			return pos{row, col}, true
		}
	}
}

// findChar finds the n-th occurrence of ch in l, from col, in the direction of the find
// command kind (f F t T). repeat is true for ; and , : after t and T, the char that is next to
// the cursor does not count.
func findChar(l []rune, col int, kind, ch rune, n int, repeat bool) (int, bool) {
	dir := 1
	if kind == 'F' || kind == 'T' {
		dir = -1
	}
	start := col + dir
	if repeat && (kind == 't' || kind == 'T') {
		start += dir
	}
	for i := start; i >= 0 && i < len(l); i += dir {
		if l[i] != ch {
			continue
		}
		if n--; n == 0 {
			switch kind {
			case 't':
				return i - 1, true
			case 'T':
				return i + 1, true
			}
			return i, true
		}
	}
	return 0, false
}

// matchBracket finds the bracket that pairs with the first bracket at or after col in the line.
func matchBracket(b *buffer.Buffer, p pos) (pos, bool) {
	const open, closing = "([{", ")]}"
	l := b.Lines[p.row]
	i := p.col
	for i < len(l) && !strings.ContainsRune(open+closing, l[i]) {
		i++
	}
	if i >= len(l) {
		return p, false
	}
	c := l[i]
	if k := strings.IndexRune(open, c); k >= 0 {
		return scanClose(b, pos{p.row, i}, rune(open[k]), rune(closing[k]))
	}
	k := strings.IndexRune(closing, c)
	return scanOpen(b, pos{p.row, i}, rune(open[k]), rune(closing[k]))
}

// scanClose finds the closing bracket that pairs with the open bracket at p.
func scanClose(b *buffer.Buffer, p pos, open, closing rune) (pos, bool) {
	depth := 0
	for row := p.row; row < len(b.Lines); row++ {
		l := b.Lines[row]
		start := 0
		if row == p.row {
			start = p.col
		}
		for i := start; i < len(l); i++ {
			switch l[i] {
			case open:
				depth++
			case closing:
				if depth--; depth == 0 {
					return pos{row, i}, true
				}
			}
		}
	}
	return p, false
}

// scanOpen finds the open bracket that pairs with the closing bracket at p.
func scanOpen(b *buffer.Buffer, p pos, open, closing rune) (pos, bool) {
	depth := 0
	for row := p.row; row >= 0; row-- {
		l := b.Lines[row]
		start := len(l) - 1
		if row == p.row {
			start = p.col
		}
		for i := start; i >= 0; i-- {
			switch l[i] {
			case closing:
				depth++
			case open:
				if depth--; depth == 0 {
					return pos{row, i}, true
				}
			}
		}
	}
	return p, false
}

// motion computes a motion from the cursor. op is true if an operator waits for it.
func (v *Vim) motion(c *cmd, n int, has, op bool) mres {
	b := v.b
	row, col := b.Row, b.Col
	last := len(b.Lines) - 1
	l := b.Lines[row]
	r := mres{row: row, col: col}
	bad := func() mres { r.fail = true; return r }
	big := c.name == "W" || c.name == "B" || c.name == "E" || c.name == "gE"
	switch c.name {
	case "h":
		if col == 0 {
			return bad()
		}
		r.col = max(0, col-n)
	case "l":
		lim := len(l) - 1
		if op {
			lim = len(l)
		}
		if col >= lim {
			return bad()
		}
		r.col = min(lim, col+n)
	case "j", "k", "ctrl+d", "ctrl+u", "ctrl+f", "ctrl+b":
		d := n
		switch c.name {
		case "k":
			d = -n
		case "ctrl+d":
			d = max(1, v.rows/2)
		case "ctrl+u":
			d = -max(1, v.rows/2)
		case "ctrl+f":
			d = max(1, v.rows-1)
		case "ctrl+b":
			d = -max(1, v.rows-1)
		}
		if (d > 0 && row == last) || (d < 0 && row == 0) {
			return bad()
		}
		t := max(0, min(last, row+d))
		w := b.Want
		if w < 0 {
			w = col
		}
		tl := len(b.Lines[t])
		r.row, r.col, r.linewise, r.keepWant, r.want = t, min(w, max(0, tl-1)), true, true, w
		if v.visual() && w >= wantEOL {
			r.col = tl
		}
	case "0":
		r.col = 0
	case "^":
		r.col = firstNonBlank(l)
	case "$":
		t := row + n - 1
		if t > last {
			return bad()
		}
		tl := len(b.Lines[t])
		r.row, r.col, r.incl, r.keepWant, r.want = t, max(0, tl-1), true, true, wantEOL
		if v.visual() {
			r.col = tl
		}
	case "w", "W":
		p, moved := pos{row, col}, false
		for i := 0; i < n; i++ {
			np := fwdWord(b, p, big, op && i == n-1)
			if np == p {
				break
			}
			p, moved = np, true
		}
		if !moved {
			return bad()
		}
		r.row, r.col = p.row, p.col
	case "b", "B":
		p, moved := pos{row, col}, false
		for i := 0; i < n; i++ {
			np, ok := bckWord(b, p, big)
			if !ok {
				break
			}
			p, moved = np, true
		}
		if !moved {
			return bad()
		}
		r.row, r.col = p.row, p.col
	case "e", "E", "ge", "gE":
		p, moved := pos{row, col}, false
		for i := 0; i < n; i++ {
			var np pos
			var ok bool
			if c.name[0] == 'g' {
				np, ok = bckEnd(b, p, big)
			} else {
				np, ok = endWord(b, p, big)
			}
			if !ok {
				break
			}
			p, moved = np, true
		}
		if !moved {
			return bad()
		}
		r.row, r.col, r.incl = p.row, p.col, true
	case "gg", "G":
		t := 0
		if c.name == "G" {
			t = last
		}
		if has {
			t = n - 1
		}
		r.row, r.linewise, r.jump = max(0, min(last, t)), true, true
		r.col = firstNonBlank(b.Lines[r.row])
	case "f", "F", "t", "T":
		i, ok := findChar(l, col, rune(c.name[0]), c.arg, n, false)
		if !ok {
			return bad()
		}
		v.lastFind.ok, v.lastFind.cmd, v.lastFind.ch = true, rune(c.name[0]), c.arg
		r.col, r.incl = i, c.name == "f" || c.name == "t"
	case ";", ",":
		if !v.lastFind.ok {
			return bad()
		}
		kind := v.lastFind.cmd
		if c.name == "," {
			kind = map[rune]rune{'f': 'F', 'F': 'f', 't': 'T', 'T': 't'}[kind]
		}
		i, ok := findChar(l, col, kind, v.lastFind.ch, n, true)
		if !ok {
			return bad()
		}
		r.col, r.incl = i, kind == 'f' || kind == 't'
	case "%":
		if has {
			t := (n*len(b.Lines) + 99) / 100
			r.row, r.linewise, r.jump = max(0, min(last, t-1)), true, true
			r.col = firstNonBlank(b.Lines[r.row])
			break
		}
		p, ok := matchBracket(b, pos{row, col})
		if !ok {
			return bad()
		}
		r.row, r.col, r.incl, r.jump = p.row, p.col, true, true
	case "H", "M", "L":
		top := max(0, min(b.Off, last))
		bottom := min(top+v.rows-1, last)
		t := top + n - 1
		switch c.name {
		case "H":
			t = min(t, bottom)
		case "L":
			t = max(bottom-(n-1), top)
		default:
			t = top + (bottom-top)/2
		}
		r.row, r.linewise, r.jump = t, true, true
		r.col = firstNonBlank(b.Lines[t])
	case "}":
		rr := row
		if row == last && col >= len(l) {
			return bad()
		}
		for i := 0; i < n; i++ {
			for rr <= last && len(b.Lines[rr]) == 0 {
				rr++
			}
			for rr <= last && len(b.Lines[rr]) != 0 {
				rr++
			}
			if rr > last {
				break
			}
		}
		r.jump = true
		if rr > last {
			r.row, r.col = last, len(b.Lines[last])
		} else {
			r.row, r.col = rr, 0
		}
	case "{":
		rr := row
		if row == 0 && col == 0 {
			return bad()
		}
		for i := 0; i < n; i++ {
			for rr >= 0 && len(b.Lines[rr]) == 0 {
				rr--
			}
			for rr >= 0 && len(b.Lines[rr]) != 0 {
				rr--
			}
			if rr < 0 {
				break
			}
		}
		r.jump, r.row, r.col = true, max(rr, 0), 0
	case "n", "N":
		p, ok := v.searchStep(c.name == "N", n)
		if !ok {
			return bad()
		}
		r.row, r.col, r.jump = p.row, p.col, true
	case "*", "#":
		p, ok := v.starSearch(c.name == "#")
		if !ok {
			return bad()
		}
		r.row, r.col, r.jump = p.row, p.col, true
	case "'", "`":
		name := c.arg
		if name == '`' {
			name = '\''
		}
		p, ok := v.marks[name]
		if !ok {
			v.status("mark not set", true)
			return bad()
		}
		p.row = max(0, min(last, p.row))
		r.jump = true
		if c.name == "'" {
			r.row, r.linewise = p.row, true
			r.col = firstNonBlank(b.Lines[p.row])
		} else {
			r.row, r.col = p.row, max(0, min(p.col, len(b.Lines[p.row])))
		}
	case "/", "?":
		if !v.haveTgt {
			r.prompt = true
			return r
		}
		v.haveTgt = false
		r.row, r.col, r.jump = v.target.row, v.target.col, true
	default:
		return bad()
	}
	return r
}

// goTo moves the cursor to the result of a motion.
func (v *Vim) goTo(r mres) {
	b := v.b
	if r.fail {
		v.fail()
		return
	}
	if r.jump {
		v.marks['\''] = pos{b.Row, b.Col}
	}
	b.Row, b.Col, b.Want = r.row, r.col, -1
	if r.keepWant {
		b.Want = r.want
	}
}

// searchStep jumps n times to the next match of the last pattern. reverse goes against the
// direction of the last search. It returns the position and leaves the cursor where it was.
func (v *Vim) searchStep(reverse bool, n int) (pos, bool) {
	b := v.b
	start := pos{b.Row, b.Col}
	defer func() { b.Row, b.Col = start.row, start.col }()
	s := &b.Search
	if s.Re == nil {
		v.status("no previous regular expression", true)
		return start, false
	}
	for i := 0; i < n; i++ {
		wrapped, ok := b.SearchJump(reverse)
		if !ok {
			v.status("pattern not found: "+s.Pattern, true)
			return start, false
		}
		if wrapped {
			v.wrapMessage(s.Backward != reverse)
		}
	}
	return pos{b.Row, b.Col}, true
}

// wrapMessage tells the user that a search went around the end of the text.
func (v *Vim) wrapMessage(backward bool) {
	if backward {
		v.status("search hit TOP, continuing at BOTTOM", false)
		return
	}
	v.status("search hit BOTTOM, continuing at TOP", false)
}

// starSearch searches for the word under the cursor, forward or backward.
func (v *Vim) starSearch(backward bool) (pos, bool) {
	b := v.b
	l := b.Lines[b.Row]
	i := b.Col
	for i < len(l) && !buffer.IsWordRune(l[i]) {
		i++
	}
	if i >= len(l) {
		v.status("no string under cursor", true)
		return pos{}, false
	}
	s, e := i, i
	for s > 0 && buffer.IsWordRune(l[s-1]) {
		s--
	}
	for e < len(l) && buffer.IsWordRune(l[e]) {
		e++
	}
	word := string(l[s:e])
	pat := regexp.QuoteMeta(word)
	if len(word) == len(l[s:e]) { // \b knows ASCII only
		pat = `\b` + pat + `\b`
	}
	start := pos{b.Row, b.Col}
	if err := b.SetSearch(pat, backward); err != nil {
		v.status("bad pattern: "+err.Error(), true)
		return start, false
	}
	b.Col = s
	wrapped, ok := b.SearchJump(false)
	p := pos{b.Row, b.Col}
	b.Row, b.Col = start.row, start.col
	if !ok {
		v.status(fmt.Sprintf("pattern not found: %s", pat), true)
		return start, false
	}
	if wrapped {
		v.wrapMessage(backward)
	}
	return p, true
}
