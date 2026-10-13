package buffer

// The selection goes from an anchor to the cursor. The conventional keymap makes it with
// shift and a move key, ctrl+a, or a mouse drag. Vim visual mode sets SelLinewise (V) or
// SelInclusive (v) too.

// SelRange returns the selection in text order as a range with an end that is not in it.
// ok is false if no text is selected. A linewise selection covers whole lines. An inclusive
// selection adds the rune under the later end, or the line break if the end is at a line end.
func (b *Buffer) SelRange() (r1, c1, r2, c2 int, ok bool) {
	if !b.SelOn {
		return 0, 0, 0, 0, false
	}
	r1, c1 = b.clampPos(b.SelRow, b.SelCol)
	r2, c2 = b.clampPos(b.Row, b.Col)
	if before(r2, c2, r1, c1) {
		r1, c1, r2, c2 = r2, c2, r1, c1
	}
	switch {
	case b.SelLinewise:
		return r1, 0, r2, len(b.Lines[r2]), true
	case b.SelInclusive:
		switch {
		case c2 < len(b.Lines[r2]):
			c2++
		case r2 < len(b.Lines)-1:
			r2, c2 = r2+1, 0
		}
		return r1, c1, r2, c2, r1 != r2 || c1 != c2
	}
	return r1, c1, r2, c2, r1 != r2 || c1 != c2
}

// StartSel sets the anchor at the cursor if no selection is on.
func (b *Buffer) StartSel() {
	if !b.SelOn {
		b.SelRow, b.SelCol, b.SelOn = b.Row, b.Col, true
	}
}

// ClearSel removes the selection and its modes.
func (b *Buffer) ClearSel() { b.SelOn, b.SelLinewise, b.SelInclusive = false, false, false }

// SelectAll selects the whole text and puts the cursor at its end.
func (b *Buffer) SelectAll() {
	b.SelRow, b.SelCol, b.SelOn = 0, 0, true
	b.SelLinewise, b.SelInclusive = false, false
	b.Row = len(b.Lines) - 1
	b.Col, b.Want = len(b.Lines[b.Row]), -1
}

// InSel reports whether rune col of line row is selected.
func (b *Buffer) InSel(row, col int) bool {
	r1, c1, r2, c2, ok := b.SelRange()
	if !ok || row < r1 || row > r2 {
		return false
	}
	if b.SelLinewise {
		return true
	}
	return (row > r1 || col >= c1) && (row < r2 || col < c2)
}

// SelLine reports whether line row has selected text. Such lines show as source, so that
// the selection covers the true characters.
func (b *Buffer) SelLine(row int) bool {
	r1, _, r2, _, ok := b.SelRange()
	return ok && row >= r1 && row <= r2
}

// SelectedText returns the selected text, or "". A linewise selection has no final line break.
func (b *Buffer) SelectedText() string {
	r1, c1, r2, c2, ok := b.SelRange()
	if !ok {
		return ""
	}
	if b.SelLinewise {
		return b.YankLines(r1, r2)
	}
	return b.TextRange(r1, c1, r2, c2)
}

// DeleteSelection removes the selected text, clears the selection, and puts the cursor where
// the text started. It reports whether it removed text. It does not save an undo step.
func (b *Buffer) DeleteSelection() bool {
	r1, c1, r2, c2, ok := b.SelRange()
	linewise := b.SelLinewise
	b.ClearSel()
	if !ok {
		return false
	}
	if linewise {
		b.DeleteLines(r1, r2)
		return true
	}
	b.DeleteRange(r1, c1, r2, c2)
	return true
}

// ReplaceSelection removes the selection before an edit that puts text in its place. It
// saves the undo step, so the caller does not save one again. It reports whether text went.
func (b *Buffer) ReplaceSelection() bool {
	if _, _, _, _, ok := b.SelRange(); !ok {
		b.ClearSel()
		return false
	}
	b.PushUndo(false)
	return b.DeleteSelection()
}
