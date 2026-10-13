package tui

import (
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/biomassa/mded/internal/markdown"
)

// selPos is a point of the screen text: the index of a row in Model.rows, and the index of a
// cell in that row. A cell index less than 0 is left of the first cell. An index of
// len(Cells) or more is right of the last cell.
type selPos struct{ row, col int }

// before reports whether p is before q in the reading order.
func (p selPos) before(q selPos) bool {
	return p.row < q.row || p.row == q.row && p.col <= q.col
}

// viewSel is the mouse selection of view mode.
type viewSel struct {
	drag   bool // the button is down
	on     bool // the selection shows: the pointer left the first cell
	dbl    bool // the press completed a double-click
	px, py int  // the screen cell of the press
	a, h   selPos
}

// cellAt returns the index of the cell of row r under the screen column x: -1 left of the
// first cell, len(r.Cells) right of the last cell. A wide rune covers two columns.
func (m *Model) cellAt(r markdown.Row, x int) int {
	k := x - m.docRect().x - 2 - r.Indent() // the pane border, the margin, and the indent
	if k < 0 {
		return -1
	}
	rs := make([]rune, 0, len(r.Cells))
	for i, cl := range r.Cells {
		rs = append(rs, cl.R)
		if k < ansi.StringWidth(string(rs)) {
			return i
		}
	}
	return len(r.Cells)
}

// clampOff keeps a scroll offset inside the document.
func (m *Model) clampOff(off int) int {
	return max(0, min(off, len(m.rows)-m.textH()))
}

// selPosAt returns the point of the text under a screen cell. A cell above the text gives
// the first row on the screen, and a cell below it the last row on the screen. Below the
// last row of the document, it is the end of the last row. ok is false if there are no rows.
func (m *Model) selPosAt(x, y int) (selPos, bool) {
	if len(m.rows) == 0 {
		return selPos{}, false
	}
	top := m.docRect().y + 1 + m.textTop()
	i := m.doc.off + y - top
	i = max(m.doc.off, min(i, m.doc.off+m.textH()-1))
	if i >= len(m.rows) {
		return selPos{len(m.rows) - 1, 1 << 30}, true
	}
	return selPos{i, m.cellAt(m.rows[i], x)}, true
}

// viewPress starts a selection in view mode. The click action waits for the release.
func (m *Model) viewPress(x, y int, dbl bool) {
	if m.doc.image || m.rowAt(y) < 0 {
		return
	}
	p, ok := m.selPosAt(x, y)
	if !ok {
		return
	}
	m.vsel = viewSel{drag: true, dbl: dbl, px: x, py: y, a: p, h: p}
}

// viewDrag moves the end of the selection to the pointer. A pointer above or below the
// text scrolls the view by one row.
func (m *Model) viewDrag(x, y int) {
	s := &m.vsel
	if x != s.px || y != s.py {
		s.on = true
	}
	top := m.docRect().y + 1 + m.textTop()
	switch {
	case y < top:
		m.doc.off = m.clampOff(m.doc.off - 1)
	case y >= top+m.textH():
		m.doc.off = m.clampOff(m.doc.off + 1)
	}
	if p, ok := m.selPosAt(x, y); ok {
		s.h = p
	}
}

// viewRelease ends a drag. A release on the cell of the press is a click. Else the selected
// source goes to the clipboard.
func (m *Model) viewRelease() {
	s := m.vsel
	m.vsel = viewSel{on: s.on, a: s.a, h: s.h}
	if !s.on {
		m.vsel = viewSel{}
		m.viewClick(s.px, s.py, s.dbl)
		return
	}
	text := m.selSource()
	n := utf8.RuneCountInString(text)
	if n == 0 {
		m.vsel = viewSel{}
		return
	}
	m.cmd(tea.SetClipboard(text))
	if n == 1 {
		m.status("copied 1 character", false)
	} else {
		m.status("copied "+strconv.Itoa(n)+" characters", false)
	}
}

// lastRowOfLine reports whether row i is the last text row of its source line.
func (m *Model) lastRowOfLine(i int) bool {
	return i+1 >= len(m.rows) || m.rows[i+1].Line != m.rows[i].Line || m.rows[i+1].Img != nil
}

// srcPos maps a point of the screen to a position in the source: the line, and the rune
// index in it. For a start it is the first rune that the selection has. For an end it is
// the index after the last rune that the selection has. A row without cells (a code block,
// a rule, an image, an empty line) maps to its whole source line. A cell without a source
// rune (a table border) maps to the nearest cell that has one, toward the inside of the
// selection.
func (m *Model) srcPos(p selPos, end bool) (line, col int) {
	r := m.rows[p.row]
	n := len(m.doc.buf.Lines[r.Line])
	whole := func() (int, int) {
		if end {
			return r.Line, n
		}
		return r.Line, 0
	}
	if r.Img != nil || len(r.Cells) == 0 {
		return whole()
	}
	last := len(r.Cells) - 1
	c := max(0, p.col)
	past := c > last
	switch {
	case past && end && (r.Fill > 0 || m.lastRowOfLine(p.row)): // the padding of a bar, or the end of the line
		return r.Line, n
	case past:
		c = last
	}
	src := -1
	if end || past { // toward the start of the row
		for i := c; i >= 0; i-- {
			if src = r.Cells[i].Src; src >= 0 {
				break
			}
		}
	} else { // toward the end of the row
		for i := c; i <= last; i++ {
			if src = r.Cells[i].Src; src >= 0 {
				break
			}
		}
	}
	if src < 0 {
		return whole()
	}
	if end || past {
		src++
	}
	return r.Line, max(0, min(src, n))
}

// selSource returns the markdown source behind the selected cells.
func (m *Model) selSource() string {
	s := m.vsel
	if len(m.rows) == 0 || s.a.row >= len(m.rows) || s.h.row >= len(m.rows) {
		return ""
	}
	a, h := s.a, s.h
	if !a.before(h) {
		a, h = h, a
	}
	l1, c1 := m.srcPos(a, false)
	l2, c2 := m.srcPos(h, true)
	lines := m.doc.buf.Lines
	if l2 < l1 {
		l1, c1, l2, c2 = l2, c2, l1, c1
	}
	if l1 == l2 {
		if c2 <= c1 {
			return ""
		}
		return string(lines[l1][c1:c2])
	}
	parts := []string{string(lines[l1][c1:])}
	for i := l1 + 1; i < l2; i++ {
		parts = append(parts, string(lines[i]))
	}
	parts = append(parts, string(lines[l2][:c2]))
	return strings.Join(parts, "\n")
}

// selectRows returns the rows with the selection drawn. It copies the cells that it
// changes, because the markdown cache owns them. A row without cells shows the selection
// in its full width. An image row and an empty row show none.
func (m *Model) selectRows(rows []markdown.Row) []markdown.Row {
	a, h := m.vsel.a, m.vsel.h
	if !a.before(h) {
		a, h = h, a
	}
	tw := m.textW()
	for i := a.row; i <= h.row && i < len(rows); i++ {
		r := rows[i]
		switch {
		case r.Img != nil:
		case len(r.Cells) == 0:
			if plain := strings.TrimRight(ansi.Strip(r.Pre), " "); plain != "" {
				// The margin stays plain. The rest has the selection style, to the text width.
				text := ansi.Truncate(strings.TrimPrefix(plain, " "), tw, "")
				r.Pre = " " + selectionStyle(lipgloss.NewStyle()).Render(text+strings.Repeat(" ", max(0, tw-ansi.StringWidth(text))))
				rows[i] = r
			}
		default:
			lo, hi := 0, len(r.Cells)-1
			if i == a.row {
				lo = max(lo, a.col)
			}
			if i == h.row {
				hi = min(hi, h.col)
			}
			if lo > hi {
				continue
			}
			cells := make([]markdown.Cell, len(r.Cells))
			copy(cells, r.Cells)
			for k := lo; k <= hi; k++ {
				cells[k].St = selectionStyle(cells[k].St)
			}
			r.Cells, r.Pre = cells, ""
			rows[i] = r
		}
	}
	return rows
}
