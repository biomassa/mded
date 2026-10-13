package buffer

import "slices"

// Two undo policies share one set of stacks.
//
// Conventional: the edit code calls PushUndo before a change. Consecutive typed letters
// share one step (typing is true).
//
// Vim: the engine calls BeginChange before a command or an insert session and EndChange
// after it. One pair is one undo step, and the step exists only if the text changed. Calls
// nest, so a command that runs inside an insert session joins the session step. Inside a
// BeginChange pair, PushUndo does nothing. This lets shared edit code work with both policies.

// snap copies the lines. The line slices are shared, because lines are immutable.
func (b *Buffer) snap() state {
	st := state{lines: append([][]rune(nil), b.Lines...), row: b.Row, col: b.Col}
	if b.OnSnap != nil {
		st.aux = b.OnSnap()
	}
	return st
}

func (b *Buffer) pushState(s state) {
	b.undo = append(b.undo, s)
	if len(b.undo) > undoLimit {
		b.undo = b.undo[1:]
	}
	b.redo = nil
}

// PushUndo saves an undo step before a change. Typed letters (typing is true) join the
// step of the letter before them. It does nothing inside BeginChange.
func (b *Buffer) PushUndo(typing bool) {
	if b.depth > 0 || (typing && b.typing) {
		return
	}
	b.pushState(b.snap())
	b.typing = typing
}

// BreakTyping ends the typing group, so that the next typed letter starts a new step.
func (b *Buffer) BreakTyping() { b.typing = false }

// BeginChange starts a group of changes that make one undo step.
func (b *Buffer) BeginChange() {
	if b.depth == 0 {
		b.begin, b.beginRev = b.snap(), b.Rev
	}
	b.depth++
}

// EndChange ends a group. The outermost EndChange saves the step if the text changed.
func (b *Buffer) EndChange() {
	if b.depth == 0 {
		return
	}
	b.depth--
	if b.depth > 0 {
		return
	}
	if b.Rev != b.beginRev {
		b.pushState(b.begin)
	}
	b.begin = state{}
	b.typing = false
}

// InChange reports whether a BeginChange group is open.
func (b *Buffer) InChange() bool { return b.depth > 0 }

// shiftDiff tells the OnShift function how a change from the lines old to the lines now
// moves the lines. The lines that stay at the start and at the end are the same. The lines
// between them are replaced. If the replaced part has a new size, the old lines go away and
// the new lines come in, as in vim: the marks on the old lines are deleted, and the marks
// after them move.
func (b *Buffer) shiftDiff(old, now [][]rune) {
	if b.OnShift == nil || len(old) == len(now) {
		return
	}
	p := 0
	for p < len(old) && p < len(now) && slices.Equal(old[p], now[p]) {
		p++
	}
	s := 0
	for s < len(old)-p && s < len(now)-p && slices.Equal(old[len(old)-1-s], now[len(now)-1-s]) {
		s++
	}
	if gone := len(old) - p - s; gone > 0 {
		b.shift(Shift{At: p, N: -gone})
	}
	if came := len(now) - p - s; came > 0 {
		b.shift(Shift{At: p, N: came})
	}
}

// restore puts a saved state in the buffer. It reports the line shifts of the change.
func (b *Buffer) restore(s state) {
	old := b.Lines
	b.Lines, b.Row, b.Col = s.lines, s.row, s.col
	b.shiftDiff(old, s.lines)
	if b.OnRestore != nil && s.aux != nil {
		b.OnRestore(s.aux)
	}
	b.Want, b.SelOn, b.typing = -1, false, false
	b.touch()
	b.Clamp()
	b.Refresh()
}

// Undo goes back one step and restores the cursor. It reports whether it did.
func (b *Buffer) Undo() bool {
	n := len(b.undo)
	if n == 0 || b.depth > 0 {
		return false
	}
	b.redo = append(b.redo, b.snap())
	s := b.undo[n-1]
	b.undo = b.undo[:n-1]
	b.restore(s)
	return true
}

// DropUndo goes back one step like Undo, but it forgets the step: Redo cannot bring it back.
// It reports whether it did.
func (b *Buffer) DropUndo() bool {
	n := len(b.undo)
	if n == 0 || b.depth > 0 {
		return false
	}
	s := b.undo[n-1]
	b.undo = b.undo[:n-1]
	b.restore(s)
	return true
}

// Redo goes forward one step. It reports whether it did.
func (b *Buffer) Redo() bool {
	n := len(b.redo)
	if n == 0 || b.depth > 0 {
		return false
	}
	b.undo = append(b.undo, b.snap())
	s := b.redo[n-1]
	b.redo = b.redo[:n-1]
	b.restore(s)
	return true
}

// UndoDepth returns the number of undo steps.
func (b *Buffer) UndoDepth() int { return len(b.undo) }

// RedoDepth returns the number of redo steps.
func (b *Buffer) RedoDepth() int { return len(b.redo) }
