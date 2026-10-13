package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/biomassa/mded/internal/theme"
	"github.com/biomassa/mded/internal/ui"
)

// helpState is the open help overlay.
type helpState struct {
	off int // first visible line
}

// helpKey is one line of the help: a key and what it does. An empty key makes a heading.
type helpKey struct{ key, text string }

func (m *Model) openHelp() { m.help = &helpState{} }

// helpKey handles a key while the help is open.
func (m *Model) helpKey(k string) {
	switch k {
	case "esc", "q", "?", "f1":
		m.help = nil
	case "j", "down":
		m.help.off++
	case "k", "up":
		m.help.off--
	case "pgdown", "space":
		m.help.off += max(1, m.helpRect().h-3)
	case "pgup", "b":
		m.help.off -= max(1, m.helpRect().h-3)
	case "g", "home":
		m.help.off = 0
	case "G", "end":
		m.help.off = 1 << 20
	}
}

var (
	viewHelp = []helpKey{
		{"", "View mode"},
		{"j / ↓, k / ↑", "scroll one row"},
		{"ctrl+d, ctrl+u", "half page down, up"},
		{"space / pgdown, b / pgup", "page down, up"},
		{"g / home, G / end", "top, bottom"},
		{"/", "search"},
		{"n, N", "next, previous match"},
		{"tab, shift+tab", "select a checkbox or a link"},
		{"space", "toggle the selected checkbox"},
		{"enter", "follow the selected link"},
		{"backspace, ctrl+o", "back to the previous document"},
		{"esc", "clear the search highlight"},
		{"e, i", "edit"},
		{"T", "theme picker"},
		{"?, F1", "this help"},
		{"ctrl+s", "save"},
		{"q, ctrl+q", "quit"},
	}
	convHelp = []helpKey{
		{"", "Edit mode"},
		{"arrows, home, end", "move"},
		{"pgup, pgdown", "one screen up, down"},
		{"alt+←/→, ctrl+←/→", "move by word"},
		{"alt+backspace", "delete a word"},
		{"enter", "new line, continue a list"},
		{"tab", "insert spaces"},
		{"ctrl+b, ctrl+i", "bold, italic"},
		{"ctrl+k, ctrl+t", "link, checkbox"},
		{"ctrl+z, ctrl+y", "undo, redo"},
		{"shift+moves, ctrl+a", "select"},
		{"ctrl+c, ctrl+x, ctrl+v", "copy, cut, paste"},
		{"ctrl+f", "find and replace"},
		{"ctrl+s", "save"},
		{"ctrl+g", "switch to vim"},
		{"esc", "view mode"},
	}
	vimHelp = []helpKey{
		{"", "Vim edit mode"},
		{"h j k l, w b e, 0 ^ $, gg G", "motions, with counts"},
		{"f F t T ; ,  %  { }", "find a character, match, paragraph"},
		{"d c y > < gu gU g~", "operators, with motions and text objects"},
		{"x s r J ~ p P u ctrl+r .", "edits, undo, redo, repeat"},
		{"i a I A o O R", "insert and replace"},
		{"v, V", "visual mode, visual line mode"},
		{"\"a  \"+  \"_", "registers, clipboard, black hole"},
		{"ma  `a  'a", "marks"},
		{"qa  @a  @@", "macros"},
		{"/  n  N  *  #", "search"},
		{":w  :wq  :x  :q  :q!", "save and quit"},
		{":e FILE  :noh  :N  :s", "open, no highlight, go to line, substitute"},
		{"ctrl+s", "save"},
		{"ctrl+g", "switch to conventional"},
		{"esc", "normal mode, then view mode"},
	}
	treeHelp = []helpKey{
		{"", "Sidebar"},
		{"↑, ↓", "move"},
		{"→, enter", "expand a directory, open a file"},
		{"←", "collapse, or go to the parent"},
		{"/", "filter by name"},
		{"ctrl+n", "new file"},
		{"ctrl+shift+n", "new directory"},
		{"F2", "rename"},
		{"delete", "delete"},
		{"ctrl+h", "show or hide hidden files"},
	}
	globalHelp = []helpKey{
		{"", "Everywhere"},
		{"ctrl+`", "show or hide the sidebar"},
		{"ctrl+w w", "switch the focus (view and vim normal mode)"},
		{"alt+h, alt+l", "focus the sidebar, the document"},
		{"ctrl+g", "switch the edit flavor"},
		{"F1", "this help"},
		{"ctrl+q", "quit"},
	}
)

// helpSections returns the keys of the current focus and mode.
func (m *Model) helpSections() [][]helpKey {
	var first []helpKey
	switch {
	case m.focus == focusTree:
		first = treeHelp
	case m.mode == modeView:
		first = viewHelp
	case m.vimFlavor:
		first = vimHelp
	default:
		first = convHelp
	}
	return [][]helpKey{first, globalHelp}
}

// helpRect is the area of the overlay: in the middle of the screen.
func (m *Model) helpRect() rect {
	n := 0
	for _, s := range m.helpSections() {
		n += len(s) + 1
	}
	w := min(66, max(20, m.width-4))
	h := min(n+2, max(5, m.height-3))
	return rect{max(0, (m.width-w)/2), max(0, (m.height-1-h)/2), w, h}
}

// helpBox draws the overlay.
func (m *Model) helpBox() string {
	r := m.helpRect()
	inner := r.w - 2
	keyW := min(28, inner/2)
	var lines []string
	for _, sec := range m.helpSections() {
		for _, k := range sec {
			if k.key == "" {
				lines = append(lines, " "+theme.St(theme.Muted).Bold(true).Render(k.text))
				continue
			}
			key := theme.St(theme.Fg(theme.Accent)).Render(ui.Trunc(k.key, keyW-1))
			pad := strings.Repeat(" ", max(1, keyW-lipgloss.Width(key)))
			lines = append(lines, " "+key+pad+theme.St(theme.Text).Render(ui.Trunc(k.text, max(1, inner-keyW-2))))
		}
		lines = append(lines, "")
	}
	rows := r.h - 2
	m.help.off = max(0, min(m.help.off, len(lines)-rows))
	lines = lines[m.help.off:]
	return ui.Box(theme.St(theme.Fg(theme.Accent)).Bold(true).Render("Keys"), ui.PadLines(lines, inner, rows), r.w, theme.Fg(theme.Accent))
}
