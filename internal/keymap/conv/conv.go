// Package conv is the conventional edit keymap: arrows, shift selection, and ctrl keys, as
// in a common text editor. It runs over a buffer.Buffer.
package conv

import (
	"strings"

	"github.com/biomassa/mded/internal/buffer"
	"github.com/biomassa/mded/internal/keymap"
)

// hints is the hint text of edit mode.
const hints = "esc view · ctrl+s save · ctrl+f find · ctrl+g vim"

// noSelection is the message for ctrl+c and ctrl+x without a selection.
const noSelection = "select text first: shift+arrows, ctrl+a, or a mouse drag"

// Conv is the conventional keymap. It keeps no state between keys.
type Conv struct {
	tabWidth int // spaces that tab inserts
}

var _ keymap.Keymap = (*Conv)(nil)

// New returns a conventional keymap. tab inserts tabWidth spaces.
func New(tabWidth int) *Conv { return &Conv{tabWidth: max(1, tabWidth)} }

// ModeLabel returns the label of the status line.
func (c *Conv) ModeLabel() string { return "EDIT" }

// Hints returns the key hints of edit mode.
func (c *Conv) Hints() string { return hints }

// StatusLine returns no prompt: conventional mode has none.
func (c *Conv) StatusLine() (string, int, bool) { return "", 0, false }

// Reset does nothing: the keymap has no state.
func (c *Conv) Reset() {}

// Handle processes one key.
func (c *Conv) Handle(b *buffer.Buffer, k keymap.Key, width, pageRows int) []keymap.Effect {
	rev := b.Rev
	effects := c.handle(b, k, width, pageRows)
	return finish(b, rev, effects)
}

// Paste inserts text and replaces the selection.
func (c *Conv) Paste(b *buffer.Buffer, text string) []keymap.Effect {
	rev := b.Rev
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if text != "" {
		if !b.ReplaceSelection() {
			b.PushUndo(false)
		}
		b.InsertText(text)
	}
	return finish(b, rev, nil)
}

// finish keeps the search matches current and adds the Changed effect if the text changed.
func finish(b *buffer.Buffer, rev int, effects []keymap.Effect) []keymap.Effect {
	if b.Rev != rev {
		b.Refresh()
		effects = append(effects, keymap.Changed())
	}
	return effects
}

// move runs a cursor move. With extend, it makes or grows the selection. Without it, the
// move removes the selection.
func move(b *buffer.Buffer, extend bool, f func()) {
	if extend {
		b.StartSel()
	} else {
		b.ClearSel()
	}
	f()
	b.BreakTyping()
}

func (c *Conv) handle(b *buffer.Buffer, k keymap.Key, w, pageRows int) []keymap.Effect {
	b.Clamp()
	page := max(1, pageRows-1) // one screen; one line stays visible
	vertical := func(d int) func() {
		return func() { b.MoveVertical(d, w) }
	}
	pages := func(d int) func() {
		return func() {
			for i := 0; i < page; i++ {
				b.MoveVertical(d, w)
			}
		}
	}
	keepWant := false // up and down keep the wanted column. Other keys reset it.
	defer func() {
		if !keepWant {
			b.Want = -1
		}
	}()
	switch k.Name {
	case "ctrl+g":
		return []keymap.Effect{keymap.SwitchFlavor()}
	case "ctrl+s":
		return []keymap.Effect{keymap.Save()}
	case "ctrl+f":
		return []keymap.Effect{keymap.OpenFindBox()}
	case "esc":
		b.ClearSel()
		return []keymap.Effect{keymap.ExitToView()}
	case "ctrl+z":
		b.Undo()
	case "ctrl+y":
		b.Redo()

	// Moves, with and without shift.
	case "left":
		move(b, false, b.MoveLeft)
	case "right":
		move(b, false, b.MoveRight)
	case "up":
		keepWant = true
		move(b, false, vertical(-1))
	case "down":
		keepWant = true
		move(b, false, vertical(1))
	case "pgup":
		keepWant = true
		move(b, false, pages(-1))
	case "pgdown":
		keepWant = true
		move(b, false, pages(1))
	case "home":
		move(b, false, b.Home)
	case "end":
		move(b, false, b.End)
	case "alt+left", "ctrl+left":
		move(b, false, b.WordLeft)
	case "alt+right", "ctrl+right":
		move(b, false, b.WordRight)
	case "shift+left":
		move(b, true, b.MoveLeft)
	case "shift+right":
		move(b, true, b.MoveRight)
	case "shift+up":
		keepWant = true
		move(b, true, vertical(-1))
	case "shift+down":
		keepWant = true
		move(b, true, vertical(1))
	case "shift+home":
		move(b, true, b.Home)
	case "shift+end":
		move(b, true, b.End)
	case "alt+shift+left", "ctrl+shift+left":
		move(b, true, b.WordLeft)
	case "alt+shift+right", "ctrl+shift+right":
		move(b, true, b.WordRight)
	case "ctrl+a":
		b.SelectAll()
		b.BreakTyping()

	// Clipboard.
	case "ctrl+c", "ctrl+x":
		text := b.SelectedText()
		if text == "" {
			return []keymap.Effect{keymap.Status(noSelection, false)}
		}
		if k.Name == "ctrl+c" {
			return []keymap.Effect{keymap.SetClipboard(text), keymap.Status("copied", false)}
		}
		b.PushUndo(false)
		b.DeleteSelection()
		return []keymap.Effect{keymap.SetClipboard(text), keymap.Status("cut", false)}
	case "ctrl+v":
		return []keymap.Effect{keymap.ReadClipboard()}

	// Edits. A selection goes first: the edit replaces it.
	case "enter":
		if !b.ReplaceSelection() {
			b.PushUndo(false)
		}
		b.Newline()
	case "tab":
		if !b.ReplaceSelection() {
			b.PushUndo(false)
		}
		b.Insert([]rune(strings.Repeat(" ", c.tabWidth)))
	case "backspace", "delete":
		if b.ReplaceSelection() { // the selection goes, and nothing more
			break
		}
		b.PushUndo(false)
		if k.Name == "delete" {
			b.DeleteForward()
		} else {
			b.Backspace()
		}
	case "alt+backspace":
		if b.ReplaceSelection() {
			break
		}
		b.PushUndo(false)
		b.DeleteWordBack()
	case "ctrl+b":
		c.wrap(b, "**", "**")
	case "ctrl+i":
		c.wrap(b, "*", "*")
	case "ctrl+k":
		c.wrap(b, "[", "]()")
	case "ctrl+t":
		b.ClearSel()
		b.PushUndo(false)
		b.ToggleLineCheckbox()
	default:
		if keymap.IsTyped(k) {
			if !b.ReplaceSelection() {
				b.PushUndo(true)
			}
			b.Insert([]rune(k.Text))
			return nil
		}
		b.BreakTyping()
	}
	return nil
}

// wrap puts markup around the word at the cursor. It removes the selection first.
func (c *Conv) wrap(b *buffer.Buffer, before, after string) {
	b.ClearSel()
	b.PushUndo(false)
	b.WrapWord(before, after)
}
