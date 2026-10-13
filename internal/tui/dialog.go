package tui

import (
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/biomassa/mded/internal/buffer"
	"github.com/biomassa/mded/internal/keymap"
)

// dialog is the question about unsaved changes. It shows in the status line. then runs
// after the user saves or discards. esc cancels.
type dialog struct {
	then func()
}

// text returns the status line text of the dialog, in two parts: the question and the keys.
func (m *Model) dialogText() (question, keys string) {
	return "Save changes to " + m.doc.title() + "?", "s save · d discard · esc cancel"
}

// dialogKey handles a key while the dialog is open. Other keys do nothing.
func (m *Model) dialogKey(k string) {
	dlg := m.dlg
	switch k {
	case "s":
		if m.save() {
			m.dlg = nil
			dlg.then()
		} else {
			m.dlg = nil
		}
	case "d":
		m.dlg = nil
		d := m.doc
		d.buf.SetText(d.buf.Saved) // the discard also matters for a document in the link history
		dlg.then()
	case "esc":
		m.dlg = nil
	}
}

// searchPrompt is the search prompt of view mode (/). It holds the text with its cursor
// and what esc needs to restore.
type searchPrompt struct {
	text    []rune
	cur     int
	prev    buffer.Search // the search before the prompt
	row     int           // the start position of the search
	col     int
	off     int // the scroll position before the prompt
	hist    int // position in the history, len(history) for the new text
	draft   string
	note    string // problem with the pattern, in the status line
	wrapped bool   // the last jump wrapped around the document
}

// insert puts text at the cursor of the prompt.
func (s *searchPrompt) insert(text string) {
	text = oneLine(text)
	rs := []rune(text)
	s.text = append(s.text[:s.cur:s.cur], append(rs, s.text[s.cur:]...)...)
	s.cur += len(rs)
}

// line returns the prompt text for the status line, with the position of the cursor.
func (s *searchPrompt) line() (string, int) { return "/" + string(s.text), 1 + s.cur }

// startSearch opens the prompt. The search starts at the line at the top of the pane.
func (m *Model) startSearch() {
	d := m.doc
	b := d.buf
	m.reveal.active = false
	top := m.topLine()
	b.MoveTo(top, 0)
	m.srchPos = true
	m.search = &searchPrompt{prev: b.Search, row: top, col: 0, off: d.off, hist: len(m.searchHist)}
}

// searchKey handles a key in the search prompt.
func (m *Model) searchKey(msg tea.KeyPressMsg) {
	s := m.search
	k := msg.String()
	changed := false
	switch k {
	case "esc":
		d := m.doc
		d.buf.Search = s.prev
		d.buf.MoveTo(s.row, s.col)
		d.off = s.off
		m.search = nil
		return
	case "enter":
		m.searchEnter()
		return
	case "left":
		s.cur = max(0, s.cur-1)
	case "right":
		s.cur = min(len(s.text), s.cur+1)
	case "home", "ctrl+a":
		s.cur = 0
	case "end", "ctrl+e":
		s.cur = len(s.text)
	case "backspace":
		if s.cur > 0 {
			s.text = append(s.text[:s.cur-1], s.text[s.cur:]...)
			s.cur--
			changed = true
		}
	case "delete":
		if s.cur < len(s.text) {
			s.text = append(s.text[:s.cur], s.text[s.cur+1:]...)
			changed = true
		}
	case "ctrl+u":
		s.text, s.cur = s.text[s.cur:], 0
		changed = true
	case "up":
		if s.hist > 0 {
			if s.hist == len(m.searchHist) {
				s.draft = string(s.text)
			}
			s.hist--
			s.text = []rune(m.searchHist[s.hist])
			s.cur = len(s.text)
			changed = true
		}
	case "down":
		if s.hist < len(m.searchHist) {
			s.hist++
			if s.hist == len(m.searchHist) {
				s.text = []rune(s.draft)
			} else {
				s.text = []rune(m.searchHist[s.hist])
			}
			s.cur = len(s.text)
			changed = true
		}
	default:
		if keymap.IsTyped(keymap.Key{Name: k, Text: msg.Text}) && utf8.ValidString(msg.Text) {
			s.insert(msg.Text)
			changed = true
		}
	}
	if changed {
		m.searchUpdate()
	}
}

// searchUpdate runs the search again for the text of the prompt: it highlights all
// matches and jumps to the first one from the start position.
func (m *Model) searchUpdate() {
	s := m.search
	b := m.doc.buf
	text := string(s.text)
	b.MoveTo(s.row, s.col)
	s.note, s.wrapped = "", false
	if text == "" {
		b.SetSearch("", false)
		return
	}
	if err := b.SetSearch(text, false); err != nil {
		s.note = "bad pattern: " + err.Error()
		return
	}
	wrapped, ok := b.SearchAt(s.row, s.col)
	if !ok {
		s.note = "pattern not found: " + text
		return
	}
	s.wrapped = wrapped
	m.reveal = revealReq{line: b.Row, active: true}
}

// searchEnter keeps the search and closes the prompt.
func (m *Model) searchEnter() {
	s := m.search
	m.search = nil
	b := m.doc.buf
	text := string(s.text)
	if text == "" { // an empty prompt changes nothing
		b.Search = s.prev
		b.MoveTo(s.row, s.col)
		return
	}
	if n := len(m.searchHist); n == 0 || m.searchHist[n-1] != text {
		m.searchHist = append(m.searchHist, text)
	}
	switch {
	case strings.HasPrefix(s.note, "bad pattern"):
		m.status(s.note, true)
	case s.note != "":
		m.status(s.note, true)
	case s.wrapped:
		m.status("search hit BOTTOM, continuing at TOP", false)
	}
}

// searchNext jumps to the next match (n) or the previous one (N).
func (m *Model) searchNext(reverse bool) {
	b := m.doc.buf
	if b.Search.Re == nil {
		m.status("no search pattern", true)
		return
	}
	if !m.srchPos {
		b.MoveTo(m.topLine(), 0)
	}
	m.srchPos = true
	wrapped, ok := b.SearchJump(reverse)
	if !ok {
		m.status("pattern not found: "+b.Search.Pattern, true)
		return
	}
	m.reveal = revealReq{line: b.Row, active: true}
	if wrapped {
		if b.Search.Backward != reverse {
			m.status("search hit TOP, continuing at BOTTOM", false)
		} else {
			m.status("search hit BOTTOM, continuing at TOP", false)
		}
	}
}

// hasScheme reports whether a link target is a URL with a scheme, such as https://x or
// mailto:a@b. A single letter before the colon is a drive letter, not a scheme.
func hasScheme(s string) bool {
	u, err := url.Parse(s)
	return err == nil && len(u.Scheme) > 1
}

// splitAnchor splits "file.md#heading" into the file and the anchor.
func splitAnchor(s string) (file, anchor string) {
	file, anchor, _ = strings.Cut(s, "#")
	if a, err := url.PathUnescape(anchor); err == nil {
		anchor = a
	}
	return file, anchor
}

// unescape decodes %20 and the like in a link path. A bad escape keeps the text.
func unescape(s string) string {
	if u, err := url.PathUnescape(s); err == nil {
		return u
	}
	return s
}

// openURL opens a URL with the desktop opener. The opener runs detached.
func openURL(u string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	c := exec.Command(name, u)
	if err := c.Start(); err != nil {
		return err
	}
	go c.Wait() // reap the process
	return nil
}
