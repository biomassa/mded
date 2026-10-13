package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/biomassa/mded/internal/config"
	"github.com/biomassa/mded/internal/theme"
	"github.com/biomassa/mded/internal/ui"
)

// The theme picker (T) lists the themes at the right of the screen. A move previews the
// highlighted theme on the whole screen. enter keeps the theme and saves it in the config
// file. esc goes back to the theme that was in use.

// themePicker is the open theme picker.
type themePicker struct {
	names []string
	cur   int
	orig  string // the theme in use when the picker opened
}

const themePickerWidth = 40

// openThemePicker opens the picker with the cursor on the theme in use.
func (m *Model) openThemePicker() {
	names := theme.Names()
	cur := 0
	for i, n := range names {
		if n == theme.Current() {
			cur = i
		}
	}
	m.themes = &themePicker{names: names, cur: cur, orig: theme.Current()}
}

// previewTheme shows the screen in theme name. The markdown cache holds colored text, so
// it is cleared.
func (m *Model) previewTheme(name string) {
	theme.Apply(name, m.termDark)
	m.themeChanged()
}

// themeKey handles a key while the picker is open.
func (m *Model) themeKey(k string) {
	p := m.themes
	switch k {
	case "up", "k":
		p.cur = (p.cur - 1 + len(p.names)) % len(p.names)
	case "down", "j":
		p.cur = (p.cur + 1) % len(p.names)
	case "home", "g":
		p.cur = 0
	case "end", "G":
		p.cur = len(p.names) - 1
	case "enter":
		m.keepTheme()
		return
	case "esc", "T", "q":
		m.themes = nil
		m.previewTheme(p.orig)
		m.status("theme "+p.orig, false)
		return
	default:
		return
	}
	m.previewTheme(p.names[p.cur])
}

// keepTheme closes the picker with the highlighted theme and saves it for the next start.
func (m *Model) keepTheme() {
	name := m.themes.names[m.themes.cur]
	m.themes = nil
	m.previewTheme(name)
	save := m.opts.SaveTheme
	if save == nil {
		save = config.SaveTheme
	}
	m.cmd(func() tea.Msg { return themeSavedMsg{name: name, err: save(name)} })
}

// themeRect is the area of the picker: at the top right, so that most of the screen
// shows the preview.
func (m *Model) themeRect() rect {
	w := min(themePickerWidth, max(20, m.width-4))
	h := min(len(theme.Names())+5, m.height-2) // borders, a space, the list, a space, the hint
	return rect{max(0, m.width-w-1), 1, w, h}
}

// themeBox draws the picker.
func (m *Model) themeBox() string {
	p := m.themes
	r := m.themeRect()
	inner := r.w - 2
	rows := r.h - 5
	off := max(0, min(p.cur-rows/2, len(p.names)-rows)) // keep the cursor in view
	lines := []string{""}
	for i := off; i < len(p.names) && i < off+rows; i++ {
		label := ui.Trunc(theme.Label(p.names[i]), inner-4)
		if i == p.cur {
			line := lipgloss.NewStyle().Background(theme.C(theme.SelBg)).Foreground(theme.C(theme.Fg(theme.Accent))).Bold(true).
				Render(" ▸ " + label + strings.Repeat(" ", max(0, inner-3-lipgloss.Width(label))))
			lines = append(lines, line)
			continue
		}
		lines = append(lines, "   "+theme.St(theme.Text).Render(label))
	}
	lines = append(lines, "", " "+theme.St(theme.Dim).Render(ui.Trunc("↑/↓ preview · enter keep · esc back", inner-1)))
	return ui.Box(theme.St(theme.Fg(theme.Accent)).Bold(true).Render("Theme"), ui.PadLines(lines, inner, r.h-2), r.w, theme.Fg(theme.Accent))
}

// themeClick previews the theme under the mouse. A double-click keeps it. A click
// outside the picker goes back to the theme that was in use.
func (m *Model) themeClick(x, y int, dbl bool) {
	p := m.themes
	r := m.themeRect()
	if !r.has(x, y) {
		m.themes = nil
		m.previewTheme(p.orig)
		return
	}
	rows := r.h - 5
	off := max(0, min(p.cur-rows/2, len(p.names)-rows))
	i := off + y - r.y - 2 // the border and the empty line
	if i < off || i >= len(p.names) || i >= off+rows {
		return
	}
	p.cur = i
	if dbl {
		m.keepTheme()
		return
	}
	m.previewTheme(p.names[i])
}
