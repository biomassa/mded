package buffer

import (
	"strings"
	"unicode"
)

// Range operations for the vim engine. A range goes from (r1, c1) to (r2, c2) in runes. The
// end is not in the range. A range with a later start is put in order first. Positions are
// limited to the text. These operations do not save undo steps.

// order puts a range in text order and limits it to the text.
func (b *Buffer) order(r1, c1, r2, c2 int) (int, int, int, int) {
	r1, c1 = b.clampPos(r1, c1)
	r2, c2 = b.clampPos(r2, c2)
	if before(r2, c2, r1, c1) {
		return r2, c2, r1, c1
	}
	return r1, c1, r2, c2
}

// TextRange returns the text of a range.
func (b *Buffer) TextRange(r1, c1, r2, c2 int) string {
	r1, c1, r2, c2 = b.order(r1, c1, r2, c2)
	if r1 == r2 {
		return string(b.Lines[r1][c1:c2])
	}
	parts := []string{string(b.Lines[r1][c1:])}
	for r := r1 + 1; r < r2; r++ {
		parts = append(parts, string(b.Lines[r]))
	}
	parts = append(parts, string(b.Lines[r2][:c2]))
	return strings.Join(parts, "\n")
}

// DeleteRange removes a range and returns its text. The cursor goes to the start of the range.
func (b *Buffer) DeleteRange(r1, c1, r2, c2 int) string {
	r1, c1, r2, c2 = b.order(r1, c1, r2, c2)
	text := b.TextRange(r1, c1, r2, c2)
	if r1 != r2 || c1 != c2 {
		b.Lines[r1] = splice(b.Lines[r1], c1, len(b.Lines[r1]), b.Lines[r2][c2:])
		b.Lines = append(b.Lines[:r1+1], b.Lines[r2+1:]...)
		b.touch()
		if r2 > r1 {
			b.shift(Shift{At: r1 + 1, N: -(r2 - r1 - 1)}) // the lines between go away
			b.shift(Shift{At: r1 + 1, N: -1, Join: true, Col: c1 - c2})
		}
	}
	b.Row, b.Col, b.Want = r1, c1, -1
	return text
}

// ReplaceRange replaces a range with text and returns the old text. The cursor goes to the
// end of the new text.
func (b *Buffer) ReplaceRange(r1, c1, r2, c2 int, text string) string {
	old := b.DeleteRange(r1, c1, r2, c2)
	b.InsertText(text)
	return old
}

// YankLines returns lines r1 to r2 as text, without a final line break.
func (b *Buffer) YankLines(r1, r2 int) string {
	r1, r2 = b.rowSpan(r1, r2)
	parts := make([]string, 0, r2-r1+1)
	for r := r1; r <= r2; r++ {
		parts = append(parts, string(b.Lines[r]))
	}
	return strings.Join(parts, "\n")
}

// DeleteLines removes lines r1 to r2 and returns them as text. The buffer keeps one empty
// line if all lines go. The cursor goes to the start of the line that takes the place.
func (b *Buffer) DeleteLines(r1, r2 int) string {
	r1, r2 = b.rowSpan(r1, r2)
	text := b.YankLines(r1, r2)
	b.Lines = append(b.Lines[:r1], b.Lines[r2+1:]...)
	b.shift(Shift{At: r1, N: -(r2 - r1 + 1)})
	if len(b.Lines) == 0 {
		b.Lines = [][]rune{{}}
	}
	b.Row, b.Col, b.Want = min(r1, len(b.Lines)-1), 0, -1
	b.touch()
	return text
}

// InsertLines puts text as whole lines before line at. at equal to the line count appends.
// The cursor goes to the start of the first new line.
func (b *Buffer) InsertLines(at int, text string) {
	at = max(0, min(at, len(b.Lines)))
	ls := splitRunes(strings.ReplaceAll(text, "\r\n", "\n"))
	b.Lines = append(b.Lines[:at], append(ls, b.Lines[at:]...)...)
	b.shift(Shift{At: at, N: len(ls)})
	b.Row, b.Col, b.Want = at, 0, -1
	b.touch()
}

// JoinLines joins n lines that start at the cursor line, at least two. With withSpace, it
// removes the leading blanks of each joined line and puts one space between the parts,
// except after a blank, before ")", or when the joined line is empty. The cursor goes to the
// last join point. It reports whether it joined.
func (b *Buffer) JoinLines(n int, withSpace bool) bool {
	b.Clamp()
	n = max(2, n)
	last := min(b.Row+n-1, len(b.Lines)-1)
	if last <= b.Row {
		return false
	}
	cur := b.Lines[b.Row]
	col := len(cur)
	for r := b.Row + 1; r <= last; r++ {
		next := b.Lines[r]
		if withSpace {
			next = []rune(strings.TrimLeftFunc(string(next), unicode.IsSpace))
			col = len(cur)
			if len(next) > 0 && next[0] != ')' && len(cur) > 0 && !unicode.IsSpace(cur[len(cur)-1]) {
				cur = splice(cur, len(cur), len(cur), []rune{' '})
			} else if len(cur) > 0 && unicode.IsSpace(cur[len(cur)-1]) {
				col = len(cur) - 1
			}
		} else {
			col = len(cur)
		}
		cur = splice(cur, len(cur), len(cur), next)
	}
	b.Lines[b.Row] = cur
	b.Lines = append(b.Lines[:b.Row+1], b.Lines[last+1:]...)
	b.shift(Shift{At: b.Row + 1, N: -(last - b.Row), Join: true})
	b.Col, b.Want = col, -1
	b.touch()
	return true
}

// CaseMode is a kind of case change.
type CaseMode int

// The case changes of ChangeCase.
const (
	CaseUpper CaseMode = iota
	CaseLower
	CaseToggle
)

func (m CaseMode) apply(r rune) rune {
	switch m {
	case CaseUpper:
		return unicode.ToUpper(r)
	case CaseLower:
		return unicode.ToLower(r)
	}
	if unicode.IsUpper(r) {
		return unicode.ToLower(r)
	}
	return unicode.ToUpper(r)
}

// ChangeCase changes the case of the runes in a range. It does not move the cursor.
func (b *Buffer) ChangeCase(r1, c1, r2, c2 int, mode CaseMode) {
	r1, c1, r2, c2 = b.order(r1, c1, r2, c2)
	for r := r1; r <= r2; r++ {
		a, z := 0, len(b.Lines[r])
		if r == r1 {
			a = c1
		}
		if r == r2 {
			z = c2
		}
		changed := false
		nl := append([]rune(nil), b.Lines[r]...)
		for i := a; i < z; i++ {
			if n := mode.apply(nl[i]); n != nl[i] {
				nl[i], changed = n, true
			}
		}
		if changed {
			b.Lines[r] = nl
			b.touch()
		}
	}
}

// ChangeCaseLines changes the case of whole lines r1 to r2.
func (b *Buffer) ChangeCaseLines(r1, r2 int, mode CaseMode) {
	r1, r2 = b.rowSpan(r1, r2)
	b.ChangeCase(r1, 0, r2, len(b.Lines[r2]), mode)
}
