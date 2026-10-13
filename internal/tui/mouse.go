package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/biomassa/mded/internal/buffer"
	"github.com/biomassa/mded/internal/config"
	"github.com/biomassa/mded/internal/markdown"
)

// doubleClickTime is the longest time between the two clicks of a double-click.
const doubleClickTime = 400 * time.Millisecond

// clickInfo is the last click, for double-click detection.
type clickInfo struct {
	x, y int
	t    time.Time
}

// isDoubleClick records a click and reports whether it completes a double-click.
func (m *Model) isDoubleClick(x, y int) bool {
	now := time.Now()
	c := m.lastClick
	dbl := !c.t.IsZero() && now.Sub(c.t) < doubleClickTime && c.y == y && c.x-x <= 1 && x-c.x <= 1
	if dbl {
		m.lastClick = clickInfo{}
	} else {
		m.lastClick = clickInfo{x: x, y: y, t: now}
	}
	return dbl
}

// onBorder reports whether the cell is on the border between the two panes.
func (m *Model) onBorder(x, y int) bool {
	w := m.treeW()
	return w > 0 && y < m.paneH() && (x == w-1 || x == w)
}

func (m *Model) mouseClick(ms tea.Mouse) {
	x, y := ms.X, ms.Y
	dbl := m.isDoubleClick(x, y)
	switch {
	case m.quitting || m.dlg != nil:
		return
	case m.themes != nil:
		m.themeClick(x, y, dbl)
		return
	case m.help != nil:
		if !m.helpRect().has(x, y) {
			m.help = nil
		}
		return
	}
	if ms.Button != tea.MouseLeft {
		return
	}
	if m.onBorder(x, y) && !m.promptOpenExceptFind() {
		m.dragBorder = true
		return
	}
	switch {
	case m.treeRect().has(x, y):
		if m.promptOpenExceptFind() && m.focus != focusTree {
			return
		}
		m.focus = focusTree
		tr := m.treeRect()
		if r := y - tr.y - 1; r >= 0 && y < tr.y+tr.h-1 {
			m.treeAction(m.tree.Click(r, dbl))
		}
	case m.docRect().has(x, y):
		if m.promptOpenExceptFind() {
			return
		}
		m.focus = focusDoc
		m.docClick(x, y, dbl)
	}
}

// promptOpenExceptFind reports whether a prompt other than the find box is open. A click
// does not leave such a prompt.
func (m *Model) promptOpenExceptFind() bool {
	if m.find != nil {
		return false
	}
	return m.promptOpen()
}

// rowAt returns the index of the text row under the cell, or -1 outside the text.
func (m *Model) rowAt(y int) int {
	dr := m.docRect()
	r := y - dr.y - 1 - m.textTop()
	if r < 0 || r >= m.textH() {
		return -1
	}
	return m.doc.off + r
}

// posAt returns the source line and rune under a cell of the text. src is the rune of the
// cell itself, or -1 if the cell has no rune of the source (a border, a margin, an image).
// ok is false outside the text rows.
func (m *Model) posAt(x, y int) (line, col, src int, ok bool) {
	i := m.rowAt(y)
	lines := m.doc.buf.Lines
	if i < 0 {
		return 0, 0, -1, false
	}
	if i >= len(m.rows) { // below the text: the end of the document
		last := len(lines) - 1
		return last, len(lines[last]), -1, true
	}
	r := m.rows[i]
	n := len(lines[r.Line])
	if r.Cells == nil { // a rendered block or an image: the start of its line
		return r.Line, 0, -1, true
	}
	k := x - m.docRect().x - 2 // the border and the margin
	col, src = n, -1
	switch {
	case k >= 0 && k < len(r.Cells):
		col, src = r.Cells[k].Src, r.Cells[k].Src
		if col < 0 {
			col = 0
			if k > 0 {
				col = r.Cells[k-1].Src + 1
			}
			src = -1
		}
	case k < 0 && len(r.Cells) > 0:
		col = max(0, r.Cells[0].Src)
	case r.Source && len(r.Cells) > 0 && k >= len(r.Cells):
		col = r.Cells[len(r.Cells)-1].Src + 1
	}
	return r.Line, min(col, n), src, true
}

// clickAct is what a click in view mode did. A double-click reads it, because its first click
// already did the single-click action.
type clickAct struct {
	checkbox bool           // the click toggled a checkbox
	link     bool           // the click followed a link
	buf      *buffer.Buffer // the buffer of the toggle
	rev      int            // the Rev of the buffer after the toggle
}

// mouseKeymap is the part of a keymap that handles the mouse. The vim engine has it.
type mouseKeymap interface {
	StartVisual(b *buffer.Buffer, row, col int)
	LeaveVisual(b *buffer.Buffer)
}

// docClick handles a left click in the document pane.
func (m *Model) docClick(x, y int, dbl bool) {
	line, col, src, ok := m.posAt(x, y)
	if !ok {
		return
	}
	b := m.doc.buf
	act := m.lastAct
	m.lastAct = clickAct{}
	if m.mode == modeEdit {
		if mk, ok := m.km.(mouseKeymap); ok && m.vimFlavor {
			mk.LeaveVisual(b)
		}
		b.MoveTo(line, col)
		b.SelRow, b.SelCol, b.SelOn = line, col, false
		b.BreakTyping()
		m.dragSel = true
		m.follow = true
		return
	}
	if m.doc.image {
		return
	}
	if dbl {
		switch {
		case act.link: // the first click followed the link
		case act.checkbox:
			// Undo the toggle of the first click: the text and the saved state come back.
			if act.buf == b && act.rev == b.Rev {
				b.DropUndo()
			}
			m.enterEdit(&[2]int{line, 0})
		default:
			m.enterEdit(&[2]int{line, 0})
		}
		return
	}
	if src < 0 {
		return
	}
	for i, it := range m.getItems() {
		if it.Line == line && src >= it.Start && src < it.End {
			m.itemSel = i
			if it.Kind == markdown.Checkbox {
				m.toggleItem(it)
				m.lastAct = clickAct{checkbox: true, buf: b, rev: b.Rev}
			} else {
				m.lastAct = clickAct{link: true}
				m.followLink(it.URL)
			}
			return
		}
	}
}

func (m *Model) mouseMotion(ms tea.Mouse) {
	switch {
	case m.dragBorder:
		m.sidebarW = max(8, min(ms.X+1, m.width-minDocWidth))
		m.dragWidth = m.sidebarW
	case m.dragSel && m.mode == modeEdit:
		line, col, _, ok := m.posAt(ms.X, ms.Y)
		if !ok {
			return
		}
		b := m.doc.buf
		b.Row, b.Col, b.Want = line, col, -1
		if mk, ok := m.km.(mouseKeymap); ok && m.vimFlavor { // vim: the drag starts visual mode
			if line != b.SelRow || col != b.SelCol || b.SelOn {
				mk.StartVisual(b, b.SelRow, b.SelCol)
			}
			b.BreakTyping()
			m.follow = true
			return
		}
		b.SelOn = line != b.SelRow || col != b.SelCol
		b.BreakTyping()
		m.follow = true
	}
}

func (m *Model) mouseRelease(ms tea.Mouse) {
	m.dragSel = false
	if !m.dragBorder {
		return
	}
	m.dragBorder = false
	w := m.treeW()
	if w == 0 || w == m.cfg.SidebarWidth {
		return
	}
	m.cfg.SidebarWidth = w
	save := m.opts.SaveSidebarWidth
	if save == nil {
		save = config.SaveSidebarWidth
	}
	m.cmd(func() tea.Msg { return widthSavedMsg{err: save(w)} })
}

func (m *Model) mouseWheel(ms tea.Mouse) {
	d := 3
	if ms.Button == tea.MouseWheelUp {
		d = -3
	}
	switch {
	case m.themes != nil || m.dlg != nil:
	case m.help != nil:
		m.help.off += d
	case m.treeRect().has(ms.X, ms.Y):
		m.tree.Scroll(d)
	case m.docRect().has(ms.X, ms.Y):
		m.doc.off += d
	}
}
