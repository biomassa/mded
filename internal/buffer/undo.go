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
//
// Memory: a step keeps only the lines that it changed (a patch), not a copy of all lines.
// The step that is open (the last PushUndo, or a BeginChange group) keeps one copy of the
// line headers until it closes. Lines are immutable, so a line that did not change is the
// same slice before and after the step, and a compare of the pointers finds the changed
// range.

// state is the start of an open undo step: a copy of the line headers, the cursor, and
// the version of the text.
type state struct {
	lines    [][]rune
	row, col int
	aux      any // the value of OnSnap at the time of the snapshot
	ver      int
}

// patch is a closed undo or redo step. To apply it, the lines at to at+n-1 go out, and old
// comes in. Then the cursor, the OnSnap value, and the version are those of the patch.
type patch struct {
	at, n    int
	old      [][]rune
	row, col int
	aux      any
	ver      int
}

// snap copies the line headers. The line slices are shared, because lines are immutable.
func (b *Buffer) snap() state {
	st := state{lines: append([][]rune(nil), b.Lines...), row: b.Row, col: b.Col, ver: b.ver}
	if b.OnSnap != nil {
		st.aux = b.OnSnap()
	}
	return st
}

// sameLine reports whether a and b are the same line slice. An empty line is the same as
// any empty line.
func sameLine(a, b []rune) bool { return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0]) }

// diff makes the patch that changes the lines now back to the lines of s.
func diff(s state, now [][]rune) patch {
	old := s.lines
	p := 0
	for p < len(old) && p < len(now) && sameLine(old[p], now[p]) {
		p++
	}
	q := 0
	for q < len(old)-p && q < len(now)-p && sameLine(old[len(old)-1-q], now[len(now)-1-q]) {
		q++
	}
	return patch{at: p, n: len(now) - p - q, old: append([][]rune(nil), old[p:len(old)-q]...),
		row: s.row, col: s.col, aux: s.aux, ver: s.ver}
}

// closeOpen turns the open step into a patch on the undo stack.
func (b *Buffer) closeOpen() {
	if b.open == nil {
		return
	}
	b.undo = append(b.undo, diff(*b.open, b.Lines))
	b.open = nil
	if len(b.undo) > undoLimit {
		b.undo = b.undo[1:]
	}
}

// openStep starts an undo step at the state s and clears the redo stack.
func (b *Buffer) openStep(s state) {
	b.closeOpen()
	if n := len(b.undo); n >= undoLimit { // the open step counts as one of the steps
		b.undo = b.undo[n-undoLimit+1:]
	}
	b.open = &s
	b.redo = nil
}

// PushUndo saves an undo step before a change. Typed letters (typing is true) join the
// step of the letter before them. It does nothing inside BeginChange.
func (b *Buffer) PushUndo(typing bool) {
	if b.depth > 0 || (typing && b.typing) {
		return
	}
	b.openStep(b.snap())
	b.typing = typing
}

// BreakTyping ends the typing group, so that the next typed letter starts a new step.
func (b *Buffer) BreakTyping() { b.typing = false }

// BeginChange starts a group of changes that make one undo step.
func (b *Buffer) BeginChange() {
	if b.depth == 0 {
		b.closeOpen() // a conventional step before the group ends here
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
		b.openStep(b.begin)
		b.closeOpen()
	}
	b.begin = state{}
	b.typing = false
}

// InChange reports whether a BeginChange group is open.
func (b *Buffer) InChange() bool { return b.depth > 0 }

// shiftDiff tells the OnShift function how a change of the lines at+old to the lines
// at+now moves the lines. The lines that stay at the start and at the end are the same. The
// lines between them are replaced. If the replaced part has a new size, the old lines go
// away and the new lines come in, as in vim: the marks on the old lines are deleted, and
// the marks after them move.
func (b *Buffer) shiftDiff(at int, old, now [][]rune) {
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
		b.shift(Shift{At: at + p, N: -gone})
	}
	if came := len(now) - p - s; came > 0 {
		b.shift(Shift{At: at + p, N: came})
	}
}

// apply puts the patch p in the buffer and returns the patch that goes back.
func (b *Buffer) apply(p patch) patch {
	at, n := min(p.at, len(b.Lines)), p.n
	n = min(n, len(b.Lines)-at)
	back := patch{at: at, n: len(p.old), old: append([][]rune(nil), b.Lines[at:at+n]...),
		row: b.Row, col: b.Col, ver: b.ver}
	if b.OnSnap != nil {
		back.aux = b.OnSnap()
	}
	lines := make([][]rune, 0, len(b.Lines)-n+len(p.old))
	lines = append(append(append(lines, b.Lines[:at]...), p.old...), b.Lines[at+n:]...)
	if len(lines) == 0 {
		lines = [][]rune{{}}
	}
	b.Lines, b.Row, b.Col = lines, p.row, p.col
	b.shiftDiff(at, back.old, p.old)
	if b.OnRestore != nil && p.aux != nil {
		b.OnRestore(p.aux)
	}
	b.Want, b.SelOn, b.typing = -1, false, false
	b.touch()
	b.ver = p.ver // the text is the text of that version again
	b.Clamp()
	b.Refresh()
	return back
}

// pop takes the last patch of a stack. ok is false for an empty stack and inside a group.
func (b *Buffer) pop(stack *[]patch) (patch, bool) {
	n := len(*stack)
	if n == 0 || b.depth > 0 {
		return patch{}, false
	}
	p := (*stack)[n-1]
	*stack = (*stack)[:n-1]
	return p, true
}

// Undo goes back one step and restores the cursor. It reports whether it did.
func (b *Buffer) Undo() bool {
	if b.depth > 0 {
		return false
	}
	b.closeOpen()
	p, ok := b.pop(&b.undo)
	if !ok {
		return false
	}
	b.redo = append(b.redo, b.apply(p))
	return true
}

// DropUndo goes back one step like Undo, but it forgets the step: Redo cannot bring it back.
// It reports whether it did.
func (b *Buffer) DropUndo() bool {
	if b.depth > 0 {
		return false
	}
	b.closeOpen()
	p, ok := b.pop(&b.undo)
	if !ok {
		return false
	}
	b.apply(p)
	return true
}

// Redo goes forward one step. It reports whether it did.
func (b *Buffer) Redo() bool {
	if b.depth > 0 {
		return false
	}
	b.closeOpen()
	p, ok := b.pop(&b.redo)
	if !ok {
		return false
	}
	b.undo = append(b.undo, b.apply(p))
	return true
}

// UndoDepth returns the number of undo steps.
func (b *Buffer) UndoDepth() int {
	if b.open != nil {
		return len(b.undo) + 1
	}
	return len(b.undo)
}

// RedoDepth returns the number of redo steps.
func (b *Buffer) RedoDepth() int { return len(b.redo) }
