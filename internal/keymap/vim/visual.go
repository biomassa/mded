package vim

import (
	"github.com/charmbracelet/x/ansi"

	"github.com/biomassa/mded/internal/buffer"
	"github.com/biomassa/mded/internal/keymap"
)

// visRep is the size of a visual selection. The dot command makes a selection of the same
// size from the cursor, then runs the change again.
type visRep struct {
	linewise bool
	lines    int  // number of lines
	cols     int  // charwise: display columns. One line: the width of the selection. More lines: the last display column of the last char.
	eol      bool // charwise: the selection goes to the end of the last line ($)
}

// vimTabstop is the width of a tab in the display columns of a visual span. It is the default
// tabstop of vim. The tab width of the editor does not change it.
const vimTabstop = 8

// vcols returns the first display column of char i of line l, and the first column after it.
// A tab goes to the next tabstop. A wide rune takes 2 columns. The line end (i at or after
// len(l)) is one column wide.
func vcols(l []rune, i int) (first, next int) {
	v := 0
	for k := 0; k < len(l) && k < i; k++ {
		v = vnext(v, l[k])
	}
	if i >= len(l) {
		return v, v + 1
	}
	return v, vnext(v, l[i])
}

// vnext returns the display column after rune r that starts at column v.
func vnext(v int, r rune) int {
	if r == '\t' {
		return v + vimTabstop - v%vimTabstop
	}
	return v + ansi.StringWidth(string(r))
}

// vcolChar returns the index of the char that covers display column vc of line l. After the
// last char it returns len(l), the line end.
func vcolChar(l []rune, vc int) int {
	v := 0
	for i, r := range l {
		v = vnext(v, r)
		if vc < v {
			return i
		}
	}
	return len(l)
}

// selEnds returns the two ends of the selection in text order.
func (v *Vim) selEnds() (a, e pos) {
	b := v.b
	a, e = pos{b.SelRow, b.SelCol}, pos{b.Row, b.Col}
	if e.row < a.row || (e.row == a.row && e.col < a.col) {
		a, e = e, a
	}
	return a, e
}

// visInfo returns the size of the selection. Call it before the selection ends.
func (v *Vim) visInfo(linewise bool) *visRep {
	a, e := v.selEnds()
	r := &visRep{linewise: linewise, lines: e.row - a.row + 1}
	if !linewise {
		r.eol = v.b.Want >= wantEOL
		_, last := vcols(v.b.Lines[e.row], e.col)
		r.cols = last - 1
		if r.lines == 1 {
			first, _ := vcols(v.b.Lines[a.row], a.col)
			r.cols = last - first
		}
	}
	return r
}

// setVisRep makes the running command repeatable on a selection of the size r.
func (v *Vim) setVisRep(r *visRep) {
	if v.cur != nil {
		v.cur.vis = r
	}
}

// startVisRep starts a visual mode at the cursor with the size r. Vim rules: a selection of
// one line has the same number of chars, a selection of more lines has the same number of
// lines and ends in the same column. The end is limited to the text.
func (v *Vim) startVisRep(r *visRep) {
	b := v.b
	row := min(b.Row+r.lines-1, len(b.Lines)-1)
	b.SelRow, b.SelCol, b.SelOn = b.Row, b.Col, true
	b.SelLinewise, b.SelInclusive = r.linewise, !r.linewise
	v.mode = modeVLine
	b.Want = -1
	if !r.linewise {
		v.mode = modeVisual
		l := b.Lines[row]
		n := len(l)
		vc := r.cols
		if r.lines == 1 {
			// The cursor column of a tab is its last column, as in vim in normal mode.
			first, next := vcols(l, b.Col)
			if b.Col < n && l[b.Col] == '\t' {
				first = next - 1
			}
			vc = first + r.cols - 1
		}
		b.Col = vcolChar(l, vc) // as in vim, the end can be on the line break
		if r.eol {
			b.Col, b.Want = n, wantEOL
		}
	}
	b.Row = row
}

// visualReplace runs r in visual mode: every selected char, but not a line break, becomes
// the char of the command. r and enter replace the selection with one line break.
func (v *Vim) visualReplace(c cmd) {
	b := v.b
	linewise := b.SelLinewise
	rep := v.visInfo(linewise)
	r1, c1, r2, c2, ok := b.SelRange()
	v.exitVisual()
	if !ok {
		return
	}
	v.change = true
	v.setVisRep(rep)
	b.Want = -1
	if c.arg == '\n' {
		b.DeleteRange(r1, c1, r2, c2)
		b.SplitLine()
		b.Row, b.Col = r1, c1
		if linewise {
			b.Col = 0
		}
		return
	}
	for r := r1; r <= r2; r++ {
		l := b.Lines[r]
		a, z := 0, len(l)
		if r == r1 {
			a = c1
		}
		if r == r2 {
			z = min(c2, len(l))
		}
		if a >= z {
			continue
		}
		nl := append([]rune(nil), l...)
		for i := a; i < z; i++ {
			nl[i] = c.arg
		}
		b.SetLine(r, nl)
	}
	b.Row, b.Col = r1, c1
}

// visualPut runs p and P in visual mode: the register replaces the selection.
func (v *Vim) visualPut(c cmd) {
	n := c.count()
	if c.reg == '+' || c.reg == '*' {
		v.putReq = &putReq{before: c.name == "P", count: n, vis: true}
		v.fx = append(v.fx, keymap.ReadClipboard())
		return
	}
	r, ok := v.readReg(c.reg)
	if !ok { // as in vim, the selection goes away and nothing comes in
		v.fail()
		n = 0
	}
	v.replaceSelection(r, c.name == "P", n)
}

// replaceSelection puts register content r in place of the selection n times (not at all for
// n of 0). The replaced
// text goes to the unnamed register, as a delete does. With keep (P), no register changes.
func (v *Vim) replaceSelection(r regval, keep bool, n int) {
	b := v.b
	linewise := b.SelLinewise
	rep := v.visInfo(linewise)
	a, e := v.selEnds()
	r1, c1, r2, c2, ok := b.SelRange()
	v.exitVisual()
	if !ok && !linewise {
		return
	}
	v.change = true
	v.setVisRep(rep)
	var old regval
	if linewise {
		old = regval{text: b.YankLines(a.row, e.row), linewise: true}
		whole := a.row == 0 && e.row == len(b.Lines)-1
		b.DeleteLines(a.row, e.row)
		r.linewise = true // the text goes on lines of its own
		switch {
		case n == 0:
		case whole:
			v.put(r, true, n)
			row, col := b.Row, b.Col
			b.DeleteLines(len(b.Lines)-1, len(b.Lines)-1) // the empty line that stayed
			b.Row, b.Col = row, col
		case a.row > len(b.Lines)-1: // the selection reached the last line
			b.Row = len(b.Lines) - 1
			v.put(r, false, n)
		default:
			b.Row = a.row
			v.put(r, true, n)
		}
	} else {
		old = regval{text: b.DeleteRange(r1, c1, r2, c2)}
		switch l := b.Lines[r1]; {
		case n == 0:
		case r.linewise: // the line breaks at the selection, and the lines go in between
			b.SplitLine()
			b.Row, b.Col = r1, c1
			v.put(r, false, n)
		case c1 >= len(l) && len(l) > 0: // the selection ended the line: put after its last char
			b.Col = len(l) - 1
			v.put(r, false, n)
		default:
			v.put(r, true, n)
		}
	}
	if !keep {
		v.writeReg(0, old, true, !old.linewise && r1 == r2)
	}
}

// shiftMarks moves the marks when the buffer adds or removes lines. A mark on a line that
// goes away is deleted. If lines are joined to the line before, their marks move to it.
func (v *Vim) shiftMarks(s buffer.Shift) {
	for k, p := range v.marks {
		switch {
		case s.N > 0:
			if p.row >= s.At {
				p.row += s.N
			}
		case p.row >= s.At-s.N:
			p.row += s.N
		case p.row >= s.At:
			if !s.Join {
				delete(v.marks, k)
				continue
			}
			p.row, p.col = s.At-1, max(0, p.col+s.Col)
		}
		v.marks[k] = p
	}
}

// snapMarks returns a copy of the named marks for an undo step. In vim, undo and redo bring
// back the named marks (a to z) that the step has, also a mark that the change deleted.
func (v *Vim) snapMarks() any {
	m := map[rune]pos{}
	for k, p := range v.marks {
		if k >= 'a' && k <= 'z' {
			m[k] = p
		}
	}
	return m
}

// restoreMarks puts back the named marks of an undo step. A mark that the step does not have
// stays as it is.
func (v *Vim) restoreMarks(a any) {
	if m, ok := a.(map[rune]pos); ok {
		for k, p := range m {
			v.marks[k] = p
		}
	}
}
