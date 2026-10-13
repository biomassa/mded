package conv

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/biomassa/mded/internal/buffer"
	"github.com/biomassa/mded/internal/keymap"
)

// Fields of the find box, in tab order.
const (
	FieldFind = iota
	FieldReplace
	FieldCase
	fieldCount
)

// FindBox is the state of the find and replace box (ctrl+f). The search is literal. enter
// goes to the next match. ctrl+r and ctrl+a replace with the replace field as it is, so
// that an empty field removes the matches. The UI draws the box from the exported fields.
type FindBox struct {
	Find, Repl textinput.Model
	Focus      int // the field with the focus: FieldFind, FieldReplace, or FieldCase
	MatchCase  bool
	Matches    []buffer.Match
	Cur        int // index of the current match, -1 if there is none
}

func newInput(prompt string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = prompt
	ti.CharLimit = 200
	s := textinput.DefaultStyles(true)
	s.Cursor.Blink = false
	ti.SetStyles(s)
	ti.SetVirtualCursor(false) // the UI shows the real terminal cursor
	return ti
}

// NewFindBox returns a closed find box with empty fields.
func NewFindBox() *FindBox {
	return &FindBox{
		Find: newInput(" find    › "),
		Repl: newInput(" replace › "),
		Cur:  -1,
	}
}

// Open puts the focus on the find field. width is the inner width of the pane. It returns
// the command that starts the cursor of the field.
func (f *FindBox) Open(b *buffer.Buffer, width int) tea.Cmd {
	f.Find.SetWidth(max(8, width-16))
	f.Repl.SetWidth(max(8, width-16))
	f.Focus = FieldFind
	f.Repl.Blur()
	f.refind(b)
	return f.Find.Focus()
}

// refind finds the matches again and goes to the first match at or after the cursor.
func (f *FindBox) refind(b *buffer.Buffer) {
	f.Matches = b.FindLiteral(f.Find.Value(), f.MatchCase)
	f.Cur = -1
	for i, m := range f.Matches {
		if m.Row > b.Row || (m.Row == b.Row && m.A >= b.Col) {
			f.Cur = i
			break
		}
	}
	if f.Cur < 0 && len(f.Matches) > 0 {
		f.Cur = 0
	}
	f.gotoMatch(b)
}

// gotoMatch puts the cursor at the start of the current match.
func (f *FindBox) gotoMatch(b *buffer.Buffer) {
	if f.Cur >= 0 && f.Cur < len(f.Matches) {
		m := f.Matches[f.Cur]
		b.MoveTo(m.Row, m.A)
	}
}

// step moves to the next (d = 1) or previous (d = -1) match, with wrap-around.
func (f *FindBox) step(b *buffer.Buffer, d int) {
	if n := len(f.Matches); n > 0 {
		f.Cur = (f.Cur + d + n) % n
		f.gotoMatch(b)
	}
}

// Refresh finds the matches again after the text changed outside the box.
func (f *FindBox) Refresh(b *buffer.Buffer) { f.Matches = b.FindLiteral(f.Find.Value(), f.MatchCase) }

// replaceCurrent replaces the current match and goes to the next match.
func (f *FindBox) replaceCurrent(b *buffer.Buffer) {
	if f.Cur < 0 || f.Cur >= len(f.Matches) {
		return
	}
	b.PushUndo(false)
	b.ReplaceMatch(f.Matches[f.Cur], f.Repl.Value())
	f.refind(b) // the cursor is after the new text, so the search goes on from there
}

// HandleKey processes a key while the box is open. closed is true if the box closes. The
// box takes all keys. The effects hold Changed after a replace.
func (f *FindBox) HandleKey(b *buffer.Buffer, msg tea.KeyPressMsg) (cmd tea.Cmd, effects []keymap.Effect, closed bool) {
	rev := b.Rev
	defer func() {
		if b.Rev != rev {
			b.Refresh()
			effects = append(effects, keymap.Changed())
		}
	}()
	key := msg.String()
	switch key {
	case "esc":
		return nil, nil, true // the cursor stays at the current match
	case "tab", "shift+tab":
		d := 1
		if key == "shift+tab" {
			d = -1
		}
		f.Focus = (f.Focus + d + fieldCount) % fieldCount
		f.Find.Blur()
		f.Repl.Blur()
		switch f.Focus {
		case FieldFind:
			return f.Find.Focus(), nil, false
		case FieldReplace:
			return f.Repl.Focus(), nil, false
		}
		return nil, nil, false
	case "shift+enter":
		f.step(b, -1)
		return nil, nil, false
	case "enter":
		f.step(b, 1)
		return nil, nil, false
	case "ctrl+r":
		f.replaceCurrent(b)
		return nil, nil, false
	case "ctrl+a":
		if len(b.FindLiteral(f.Find.Value(), f.MatchCase)) > 0 {
			b.PushUndo(false)
			b.ReplaceAll(f.Find.Value(), f.Repl.Value(), f.MatchCase)
		}
		return nil, nil, true
	case "space":
		if f.Focus == FieldCase {
			f.MatchCase = !f.MatchCase
			f.refind(b)
			return nil, nil, false
		}
	}
	switch f.Focus {
	case FieldFind:
		before := f.Find.Value()
		f.Find, cmd = f.Find.Update(msg)
		if f.Find.Value() != before {
			f.refind(b)
		}
	case FieldReplace:
		f.Repl, cmd = f.Repl.Update(msg)
	}
	return cmd, nil, false
}
