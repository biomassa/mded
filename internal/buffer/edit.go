package buffer

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	// listPrefix matches the start of a list item: indent, marker, space, optional checkbox.
	listPrefix = regexp.MustCompile(`^(\s*)([-*+]|\d+[.)])(\s+)(\[[ xX]\]\s)?`)
	// checkboxLine matches a list item with a checkbox. Group 2 is the mark.
	checkboxLine = regexp.MustCompile(`^(\s*(?:[-*+]|\d+[.)])\s+\[)([ xX])(\]\s)`)
)

// IsWordRune reports whether r is part of a word: a letter, a digit, or an underscore.
func IsWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

// Insert puts runes at the cursor. The runes must not contain line breaks.
func (b *Buffer) Insert(rs []rune) {
	b.Clamp()
	b.Lines[b.Row] = splice(b.Lines[b.Row], b.Col, b.Col, rs)
	b.Col += len(rs)
	b.Want = -1
	b.touch()
}

// InsertText puts text at the cursor. The text can have line breaks. The cursor goes to the
// end of the text.
func (b *Buffer) InsertText(s string) {
	parts := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i, p := range parts {
		if i > 0 {
			b.SplitLine()
		}
		b.Insert([]rune(p))
	}
}

// SplitLine breaks the line at the cursor. The cursor goes to the start of the new line.
func (b *Buffer) SplitLine() {
	b.Clamp()
	l := b.Lines[b.Row]
	head, tail := splice(l, b.Col, len(l), nil), splice(l, 0, b.Col, nil)
	b.Lines[b.Row] = head
	b.Lines = append(b.Lines[:b.Row+1], append([][]rune{tail}, b.Lines[b.Row+1:]...)...)
	b.Row++
	b.Col, b.Want = 0, -1
	b.touch()
}

// Newline splits the line. On a list item, the new line starts the next item. On an empty
// item, it ends the list instead.
func (b *Buffer) Newline() {
	b.Clamp()
	l := string(b.Lines[b.Row])
	p := listPrefix.FindStringSubmatch(l)
	if p == nil || b.Col < len([]rune(p[0])) { // no item, or the cursor is inside the marker
		b.SplitLine()
		return
	}
	if strings.TrimSpace(l[len(p[0]):]) == "" { // an empty item ends the list
		b.Lines[b.Row] = []rune{}
		b.Col, b.Want = 0, -1
		b.touch()
		return
	}
	marker := p[2]
	if n, err := strconv.Atoi(strings.TrimRight(marker, ".)")); err == nil {
		marker = strconv.Itoa(n+1) + marker[len(marker)-1:]
	}
	next := p[1] + marker + p[3]
	if p[4] != "" {
		next += "[ ] "
	}
	b.SplitLine()
	b.Insert([]rune(next))
}

// OpenPrefix returns the start of a new line that vim o (above false) or O (above true) opens
// next to line row. On a list item, it is the marker of the item, with an empty checkbox if
// the item has one. A numbered item gives the next number below and the same number above.
// On an empty item, it is the indent only. On any other line, it is empty.
func (b *Buffer) OpenPrefix(row int, above bool) string {
	if row < 0 || row >= len(b.Lines) {
		return ""
	}
	l := string(b.Lines[row])
	p := listPrefix.FindStringSubmatch(l)
	if p == nil {
		return ""
	}
	if strings.TrimSpace(l[len(p[0]):]) == "" {
		return p[1]
	}
	marker := p[2]
	if n, err := strconv.Atoi(strings.TrimRight(marker, ".)")); err == nil && !above {
		marker = strconv.Itoa(n+1) + marker[len(marker)-1:]
	}
	next := p[1] + marker + p[3]
	if p[4] != "" {
		next += "[ ] "
	}
	return next
}

// Backspace deletes the rune before the cursor. At the start of a line, it joins the line
// with the line above.
func (b *Buffer) Backspace() {
	b.Clamp()
	b.Want = -1
	if b.Col > 0 {
		b.Lines[b.Row] = splice(b.Lines[b.Row], b.Col-1, b.Col, nil)
		b.Col--
		b.touch()
		return
	}
	if b.Row > 0 {
		prev := b.Lines[b.Row-1]
		b.Col = len(prev)
		b.Lines[b.Row-1] = splice(prev, len(prev), len(prev), b.Lines[b.Row])
		b.Lines = append(b.Lines[:b.Row], b.Lines[b.Row+1:]...)
		b.Row--
		b.touch()
	}
}

// DeleteForward deletes the rune under the cursor. At the end of a line, it joins the next
// line to this one.
func (b *Buffer) DeleteForward() {
	b.Clamp()
	b.Want = -1
	l := b.Lines[b.Row]
	if b.Col < len(l) {
		b.Lines[b.Row] = splice(l, b.Col, b.Col+1, nil)
		b.touch()
		return
	}
	if b.Row < len(b.Lines)-1 {
		b.Lines[b.Row] = splice(l, len(l), len(l), b.Lines[b.Row+1])
		b.Lines = append(b.Lines[:b.Row+1], b.Lines[b.Row+2:]...)
		b.touch()
	}
}

// DeleteWordBack deletes the word before the cursor. At the start of a line, it joins the
// line with the line above.
func (b *Buffer) DeleteWordBack() {
	b.Clamp()
	if b.Col == 0 {
		b.Backspace()
		return
	}
	end := b.Col
	b.WordLeft()
	b.Lines[b.Row] = splice(b.Lines[b.Row], b.Col, end, nil)
	b.touch()
}

// MoveLeft moves one rune left. It goes to the end of the line above at a line start.
func (b *Buffer) MoveLeft() {
	b.Clamp()
	b.Want = -1
	switch {
	case b.Col > 0:
		b.Col--
	case b.Row > 0:
		b.Row--
		b.Col = len(b.Lines[b.Row])
	}
}

// MoveRight moves one rune right. It goes to the next line at a line end.
func (b *Buffer) MoveRight() {
	b.Clamp()
	b.Want = -1
	switch {
	case b.Col < len(b.Lines[b.Row]):
		b.Col++
	case b.Row < len(b.Lines)-1:
		b.Row++
		b.Col = 0
	}
}

// Home moves to the start of the line.
func (b *Buffer) Home() { b.Col, b.Want = 0, -1 }

// End moves to the end of the line.
func (b *Buffer) End() {
	b.Clamp()
	b.Col, b.Want = len(b.Lines[b.Row]), -1
}

// WordLeft moves to the start of the word before the cursor.
func (b *Buffer) WordLeft() {
	b.Clamp()
	l := b.Lines[b.Row]
	if b.Col == 0 {
		b.MoveLeft()
		return
	}
	b.Want = -1
	i := b.Col
	for i > 0 && !IsWordRune(l[i-1]) {
		i--
	}
	for i > 0 && IsWordRune(l[i-1]) {
		i--
	}
	b.Col = i
}

// WordRight moves to the end of the word after the cursor.
func (b *Buffer) WordRight() {
	b.Clamp()
	l := b.Lines[b.Row]
	if b.Col == len(l) {
		b.MoveRight()
		return
	}
	b.Want = -1
	i := b.Col
	for i < len(l) && !IsWordRune(l[i]) {
		i++
	}
	for i < len(l) && IsWordRune(l[i]) {
		i++
	}
	b.Col = i
}

// WrapWord puts before and after around the word at the cursor. Without a word, it inserts
// both and puts the cursor between them.
func (b *Buffer) WrapWord(before, after string) {
	b.Clamp()
	l := b.Lines[b.Row]
	s, t := b.Col, b.Col
	for s > 0 && IsWordRune(l[s-1]) {
		s--
	}
	for t < len(l) && IsWordRune(l[t]) {
		t++
	}
	b.Want = -1
	if s == t {
		b.Insert([]rune(before + after))
		b.Col -= len([]rune(after))
		if after == "]()" { // a link without text: the cursor goes into the brackets
			b.Col = s + 1
		}
		return
	}
	word := l[s:t]
	nl := append(append([]rune{}, l[:s]...), []rune(before)...)
	nl = append(nl, word...)
	nl = append(nl, []rune(after)...)
	nl = append(nl, l[t:]...)
	b.Lines[b.Row] = nl
	b.Col = s + len([]rune(before)) + len(word) + len([]rune(after))
	if after == "]()" { // a link: the cursor goes into the parentheses for the URL
		b.Col--
	}
	b.touch()
}

// toggleCheckboxSrc switches the checkbox of one line between [ ] and [x].
func toggleCheckboxSrc(l string) (string, bool) {
	sub := checkboxLine.FindStringSubmatch(l)
	if sub == nil {
		return l, false
	}
	mark := "x"
	if sub[2] != " " {
		mark = " "
	}
	return sub[1] + mark + sub[3] + l[len(sub[0]):], true
}

// ToggleCheckboxAt switches the checkbox of line row. It reports whether the line has one.
func (b *Buffer) ToggleCheckboxAt(row int) bool {
	if row < 0 || row >= len(b.Lines) {
		return false
	}
	src, ok := toggleCheckboxSrc(string(b.Lines[row]))
	if ok {
		b.SetLine(row, []rune(src))
	}
	return ok
}

// ToggleLineCheckbox switches the checkbox of the cursor line, or makes the line a checkbox
// item.
func (b *Buffer) ToggleLineCheckbox() {
	b.Clamp()
	l := string(b.Lines[b.Row])
	switch p := listPrefix.FindStringSubmatch(l); {
	case p != nil && p[4] != "":
		b.ToggleCheckboxAt(b.Row)
	case p != nil:
		b.SetLine(b.Row, []rune(p[0]+"[ ] "+l[len(p[0]):]))
		b.Col += 4
	default:
		b.SetLine(b.Row, []rune("- [ ] "+l))
		b.Col += 6
	}
}

// Indent puts n spaces before each non-empty line from r1 to r2. It does not move the cursor.
func (b *Buffer) Indent(r1, r2, n int) {
	r1, r2 = b.rowSpan(r1, r2)
	pad := []rune(strings.Repeat(" ", max(0, n)))
	for r := r1; r <= r2; r++ {
		if len(b.Lines[r]) > 0 {
			b.Lines[r] = splice(b.Lines[r], 0, 0, pad)
			b.touch()
		}
	}
}

// Outdent removes up to n leading spaces from each line from r1 to r2. A leading tab counts
// as a full indent and goes whole. It does not move the cursor.
func (b *Buffer) Outdent(r1, r2, n int) {
	r1, r2 = b.rowSpan(r1, r2)
	for r := r1; r <= r2; r++ {
		l := b.Lines[r]
		i := 0
		for i < len(l) && i < n && l[i] == ' ' {
			i++
		}
		if i == 0 && len(l) > 0 && l[0] == '\t' {
			i = 1
		}
		if i > 0 {
			b.Lines[r] = splice(l, 0, i, nil)
			b.touch()
		}
	}
}

// rowSpan orders and limits a line span.
func (b *Buffer) rowSpan(r1, r2 int) (int, int) {
	if r1 > r2 {
		r1, r2 = r2, r1
	}
	return max(0, r1), min(r2, len(b.Lines)-1)
}

// ---- layout ----

// WrapLine splits a line into screen segments of at most w runes, at spaces if possible.
// It returns the start of each segment.
func WrapLine(l []rune, w int) []int {
	w = max(1, w)
	starts := []int{0}
	for s := 0; len(l)-s > w; {
		cut := s + w
		for i := cut; i > s+w/2; i-- {
			if l[i-1] == ' ' {
				cut = i
				break
			}
		}
		starts = append(starts, cut)
		s = cut
	}
	return starts
}

// ScreenPos returns the screen line and column of the cursor for text width w.
func (b *Buffer) ScreenPos(w int) (y, x int) {
	b.Clamp()
	for i := 0; i < b.Row; i++ {
		y += len(WrapLine(b.Lines[i], w))
	}
	starts := WrapLine(b.Lines[b.Row], w)
	seg := 0
	for k, s := range starts {
		if b.Col >= s {
			seg = k
		}
	}
	return y + seg, b.Col - starts[seg]
}

// MoveVertical moves the cursor d screen lines and keeps the wanted column if it can.
// Lines wrap at width w.
func (b *Buffer) MoveVertical(d, w int) {
	y, x := b.ScreenPos(w)
	if b.Want < 0 {
		b.Want = x
	}
	target := y + d
	if target < 0 {
		return
	}
	n := 0
	for i, l := range b.Lines {
		starts := WrapLine(l, w)
		if target < n+len(starts) {
			k := target - n
			end := len(l)
			if k+1 < len(starts) {
				end = starts[k+1] - 1
			}
			b.Row, b.Col = i, min(starts[k]+b.Want, end)
			return
		}
		n += len(starts)
	}
}
