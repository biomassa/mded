package tui

import (
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/biomassa/mded/internal/keymap/conv"
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
	v.Cursor = m.cursor()
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
			s = markdown.RenderRow(r, tw, -1) // the terminal draws the cursor: see cursor
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
// cursor is the position of the text cursor in runes, or -1.
func (m *Model) promptLine() (text string, cursor int, note string, ok bool) {
	switch {
	case m.dlg != nil:
		q, keys := m.dialogText()
		return theme.St(theme.Text).Bold(true).Render(q) + "  " + theme.St(theme.Dim).Render(keys), -1, "", true
	case m.img != nil && m.img.altOn:
		t, c := m.img.altLine()
		return t, c, "", true
	case m.search != nil:
		t, c := m.search.line()
		return t, c, m.search.note, true
	}
	if line, on := m.tree.Prompt(); on {
		return line, -1, "", true // m.cursor() asks the tree for the cursor column
	}
	if m.mode == modeEdit && m.focus == focusDoc && m.find == nil {
		if t, c, on := m.km.StatusLine(); on {
			return t, c, "", true
		}
	}
	return "", -1, "", false
}

// statusLine draws the last row: the mode label, the path, the hints or a message, and
// the position. A prompt replaces all of it.
func (m *Model) statusLine() string {
	w := m.width
	base := lipgloss.NewStyle()
	if text, _, note, ok := m.promptLine(); ok {
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
	if m.img != nil && !m.img.altOn {
		r := m.pickerRect()
		layers = append(layers, lipgloss.NewLayer(m.pickerBox()).X(r.x).Y(r.y).Z(3))
	}
	if len(layers) == 1 {
		return screen
	}
	return lipgloss.NewCompositor(layers...).Render()
}

// cursorShaper is the optional interface of a keymap that has modes: Mode returns
// "normal", "insert", "replace", "visual", or "vline". The vim engine has it. The
// package does not import the vim package.
type cursorShaper interface{ Mode() string }

// docCursorShape returns the shape of the document cursor for the keymap: a block in the
// normal and visual modes of vim, an underscore in all other cases.
func (m *Model) docCursorShape() tea.CursorShape {
	if k, ok := m.km.(cursorShaper); ok {
		switch k.Mode() {
		case "normal", "visual", "vline":
			return tea.CursorBlock
		}
	}
	return tea.CursorUnderline
}

// newCursor returns a blinking cursor in the accent color at x, y.
func newCursor(x, y int, shape tea.CursorShape) *tea.Cursor {
	c := tea.NewCursor(x, y)
	c.Color = theme.C(theme.Accent)
	c.Shape = shape
	c.Blink = true
	return c
}

// textWidth returns the width in cells of the first n runes of text.
func textWidth(text string, n int) int {
	rs := []rune(text)
	return ansi.StringWidth(string(rs[:max(0, min(n, len(rs)))]))
}

// cursor returns the terminal cursor for the screen, or nil if the cursor is hidden. Only
// one place has the cursor: an open prompt, then a field of the find box, then the
// document in edit mode. View mode, the popups without a text field, and the dialogs have
// none.
func (m *Model) cursor() *tea.Cursor {
	if m.width == 0 || m.doc == nil || m.dlg != nil || m.help != nil || m.themes != nil {
		return nil
	}
	if m.img != nil && !m.img.altOn { // the list of the image picker has no text field
		return nil
	}
	row := m.height - 1 // the status line
	statusAt := func(text string, c int) *tea.Cursor {
		x := min(textWidth(text, c), max(0, m.width-1))
		return newCursor(x, row, tea.CursorUnderline)
	}
	switch {
	case m.img != nil:
		t, c := m.img.altLine()
		return statusAt(t, c)
	case m.search != nil:
		t, c := m.search.line()
		return statusAt(t, c)
	}
	if _, on := m.tree.Prompt(); on {
		c := m.tree.PromptCursor()
		if c < 0 { // the question of a delete has no text field
			return nil
		}
		return newCursor(min(c, max(0, m.width-1)), row, tea.CursorUnderline)
	}
	if m.mode != modeEdit || m.focus != focusDoc {
		return nil
	}
	if m.find != nil {
		return m.findCursor()
	}
	if t, c, on := m.km.StatusLine(); on && c >= 0 {
		return statusAt(t, c)
	}
	return m.docCursor()
}

// findCursor returns the cursor of the focused text field of the find box, or nil if the
// focus is on the check box or if the field is not on the screen.
func (m *Model) findCursor() *tea.Cursor {
	f := m.find
	var in *textinput.Model
	switch f.Focus {
	case conv.FieldFind:
		in = &f.Find
	case conv.FieldReplace:
		in = &f.Repl
	default:
		return nil
	}
	dr := m.docRect()
	y := 2 + f.Focus // the pane border, the box border, then the fields in order
	if y >= dr.h-1 || y >= len(m.findLn) {
		return nil
	}
	rs := []rune(in.Value())
	pos := max(0, min(in.Position(), len(rs)))
	w := ansi.StringWidth(in.Prompt)
	cw := ansi.StringWidth(string(rs[:pos]))
	if n := in.Width(); n > 0 {
		cw = min(cw, n) // the field scrolls: the cursor stays in its width
	}
	x := dr.x + 2 + w + cw // the pane border and the box border
	return newCursor(min(x, dr.x+dr.w-3), y, tea.CursorUnderline)
}

// docCursor returns the cursor in the text of the document, or nil if its row is not on
// the screen.
func (m *Model) docCursor() *tea.Cursor {
	d := m.doc
	b := d.buf
	cy := markdown.CursorRow(m.rows, b.Row, b.Col)
	th := m.textH()
	if cy < 0 || cy < d.off || cy >= d.off+th || cy >= len(m.rows) {
		return nil
	}
	r := m.rows[cy]
	cx := 0
	if r.Img == nil {
		n := max(0, min(b.Col-r.Start, len(r.Cells)))
		var sb strings.Builder
		for _, cl := range r.Cells[:n] {
			sb.WriteRune(cl.R)
		}
		cx = ansi.StringWidth(sb.String())
	}
	dr := m.docRect()
	x := dr.x + 2 + 1 + cx // the pane border, the margin, and the cells before the cursor
	y := 1 + m.textTop() + cy - d.off
	return newCursor(min(x, dr.x+dr.w-2), y, m.docCursorShape())
}
