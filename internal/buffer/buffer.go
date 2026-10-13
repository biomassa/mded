// Package buffer is the text buffer of mded. It holds the lines, the cursor, the selection,
// the undo stacks, and the search state. It has no dependency on the terminal libraries.
//
// One rule keeps undo cheap: code never changes a line in place. A line is a []rune that
// the buffer treats as immutable. To change a line, build a new slice and call SetLine.
// An undo step then copies only the slice of line headers.
package buffer

import "strings"

// undoLimit is the number of undo steps that the buffer keeps.
const undoLimit = 200

// state is one undo step: the lines and the cursor.
type state struct {
	lines    [][]rune
	row, col int
}

// Buffer is the text of one document with its cursor, selection, undo, and search state.
type Buffer struct {
	Lines    [][]rune // always at least one line
	Row, Col int      // cursor: line and rune in the line
	Want     int      // column that up/down try to keep, -1 for none
	Off      int      // first visible screen line. The UI owns this field.
	Saved    string   // text at the last save or load
	Rev      int      // change counter. It grows with each change of the text.

	// The selection goes from the anchor SelRow, SelCol to the cursor.
	SelOn          bool
	SelRow, SelCol int
	SelLinewise    bool // vim V mode: whole lines
	SelInclusive   bool // vim v mode: the character under the later end is selected too

	Search Search // shared by view mode, conventional edit, and vim

	undo, redo []state
	typing     bool  // the last change was typing, so the next letter joins its undo step
	depth      int   // nesting of BeginChange
	begin      state // snapshot at the outermost BeginChange
	beginRev   int
	dirtyRev   int // Rev for which dirtyVal is valid
	dirtyVal   bool
}

// New returns a buffer that holds text, marked as saved.
func New(text string) *Buffer {
	b := &Buffer{}
	b.SetText(text)
	return b
}

// SetText loads text, marks it as saved, and clears the undo history and the selection.
// The cursor goes to the start. The search pattern stays.
func (b *Buffer) SetText(text string) {
	b.Lines = splitRunes(text)
	b.Row, b.Col, b.Want, b.Off = 0, 0, -1, 0
	b.SelOn, b.SelLinewise, b.SelInclusive = false, false, false
	b.undo, b.redo, b.typing, b.depth = nil, nil, false, 0
	b.Saved = text
	b.touch()
	b.dirtyRev = -1
	b.Refresh()
}

func splitRunes(s string) [][]rune {
	parts := strings.Split(s, "\n")
	out := make([][]rune, len(parts))
	for i, l := range parts {
		out[i] = []rune(l)
	}
	return out
}

// Text returns the text with LF line ends.
func (b *Buffer) Text() string {
	parts := make([]string, len(b.Lines))
	for i, l := range b.Lines {
		parts[i] = string(l)
	}
	return strings.Join(parts, "\n")
}

// Dirty reports whether the text differs from the saved text.
func (b *Buffer) Dirty() bool {
	if b.dirtyRev != b.Rev {
		b.dirtyVal = b.Text() != b.Saved
		b.dirtyRev = b.Rev
	}
	return b.dirtyVal
}

// MarkSaved records the current text as the saved text.
func (b *Buffer) MarkSaved() {
	b.Saved = b.Text()
	b.dirtyRev = -1
}

// touch counts a change of the text.
func (b *Buffer) touch() { b.Rev++ }

// LineCount returns the number of lines.
func (b *Buffer) LineCount() int { return len(b.Lines) }

// SetLine replaces line row with l. The buffer takes l: the caller must not change it later.
func (b *Buffer) SetLine(row int, l []rune) {
	if row < 0 || row >= len(b.Lines) {
		return
	}
	b.Lines[row] = l
	b.touch()
}

// Clamp moves the cursor into the text.
func (b *Buffer) Clamp() {
	b.Row = max(0, min(b.Row, len(b.Lines)-1))
	b.Col = max(0, min(b.Col, len(b.Lines[b.Row])))
}

// MoveTo puts the cursor at row, col, limited to the text. It clears the wanted column.
func (b *Buffer) MoveTo(row, col int) {
	b.Row, b.Col, b.Want = row, col, -1
	b.Clamp()
}

// splice returns a new slice: l with l[a:z] replaced by ins.
func splice(l []rune, a, z int, ins []rune) []rune {
	out := make([]rune, 0, len(l)-(z-a)+len(ins))
	out = append(out, l[:a]...)
	out = append(out, ins...)
	return append(out, l[z:]...)
}

// clampPos limits a position to the text.
func (b *Buffer) clampPos(row, col int) (int, int) {
	row = max(0, min(row, len(b.Lines)-1))
	return row, max(0, min(col, len(b.Lines[row])))
}

// before reports whether position (r1, c1) comes before (r2, c2).
func before(r1, c1, r2, c2 int) bool { return r1 < r2 || (r1 == r2 && c1 < c2) }
