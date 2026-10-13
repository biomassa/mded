package tui

import (
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/biomassa/mded/internal/kitty"
	"github.com/biomassa/mded/internal/markdown"
	"github.com/biomassa/mded/internal/theme"
	"github.com/biomassa/mded/internal/ui"
)

// viewHints is the hint text of view mode.
const viewHints = "e edit · / search · tab links · T theme · ? help · q quit"

// emptyHint is the text of the pane when no file is open.
const emptyHint = "no file open · pick one in the sidebar"

// View draws the screen. It only reads the state that Update built.
func (m *Model) View() tea.View {
	m.shown.CompareAndSwap(0, time.Now().UnixNano())
	v := tea.NewView(m.screen())
	v.MouseMode = tea.MouseModeCellMotion // clicks, wheel, and motion while a button is down
	v.AltScreen = true
	v.WindowTitle = "mded"
	if bg := theme.Bg(); bg != "" { // a palette theme sets the terminal background while mded runs
		v.BackgroundColor = theme.C(bg)
	}
	return v
}

// screen returns the whole screen as one string.
func (m *Model) screen() string {
	if m.width == 0 || m.doc == nil {
		return ""
	}
	dr := m.docRect()
	var panes []string
	if m.showTree {
		tr := m.treeRect()
		border := theme.Border
		title := theme.St(theme.Muted).Bold(true).Render(m.tree.Title())
		if m.focus == focusTree {
			border = theme.Accent
			title = theme.St(theme.Fg(theme.Accent)).Bold(true).Render(m.tree.Title())
		}
		panes = append(panes, ui.Box(title, m.tree.View(tr.w-2, tr.h-2, m.focus == focusTree), tr.w, border))
	}
	border := theme.Border
	title := theme.St(theme.Muted).Bold(true).Render(m.doc.title())
	if m.focus == focusDoc {
		border = theme.Accent
		title = theme.St(theme.Fg(theme.Accent)).Bold(true).Render(m.doc.title())
	}
	panes = append(panes, ui.Box(title, m.docLines(dr.w-2, dr.h-2), dr.w, border))
	screen := lipgloss.JoinHorizontal(lipgloss.Top, panes...) + "\n" + m.statusLine()
	return m.overlays(screen)
}

// docLines draws the inner lines of the document pane.
func (m *Model) docLines(iw, ph int) []string {
	d := m.doc
	var out []string
	if m.findLn != nil {
		out = append(out, m.findLn[:min(len(m.findLn), ph-1)]...)
	}
	if d.path == "" && len(d.buf.Lines) == 1 && len(d.buf.Lines[0]) == 0 && m.mode == modeView {
		out = append(out, " "+theme.St(theme.Dim).Render(emptyHint))
		return ui.PadLines(out, iw, ph)
	}
	th := ph - len(out)
	tw := iw - 2
	b := d.buf
	cy := -1
	if m.mode == modeEdit && m.focus == focusDoc {
		cy = markdown.CursorRow(m.rows, b.Row, b.Col)
	}
	fill := lipgloss.NewStyle()
	for i := d.off; i < d.off+th && i < len(m.rows); i++ {
		r := m.rows[i]
		var s string
		switch {
		case r.Img != nil:
			if p, ok := m.placed[i]; ok {
				s = " " + kitty.RowString(p.id, r.ImgRow, p.cols)
			}
		default:
			cx := -1
			if i == cy {
				cx = b.Col - r.Start
			}
			s = markdown.RenderRow(r, tw, cx)
		}
		out = append(out, ui.Pad(s, iw, fill))
	}
	return ui.PadLines(out, iw, ph)
}

// ---- status line ----

// position returns the "line:col" and the percent for the right side of the status line.
func (m *Model) position() string {
	d := m.doc
	b := d.buf
	if m.mode == modeEdit {
		pct := 100
		if n := len(b.Lines); n > 0 {
			pct = (b.Row + 1) * 100 / n
		}
		return strconv.Itoa(b.Row+1) + ":" + strconv.Itoa(b.Col+1) + "  " + strconv.Itoa(pct) + "%"
	}
	pct, line := 100, 1
	if n := len(m.rows); n > 0 {
		top := min(d.off, n-1)
		line = m.rows[top].Line + 1
		pct = min(n, d.off+m.textH()) * 100 / n
	}
	return strconv.Itoa(line) + ":1  " + strconv.Itoa(pct) + "%"
}

// promptLine returns the text of the prompt that owns the status line, if there is one.
// cursor is the position of the text cursor in cells, or -1.
func (m *Model) promptLine() (text string, cursor int, note string, ok bool) {
	switch {
	case m.dlg != nil:
		q, keys := m.dialogText()
		return theme.St(theme.Text).Bold(true).Render(q) + "  " + theme.St(theme.Dim).Render(keys), -1, "", true
	case m.search != nil:
		t, c := m.search.line()
		return t, c, m.search.note, true
	}
	if line, on := m.tree.Prompt(); on {
		return line, -1, "", true
	}
	if m.mode == modeEdit && m.focus == focusDoc && m.find == nil {
		if t, c, on := m.km.StatusLine(); on {
			return t, c, "", true
		}
	}
	return "", -1, "", false
}

// withCursor draws a plain text with a block cursor at rune position c.
func withCursor(text string, c int) string {
	rs := []rune(text)
	if c < 0 || c > len(rs) {
		return text
	}
	cur := lipgloss.NewStyle().Reverse(true)
	if c == len(rs) {
		return string(rs) + cur.Render(" ")
	}
	return string(rs[:c]) + cur.Render(string(rs[c])) + string(rs[c+1:])
}

// statusLine draws the last row: the mode label, the path, the hints or a message, and
// the position. A prompt replaces all of it.
func (m *Model) statusLine() string {
	w := m.width
	base := lipgloss.NewStyle()
	if text, cur, note, ok := m.promptLine(); ok {
		if cur >= 0 {
			text = withCursor(text, cur)
		}
		right := ""
		if note != "" {
			right = theme.St(theme.Error).Render(note)
		}
		return joinEnds(text, right, w, base)
	}
	d := m.doc
	label := "VIEW"
	if m.mode == modeEdit {
		label = m.km.ModeLabel()
	}
	left := theme.St(theme.Accent).Reverse(true).Bold(true).Render(" "+label+" ") + " "
	path := d.path
	if path == "" {
		path = "[new]"
	}
	left += theme.St(theme.Text).Render(path)
	if d.buf.Dirty() {
		left += " " + theme.St(theme.Warn).Render("[+]")
	}
	right := theme.St(theme.Muted).Render(m.position())
	mid := ""
	switch {
	case m.msg != "":
		hex := theme.Text
		if m.msgErr {
			hex = theme.Error
		}
		mid = theme.St(hex).Render(m.msg)
	default:
		mid = theme.St(theme.Dim).Render(m.hints())
	}
	// The message wins over the position and the path when the line is short.
	avail := w - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if m.msg != "" && lipgloss.Width(mid) > avail {
		right = ""
		avail = w - lipgloss.Width(left) - 2
		if lipgloss.Width(mid) > avail {
			left = theme.St(theme.Accent).Reverse(true).Bold(true).Render(" " + label + " ")
			avail = w - lipgloss.Width(left) - 2
		}
	}
	if avail > 0 {
		mid = ui.Trunc(mid, avail)
	} else {
		mid = ""
	}
	gap := w - lipgloss.Width(left) - lipgloss.Width(mid) - lipgloss.Width(right)
	if mid == "" {
		return joinEnds(left, right, w, base)
	}
	g1 := max(1, gap/2)
	if gap < 2 {
		return joinEnds(left+" "+mid, right, w, base)
	}
	return ui.Pad(left+strings.Repeat(" ", g1)+mid+strings.Repeat(" ", gap-g1)+right, w, base)
}

// joinEnds puts left and right at the two ends of a line of width w.
func joinEnds(left, right string, w int, base lipgloss.Style) string {
	if right == "" {
		return ui.Pad(left, w, base)
	}
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		left = ansi.Truncate(left, max(0, w-lipgloss.Width(right)-1), "…")
		gap = max(1, w-lipgloss.Width(left)-lipgloss.Width(right))
	}
	return ui.Pad(left+strings.Repeat(" ", gap)+right, w, base)
}

// hints returns the key hints of the focus and mode.
func (m *Model) hints() string {
	switch {
	case m.focus == focusTree:
		return m.tree.Hints()
	case m.mode == modeEdit:
		return m.km.Hints()
	}
	return viewHints
}

// overlays draws the help and the theme picker over the screen.
func (m *Model) overlays(screen string) string {
	layers := []*lipgloss.Layer{lipgloss.NewLayer(screen)}
	if m.help != nil {
		r := m.helpRect()
		layers = append(layers, lipgloss.NewLayer(m.helpBox()).X(r.x).Y(r.y).Z(1))
	}
	if m.themes != nil {
		r := m.themeRect()
		layers = append(layers, lipgloss.NewLayer(m.themeBox()).X(r.x).Y(r.y).Z(2))
	}
	if len(layers) == 1 {
		return screen
	}
	return lipgloss.NewCompositor(layers...).Render()
}
