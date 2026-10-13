package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/biomassa/mded/internal/markdown"
)

// viewKey handles a key of view mode in the document pane.
func (m *Model) viewKey(msg tea.KeyPressMsg) {
	d := m.doc
	k := msg.String()
	th := m.textH()
	if d.image { // the image viewer has only the keys that do not need text
		switch k {
		case "q":
			m.askQuit()
		case "?":
			m.openHelp()
		case "T":
			m.openThemePicker()
		case "backspace", "ctrl+o":
			m.back()
		}
		return
	}
	switch k {
	case "j", "down":
		d.off++
	case "k", "up":
		d.off--
	case "ctrl+d":
		d.off += max(1, th/2)
	case "ctrl+u":
		d.off -= max(1, th/2)
	case "space":
		if it, ok := m.selectedItem(); ok && it.Kind == markdown.Checkbox {
			m.toggleItem(it)
			return
		}
		d.off += max(1, th-1)
	case "pgdown":
		d.off += max(1, th-1)
	case "b", "pgup":
		d.off -= max(1, th-1)
	case "g", "home":
		d.off = 0
	case "G", "end":
		d.off = len(m.rows)
	case "/":
		m.startSearch()
	case "n", "N":
		m.searchNext(k == "N")
	case "tab":
		m.moveItem(1)
	case "shift+tab":
		m.moveItem(-1)
	case "enter":
		// A selected item (tab) comes first. Else, while the search highlights are on,
		// enter edits the document at the current match.
		if it, ok := m.selectedItem(); ok {
			if it.Kind == markdown.Link {
				m.followLink(it.URL)
			}
		} else if s := &m.doc.buf.Search; s.Highlight && s.Cur >= 0 && s.Cur < len(s.Matches) {
			mt := s.Matches[s.Cur]
			m.enterEdit(&[2]int{mt.Row, mt.A})
		}
	case "backspace", "ctrl+o":
		m.back()
	case "e", "i":
		m.enterEdit(nil)
	case "T":
		m.openThemePicker()
	case "?":
		m.openHelp()
	case "q":
		m.askQuit()
	case "ctrl+s":
		m.save()
	case "esc":
		switch {
		case d.buf.Search.Highlight:
			d.buf.ClearHighlight()
		default:
			m.itemSel = -1
		}
	}
}

// selectedItem returns the item that the tab key selected.
func (m *Model) selectedItem() (markdown.Item, bool) {
	items := m.getItems()
	if m.itemSel < 0 || m.itemSel >= len(items) {
		return markdown.Item{}, false
	}
	return items[m.itemSel], true
}

// moveItem selects the next (d = 1) or the previous (d = -1) item, with wrap-around. With
// no selection, it starts at the first item below the top of the pane, or the last item
// above the bottom of the pane.
func (m *Model) moveItem(dir int) {
	items := m.getItems()
	if len(items) == 0 {
		return
	}
	next := 0
	switch {
	case m.itemSel >= 0 && m.itemSel < len(items):
		next = m.itemSel + dir
	case dir > 0:
		next = len(items) // none found yet: wrap to the first
		top := m.topLine()
		for i, it := range items {
			if it.Line >= top {
				next = i
				break
			}
		}
	default:
		next = -1
		bottom := m.bottomLine()
		for i, it := range items {
			if it.Line <= bottom {
				next = i
			}
		}
	}
	next = (next%len(items) + len(items)) % len(items)
	m.itemSel = next
	m.reveal = revealReq{line: items[next].Line, active: true}
}

// bottomLine returns the source line at the bottom of the pane.
func (m *Model) bottomLine() int {
	if len(m.rows) == 0 {
		return 0
	}
	return m.rows[min(len(m.rows)-1, m.doc.off+m.textH()-1)].Line
}

// toggleItem changes a checkbox. The document becomes modified, and edit mode can undo it.
func (m *Model) toggleItem(it markdown.Item) {
	b := m.doc.buf
	b.PushUndo(false)
	if b.ToggleCheckboxAt(it.Line) {
		b.Refresh()
	}
}
