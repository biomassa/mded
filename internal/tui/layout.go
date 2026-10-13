package tui

import (
	"fmt"

	"charm.land/lipgloss/v2"

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

// textTop is the number of pane lines above the text: the lines of the find box.
func (m *Model) textTop() int { return len(m.findLn) }

// textH is the number of rows that show text.
func (m *Model) textH() int { return max(1, m.docRect().h-2-m.textTop()) }

// matchSpan is a search match in a line.
type matchSpan struct {
	a, b int
	cur  bool
}

// layoutKey holds everything that changes the rows. When it stays the same, the rows of the
// last layout are good.
type layoutKey struct {
	doc                   *doc
	rev                   int
	mode                  mode
	row, col              int
	selOn, selLW, selIncl bool
	selRow, selCol        int
	pattern               string
	hl                    bool
	cur, nMatch           int
	findOpen              bool
	findCur, findN        int
	itemSel               int
	tw, th                int
	themeGen, imgGen      int
	plain                 bool
	focus                 focusPane
}

func (m *Model) layoutKey(tw, th int) layoutKey {
	d := m.doc
	b := d.buf
	k := layoutKey{
		doc: d, rev: b.Rev, mode: m.mode, itemSel: m.itemSel, tw: tw, th: th,
		themeGen: m.themeGen, imgGen: m.imgGen, plain: d.plain, focus: m.focus,
		pattern: b.Search.Pattern, hl: b.Search.Highlight, cur: b.Search.Cur, nMatch: len(b.Search.Matches),
	}
	if m.mode == modeEdit {
		k.row, k.col = b.Row, b.Col
		k.selOn, k.selLW, k.selIncl, k.selRow, k.selCol = b.SelOn, b.SelLinewise, b.SelInclusive, b.SelRow, b.SelCol
	}
	if m.find != nil {
		k.findOpen, k.findCur, k.findN = true, m.find.Cur, len(m.find.Matches)
	}
	return k
}

// matchSpans returns the matches to highlight, by line: the matches of the find box while
// it is open, else the matches of the search while the highlight is on.
func (m *Model) matchSpans() map[int][]matchSpan {
	b := m.doc.buf
	out := map[int][]matchSpan{}
	switch {
	case m.find != nil:
		for i, mt := range m.find.Matches {
			out[mt.Row] = append(out[mt.Row], matchSpan{mt.A, mt.B, i == m.find.Cur})
		}
	case b.Search.Highlight:
		for i, mt := range b.Search.Matches {
			out[mt.Row] = append(out[mt.Row], matchSpan{mt.A, mt.B, i == b.Search.Cur})
		}
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

// buildRows makes the rows of the document for the current state.
func (m *Model) buildRows(tw, th int) []markdown.Row {
	d := m.doc
	if d.image {
		return m.imageDocRows(tw, th)
	}
	b := d.buf
	editing := m.mode == modeEdit
	spans := m.matchSpans()
	o := markdown.LayoutOpts{
		Width:     tw,
		Plain:     d.plain,
		ImageSize: m.imageSize,
		AsSource: func(i int) bool {
			if editing && (m.find != nil || i == b.Row || b.SelLine(i)) {
				return true
			}
			return len(spans[i]) > 0
		},
		Decorate: func(line, col int, st lipgloss.Style) lipgloss.Style {
			if editing && b.InSel(line, col) {
				return selectionStyle(st)
			}
			for _, s := range spans[line] {
				if col >= s.a && col < s.b {
					return matchStyle(st, s.cur)
				}
			}
			return st
		},
	}
	rows := m.cache.Layout(b.Lines, o)
	if !editing && m.itemSel >= 0 {
		if items := m.getItems(); m.itemSel < len(items) {
			it := items[m.itemSel]
			for i, r := range rows {
				if r.Line == it.Line && r.Img == nil {
					rows[i] = highlightRow(r, it)
				}
			}
		}
	}
	return rows
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
			cells[i].St = c.St.Reverse(true)
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

// relayout builds the rows, fixes the scroll position, and starts the image work. It runs
// at the end of each Update, so that View only draws.
func (m *Model) relayout() {
	if m.width == 0 || m.doc == nil {
		return
	}
	d := m.doc
	m.findLn = m.buildFindLines()
	tw, th := m.textW(), m.textH()
	if key := m.layoutKey(tw, th); !m.laid || key != m.lk {
		m.rows = m.buildRows(tw, th)
		m.lk, m.laid = key, true
	}
	if m.reveal.active {
		m.reveal.active = false
		if i := m.firstRowOf(m.reveal.line); i >= 0 {
			switch {
			case m.reveal.top:
				d.off = i
			case i < d.off || i >= d.off+th:
				d.off = max(0, i-th/3)
			}
		}
	}
	if m.follow && m.mode == modeEdit {
		if cy := markdown.CursorRow(m.rows, d.buf.Row, d.buf.Col); cy >= 0 {
			if cy < d.off {
				d.off = cy
			}
			if cy >= d.off+th {
				d.off = cy - th + 1
			}
		}
	}
	m.follow = false
	d.off = max(0, min(d.off, len(m.rows)-th))
	m.placeImages(tw, th)
}

// firstRowOf returns the index of the first text row of a source line, or -1.
func (m *Model) firstRowOf(line int) int {
	for i, r := range m.rows {
		if r.Line == line && r.Img == nil {
			return i
		}
	}
	return -1
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
