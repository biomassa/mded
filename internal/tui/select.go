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

// selPos is a point of the selection in the source: a line and a rune index. The view
// changes while the mouse drags (it scrolls), so the selection keeps source positions, not
// screen rows.
type selPos struct{ line, col int }

// before reports whether p is before q in the text.
func (p selPos) before(q selPos) bool {
	return p.line < q.line || p.line == q.line && p.col <= q.col
}

// selPoint is a point under the mouse in two forms: as the start of a selection (the first
// rune that it has) and as the end (the index after the last rune that it has).
type selPoint struct{ s, e selPos }

// viewSel is the mouse selection of view mode.
type viewSel struct {
	drag   bool // the button is down
	on     bool // the selection shows: the pointer left the first cell
	dbl    bool // the press completed a double-click
	px, py int  // the screen cell of the press
	a, h   selPoint
}

// span returns the selection in the source: from lo to hi, hi not included.
func (s viewSel) span() (lo, hi selPos) {
	lo, hi = s.a.s, s.h.e
	if s.h.s.before(s.a.s) && s.h.s != s.a.s {
		lo, hi = s.h.s, s.a.e
	}
	return lo, hi
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

// pointAt returns the source point under a screen cell. A cell above the text gives the
// first row on the screen, and a cell below it the last row on the screen. ok is false if
// there are no rows.
func (m *Model) pointAt(x, y int) (selPoint, bool) {
	if len(m.rows) == 0 {
		return selPoint{}, false
	}
	top := m.docRect().y + 1 + m.textTop()
	i := max(0, min(y-top, min(m.textH(), len(m.rows))-1))
	col := m.cellAt(m.rows[i], x)
	if y-top >= len(m.rows) { // below the last row: the end of it
		col = 1 << 30
	}
	sl, sc := m.srcPos(i, col, false)
	el, ec := m.srcPos(i, col, true)
	return selPoint{selPos{sl, sc}, selPos{el, ec}}, true
}

// viewPress starts a selection in view mode. The click action waits for the release.
func (m *Model) viewPress(x, y int, dbl bool) {
	if m.doc.image || m.rowAt(y) < 0 {
		return
	}
	p, ok := m.pointAt(x, y)
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
	switch { // the pane scrolls one row, and the point is on the new screen
	case y < top:
		m.doc.off--
		m.relayout()
	case y >= top+m.textH():
		m.doc.off++
		m.relayout()
	}
	if p, ok := m.pointAt(x, y); ok {
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
func (m *Model) srcPos(row, cell int, end bool) (line, col int) {
	r := m.rows[row]
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
	c := max(0, cell)
	past := c > last
	switch {
	case past && end && (r.Fill > 0 || m.lastRowOfLine(row)): // the padding of a bar, or the end of the line
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

// selSource returns the markdown source of the selection.
func (m *Model) selSource() string {
	lo, hi := m.vsel.span()
	lines := m.doc.buf.Lines
	if lo.line < 0 || hi.line >= len(lines) || !lo.before(hi) {
		return ""
	}
	l1, c1 := lo.line, min(lo.col, len(lines[lo.line]))
	l2, c2 := hi.line, min(hi.col, len(lines[hi.line]))
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
// changes, because the markdown cache owns them. A cell is selected if its source rune is in
// the selection. A cell without a source rune (a table border) is selected if it is between
// selected cells, or if its whole line is in the selection. A row without cells shows the
// selection in its full width if its line is in the selection. An image row and an empty row
// show none.
func (m *Model) selectRows(rows []markdown.Row) []markdown.Row {
	lo, hi := m.vsel.span()
	in := func(line, col int) bool {
		p := selPos{line, col}
		return lo.before(p) && !(hi.before(p))
	}
	tw := m.textW()
	for i, r := range rows {
		if r.Img != nil || r.Line < lo.line || r.Line > hi.line {
			continue
		}
		if len(r.Cells) == 0 {
			if plain := strings.TrimRight(ansi.Strip(r.Pre), " "); plain != "" {
				// The margin stays plain. The rest has the selection style, to the text width.
				text := ansi.Truncate(strings.TrimPrefix(plain, " "), tw, "")
				r.Pre = " " + selectionStyle(lipgloss.NewStyle()).Render(text+strings.Repeat(" ", max(0, tw-ansi.StringWidth(text))))
				rows[i] = r
			}
			continue
		}
		whole := r.Line > lo.line && r.Line < hi.line
		first, last := -1, -1
		for k, c := range r.Cells {
			if c.Src >= 0 && (whole || in(r.Line, c.Src)) {
				if first < 0 {
					first = k
				}
				last = k
			}
		}
		if first < 0 && !whole {
			continue
		}
		if whole {
			first, last = 0, len(r.Cells)-1
		}
		cells := make([]markdown.Cell, len(r.Cells))
		copy(cells, r.Cells)
		for k := first; k <= last; k++ {
			sel := selectionStyle(*cells[k].St)
			cells[k].St = &sel
		}
		r.Cells, r.Pre = cells, ""
		rows[i] = r
	}
	return rows
}
