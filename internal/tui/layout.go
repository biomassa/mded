package tui

import (
	"fmt"
	"sort"

	"charm.land/lipgloss/v2"

	"github.com/biomassa/mded/internal/buffer"
	"github.com/biomassa/mded/internal/markdown"
	"github.com/biomassa/mded/internal/theme"
	"github.com/biomassa/mded/internal/ui"
)

// rect is a screen area in cells.
type rect struct{ x, y, w, h int }

func (r rect) has(x, y int) bool { return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h }

// minDocWidth is the smallest width of the document pane, borders included.
const minDocWidth = 20

// treeW is the width of the sidebar, borders included. It is 0 if the sidebar is hidden.
func (m *Model) treeW() int {
	if !m.showTree {
		return 0
	}
	return max(8, min(m.sidebarW, m.width-minDocWidth))
}

// paneH is the height of a pane, borders included. The status line has the last row.
func (m *Model) paneH() int { return max(3, m.height-1) }

func (m *Model) treeRect() rect { return rect{0, 0, m.treeW(), m.paneH()} }

func (m *Model) docRect() rect {
	w := m.treeW()
	return rect{w, 0, max(4, m.width-w), m.paneH()}
}

// textW is the text width of the document: the pane without borders, and a margin on
// each side.
func (m *Model) textW() int { return max(1, m.docRect().w-4) }

// bodyW is the text width of a row that is not a heading bar: textW less the indent.
func (m *Model) bodyW() int { return markdown.BodyWidth(m.textW()) }

// textTop is the number of pane lines above the text: the lines of the find box.
func (m *Model) textTop() int { return len(m.findLn) }

// textH is the number of rows that show text.
func (m *Model) textH() int { return max(1, m.docRect().h-2-m.textTop()) }

// matchSpan is a search match in a line.
type matchSpan struct {
	a, b int
	cur  bool
}

// spansFor returns the matches to highlight in a source line: the matches of the find box
// while it is open, else the matches of the search while the highlight is on. The matches are
// in text order, so a binary search finds the line. Thus the cost does not grow with the
// number of matches in the document.
func (m *Model) spansFor(line int) []matchSpan {
	b := m.doc.buf
	var ms []buffer.Match
	cur := -1
	switch {
	case m.find != nil:
		ms, cur = m.find.Matches, m.find.Cur
	case b.Search.Highlight:
		ms, cur = b.Search.Matches, b.Search.Cur
	default:
		return nil
	}
	k := sort.Search(len(ms), func(k int) bool { return ms[k].Row >= line })
	var out []matchSpan
	for ; k < len(ms) && ms[k].Row == line; k++ {
		out = append(out, matchSpan{ms[k].A, ms[k].B, k == cur})
	}
	return out
}

// matchStyle is the look of a search match: a yellow background, stronger for the current one.
func matchStyle(st lipgloss.Style, cur bool) lipgloss.Style {
	bg := theme.Mix("#E0B000", theme.BaseBg, 0.6)
	if cur {
		bg = "#E0B000"
	}
	return st.Background(theme.C(bg)).Foreground(theme.C(theme.BaseBg)).Bold(cur)
}

// selectionStyle is the look of selected text.
func selectionStyle(st lipgloss.Style) lipgloss.Style {
	return st.Background(theme.C(theme.SelBg)).Foreground(theme.C(theme.Text))
}

// layoutOpts returns the layout options for the current state.
func (m *Model) layoutOpts(tw int) markdown.LayoutOpts {
	d := m.doc
	b := d.buf
	editing := m.mode == modeEdit
	return markdown.LayoutOpts{
		Width:     tw,
		Plain:     d.plain,
		ImageSize: m.imageSize,
		AsSource: func(i int) bool {
			if editing && (m.find != nil || i == b.Row || b.SelLine(i)) {
				return true
			}
			return len(m.spansFor(i)) > 0
		},
		Decorate: func(line, col int, st *lipgloss.Style) *lipgloss.Style {
			if editing && b.InSel(line, col) {
				s := selectionStyle(*st)
				return &s
			}
			for _, s := range m.spansFor(line) {
				if col >= s.a && col < s.b {
					ms := matchStyle(*st, s.cur)
					return &ms
				}
			}
			return st
		},
	}
}

// blocksOf returns the code blocks and tables of the document. It finds them again after a
// change of the text.
func (m *Model) blocksOf() []markdown.Block {
	d := m.doc
	if d.plain || d.image {
		return nil
	}
	if !m.blkOK || m.blkDoc != d || m.blkRev != d.buf.Rev {
		m.blk, m.blkDoc, m.blkRev, m.blkOK = markdown.Blocks(d.buf.Lines), d, d.buf.Rev, true
	}
	return m.blk
}

// unitRows returns the rows of the layout unit that starts at line from (a line, or a
// whole code block or table). relayout sets the options first.
func (m *Model) unitRows(from int) []markdown.Row {
	return m.cache.UnitRows(m.doc.buf.Lines, m.lo, m.blk, from)
}

// The view of the document starts at an anchor: the first line of a unit (doc.top) and a
// row in its rows (doc.sub). Only the rows from the anchor to the bottom of the pane are
// built. A move of the view walks the anchor row by row through the units, so a move costs
// the rows that it passes, not the size of the document.

// walk moves the anchor n rows down (n > 0) or up (n < 0). It stops at the first row and at
// the last row of the document.
func (m *Model) walk(n int) {
	d := m.doc
	last := len(d.buf.Lines) - 1
	for n > 0 {
		cnt := len(m.unitRows(d.top))
		if d.sub+n < cnt {
			d.sub += n
			return
		}
		_, to := markdown.Unit(m.blk, d.top)
		if to >= last {
			d.sub = max(0, cnt-1)
			return
		}
		n -= cnt - d.sub
		d.top, d.sub = to+1, 0
	}
	for n < 0 {
		if d.sub+n >= 0 {
			d.sub += n
			return
		}
		if d.top == 0 {
			d.sub = 0
			return
		}
		n += d.sub
		from, _ := markdown.Unit(m.blk, d.top-1)
		d.top, d.sub = from, len(m.unitRows(from))
	}
}

// fixAnchor makes the anchor the first line of its unit. An edit can change the units: a new
// fence makes the lines after it a block.
func (m *Model) fixAnchor() {
	d := m.doc
	d.top = max(0, min(d.top, len(d.buf.Lines)-1))
	from, _ := markdown.Unit(m.blk, d.top)
	if from == d.top {
		return
	}
	rows := m.unitRows(from)
	k := 0
	for k < len(rows) && rows[k].Line < d.top {
		k++
	}
	d.top, d.sub = from, k
}

// lineAnchor returns the anchor of the first text row of a source line. With col >= 0, it
// is the row that has that column (the cursor).
func (m *Model) lineAnchor(line, col int) (top, sub int) {
	from, _ := markdown.Unit(m.blk, line)
	rows := m.unitRows(from)
	k := -1
	if col >= 0 {
		k = markdown.CursorRow(rows, line, col)
	}
	if k < 0 {
		for i, r := range rows {
			if r.Line == line && r.Img == nil {
				k = i
				break
			}
		}
	}
	return from, max(0, k)
}

// before reports whether the anchor (top, sub) comes before the anchor of the view.
func (m *Model) before(top, sub int) bool {
	d := m.doc
	return top < d.top || top == d.top && sub < d.sub
}

// rowsTo returns the number of rows from the anchor of the view to the anchor (top, sub),
// or -1 if it is limit rows or more, or above the view.
func (m *Model) rowsTo(top, sub, limit int) int {
	d := m.doc
	if m.before(top, sub) {
		return -1
	}
	n := -d.sub
	for i := d.top; i < len(d.buf.Lines) && n < limit; {
		if i == top {
			if n += sub; n < limit {
				return n
			}
			return -1
		}
		_, to := markdown.Unit(m.blk, i)
		n += len(m.unitRows(i))
		i = to + 1
	}
	return -1
}

// show moves the view so that the anchor (top, sub) is on the screen. If it is below the
// screen, it goes to the row at, or it goes to the top with at < 0.
func (m *Model) show(top, sub, th, at int) {
	if m.before(top, sub) {
		m.doc.top, m.doc.sub = top, sub
		return
	}
	if m.rowsTo(top, sub, th) >= 0 {
		return
	}
	m.doc.top, m.doc.sub = top, sub
	if at > 0 {
		m.walk(-at)
	}
}

// window builds the rows from the anchor down: at most th rows.
func (m *Model) window(th int) []markdown.Row {
	d := m.doc
	var rows []markdown.Row
	sub := d.sub
	for i := d.top; i < len(d.buf.Lines) && len(rows) < th; {
		_, to := markdown.Unit(m.blk, i)
		if ur := m.unitRows(i); sub < len(ur) {
			rows = append(rows, ur[sub:]...)
		}
		sub = 0
		i = to + 1
	}
	return rows[:min(len(rows), th)]
}

// highlightRow returns a copy of the row with the cells of the item reversed. It never
// changes the cells of the original: the markdown cache owns them.
func highlightRow(r markdown.Row, it markdown.Item) markdown.Row {
	if r.Cells == nil {
		return r
	}
	cells := make([]markdown.Cell, len(r.Cells))
	copy(cells, r.Cells)
	hit := false
	for i, c := range cells {
		if c.Src >= it.Start && c.Src < it.End {
			rev := c.St.Reverse(true)
			cells[i].St = &rev
			hit = true
		}
	}
	if !hit {
		return r
	}
	r.Cells, r.Pre = cells, ""
	return r
}

// getItems returns the checkboxes and links of the document. It finds them again after a
// change of the text.
func (m *Model) getItems() []markdown.Item {
	d := m.doc
	if d.plain {
		return nil
	}
	if !m.itemsOK || m.itemsDoc != d || m.itemsRev != d.buf.Rev {
		m.items, m.itemsDoc, m.itemsRev, m.itemsOK = markdown.Items(d.buf.Lines), d, d.buf.Rev, true
	}
	return m.items
}

// relayout builds the rows on the screen, moves the view, and starts the image work. It runs
// at the end of each Update, so that View only draws.
func (m *Model) relayout() {
	if m.width == 0 || m.doc == nil {
		return
	}
	d := m.doc
	m.findLn = m.buildFindLines()
	tw, th := m.textW(), m.textH()
	if d.image {
		m.rows, d.off = m.imageDocRows(tw, th), 0
		m.placeImages(tw, th)
		m.pickerSync()
		return
	}
	m.blk = m.blocksOf()
	m.lo = m.layoutOpts(tw)
	m.fixAnchor()
	switch { // the scroll request of the keys and the mouse
	case d.toEnd:
		last := len(d.buf.Lines) - 1
		from, _ := markdown.Unit(m.blk, last)
		d.top, d.sub = from, max(0, len(m.unitRows(from))-1)
		m.walk(-(th - 1))
	case d.off != 0:
		m.walk(d.off)
	}
	d.off, d.toEnd = 0, false
	if m.reveal.active {
		m.reveal.active = false
		top, sub := m.lineAnchor(m.reveal.line, -1)
		if m.reveal.top {
			d.top, d.sub = top, sub
		} else {
			m.show(top, sub, th, th/3)
		}
	}
	if m.follow && m.mode == modeEdit {
		top, sub := m.lineAnchor(d.buf.Row, d.buf.Col)
		m.show(top, sub, th, th-1)
	}
	m.follow = false
	m.rows = m.window(th)
	if len(m.rows) < th && (d.top > 0 || d.sub > 0) { // the end of the document is above the bottom
		m.walk(-(th - len(m.rows)))
		m.rows = m.window(th)
	}
	if m.mode == modeView && m.itemSel >= 0 {
		if items := m.getItems(); m.itemSel < len(items) {
			it := items[m.itemSel]
			for i, r := range m.rows {
				if r.Line == it.Line && r.Img == nil {
					m.rows[i] = highlightRow(r, it)
				}
			}
		}
	}
	if m.mode == modeView && m.vsel.on {
		m.rows = m.selectRows(m.rows)
	}
	d.buf.Off = m.topLine() // vim H, M, and L need the top line
	m.placeImages(tw, th)
	m.pickerSync()
}

// buildFindLines draws the find box for the width of the pane, or returns nil.
func (m *Model) buildFindLines() []string {
	if m.find == nil || m.mode != modeEdit {
		return nil
	}
	w := m.docRect().w - 2
	f := m.find
	check := func(on bool, label string, field int) string {
		mark := "[ ]"
		if on {
			mark = "[x]"
		}
		s := theme.St(theme.Text)
		if f.Focus == field {
			s = s.Reverse(true)
		}
		return s.Render(mark + " " + label)
	}
	inner := w - 2
	lines := []string{f.Find.View(), f.Repl.View(), " " + check(f.MatchCase, "match case", 2)}
	with := "“" + f.Repl.Value() + "”"
	if f.Repl.Value() == "" {
		with = "nothing"
	}
	hint := "enter next · ^r replace with " + with + " · ^a replace all · tab field · esc close"
	lines = append(lines, ui.WrapHint(hint, theme.Dim, inner)...)
	title := "Find"
	switch {
	case f.Find.Value() == "":
	case len(f.Matches) == 0:
		title += " · no match"
	default:
		title += fmt.Sprintf(" · %d of %d", f.Cur+1, len(f.Matches))
	}
	box := ui.Box(theme.St(theme.Fg(theme.Accent)).Bold(true).Render(title), ui.PadLines(lines, inner, len(lines)), w, theme.Fg(theme.Accent))
	return splitLines(box)
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
