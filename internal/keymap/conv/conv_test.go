package conv

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/biomassa/mded/internal/buffer"
	"github.com/biomassa/mded/internal/keymap"
)

// send runs keys on a buffer. A key with one rune that is not a name below types text.
func send(c *Conv, b *buffer.Buffer, keys ...string) []keymap.Effect {
	var all []keymap.Effect
	for _, name := range keys {
		k := keymap.Key{Name: name}
		switch {
		case name == "space":
			k.Text = " "
		case len([]rune(name)) == 1:
			k.Text = name
		}
		all = append(all, c.Handle(b, k, 40, 10)...)
	}
	return all
}

func has(effects []keymap.Effect, k keymap.Kind) bool { return keymap.Has(effects, k) }

func TestTyping(t *testing.T) {
	c, b := New(4), buffer.New("")
	eff := send(c, b, "h", "i", "space", "ż")
	if b.Text() != "hi ż" || !has(eff, keymap.KindChanged) {
		t.Errorf("text %q effects %v", b.Text(), eff)
	}
	if b.UndoDepth() != 1 { // typing is one undo step
		t.Errorf("undo steps = %d", b.UndoDepth())
	}
	send(c, b, "ctrl+z")
	if b.Text() != "" {
		t.Errorf("undo: %q", b.Text())
	}
	send(c, b, "ctrl+y")
	if b.Text() != "hi ż" {
		t.Errorf("redo: %q", b.Text())
	}
	// A move ends the typing group.
	send(c, b, "left", "x")
	if b.UndoDepth() != 2 {
		t.Errorf("steps after move = %d", b.UndoDepth())
	}
	// Alt and ctrl keys do not type, even with text.
	before := b.Text()
	c.Handle(b, keymap.Key{Name: "alt+x", Text: "x"}, 40, 10)
	if b.Text() != before {
		t.Error("alt+x typed text")
	}
}

func TestKeys(t *testing.T) {
	tests := []struct {
		name string
		text string
		row  int
		col  int
		keys []string
		want string
		wr   int
		wc   int
	}{
		{"tab", "ab", 0, 1, []string{"tab"}, "a    b", 0, 5},
		{"enter list", "- a", 0, 3, []string{"enter", "b"}, "- a\n- b", 1, 3},
		{"enter ends list", "- a", 0, 3, []string{"enter", "enter"}, "- a\n", 1, 0},
		{"backspace", "abc", 0, 3, []string{"backspace"}, "ab", 0, 2},
		{"delete", "abc", 0, 0, []string{"delete"}, "bc", 0, 0},
		{"alt+backspace", "foo bar", 0, 7, []string{"alt+backspace"}, "foo ", 0, 4},
		{"ctrl+w is not bound", "foo bar", 0, 7, []string{"ctrl+w"}, "foo bar", 0, 7},
		{"home end", "abc", 0, 1, []string{"end", "home"}, "abc", 0, 0},
		{"word moves", "foo bar baz", 0, 0, []string{"alt+right", "ctrl+right", "alt+left"}, "foo bar baz", 0, 4},
		{"bold", "a word", 0, 3, []string{"ctrl+b"}, "a **word**", 0, 10},
		{"italic", "a word", 0, 3, []string{"ctrl+i"}, "a *word*", 0, 8},
		{"link", "a word", 0, 3, []string{"ctrl+k"}, "a [word]()", 0, 9},
		{"checkbox", "item", 0, 0, []string{"ctrl+t"}, "- [ ] item", 0, 6},
		{"up down keep column", "abcdef\nab\nabcdef", 0, 5, []string{"down", "down"}, "abcdef\nab\nabcdef", 2, 5},
		{"up", "ab\ncd", 1, 1, []string{"up"}, "ab\ncd", 0, 1},
		{"pgdown", "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n13\n14\n15\n16\n17\n18\n19\n20", 0, 0, []string{"pgdown"}, "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n13\n14\n15\n16\n17\n18\n19\n20", 9, 0},
		{"pgup", "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n13\n14\n15\n16\n17\n18\n19\n20", 19, 0, []string{"pgup"}, "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n13\n14\n15\n16\n17\n18\n19\n20", 10, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, b := New(4), buffer.New(tc.text)
			b.Row, b.Col = tc.row, tc.col
			send(c, b, tc.keys...)
			if b.Text() != tc.want || b.Row != tc.wr || b.Col != tc.wc {
				t.Errorf("got %q at %d,%d, want %q at %d,%d", b.Text(), b.Row, b.Col, tc.want, tc.wr, tc.wc)
			}
		})
	}
}

func TestTabWidth(t *testing.T) {
	b := buffer.New("")
	send(New(2), b, "tab")
	if b.Text() != "  " {
		t.Errorf("tab = %q", b.Text())
	}
}

func TestSelection(t *testing.T) {
	c, b := New(4), buffer.New("hello world")
	send(c, b, "shift+right", "shift+right", "shift+right")
	if b.SelectedText() != "hel" {
		t.Fatalf("selected %q", b.SelectedText())
	}
	send(c, b, "right") // a move without shift removes the selection
	if b.SelOn || b.Col != 4 {
		t.Errorf("selection after move: %v col %d", b.SelOn, b.Col)
	}
	send(c, b, "ctrl+a")
	if b.SelectedText() != "hello world" {
		t.Errorf("ctrl+a: %q", b.SelectedText())
	}
	send(c, b, "x") // typing replaces the selection
	if b.Text() != "x" || b.SelOn {
		t.Errorf("replace: %q", b.Text())
	}
	send(c, b, "ctrl+z")
	if b.Text() != "hello world" {
		t.Errorf("undo of replace: %q", b.Text())
	}

	b = buffer.New("one two three")
	b.Col = 13
	send(c, b, "alt+shift+left")
	if b.SelectedText() != "three" {
		t.Errorf("word select: %q", b.SelectedText())
	}
	send(c, b, "ctrl+shift+left")
	if b.SelectedText() != "two three" {
		t.Errorf("ctrl+shift: %q", b.SelectedText())
	}
	send(c, b, "backspace")
	if b.Text() != "one " {
		t.Errorf("backspace on selection: %q", b.Text())
	}

	b = buffer.New("ab\ncd")
	send(c, b, "shift+down", "shift+end")
	if b.SelectedText() != "ab\ncd" {
		t.Errorf("shift+down: %q", b.SelectedText())
	}
	send(c, b, "enter")
	if b.Text() != "\n" || b.Row != 1 {
		t.Errorf("enter on selection: %q", b.Text())
	}
	b = buffer.New("abc")
	send(c, b, "shift+end", "alt+backspace")
	if b.Text() != "" {
		t.Errorf("alt+backspace on selection: %q", b.Text())
	}
	b = buffer.New("abc")
	send(c, b, "shift+end", "ctrl+b")
	if b.Text() != "**abc**" || b.SelOn {
		t.Errorf("bold with selection: %q", b.Text())
	}
}

func TestClipboard(t *testing.T) {
	c, b := New(4), buffer.New("hello world")
	eff := send(c, b, "ctrl+c")
	if len(eff) != 1 || eff[0].Kind != keymap.KindStatus || eff[0].Text != noSelection {
		t.Errorf("copy without selection: %v", eff)
	}
	if eff := send(c, b, "ctrl+x"); len(eff) != 1 || b.Text() != "hello world" {
		t.Errorf("cut without selection: %v", eff)
	}
	send(c, b, "shift+right", "shift+right")
	eff = send(c, b, "ctrl+c")
	if len(eff) != 2 || eff[0].Kind != keymap.KindSetClipboard || eff[0].Text != "he" ||
		eff[1].Text != "copied" || b.Text() != "hello world" {
		t.Errorf("copy: %v", eff)
	}
	eff = send(c, b, "ctrl+x")
	if eff[0].Text != "he" || eff[1].Text != "cut" || b.Text() != "llo world" || !has(eff, keymap.KindChanged) {
		t.Errorf("cut: %v %q", eff, b.Text())
	}
	send(c, b, "ctrl+z")
	if b.Text() != "hello world" {
		t.Errorf("undo cut: %q", b.Text())
	}
	if eff := send(c, b, "ctrl+v"); len(eff) != 1 || eff[0].Kind != keymap.KindReadClipboard {
		t.Errorf("ctrl+v: %v", eff)
	}
}

func TestPaste(t *testing.T) {
	c, b := New(4), buffer.New("hello")
	b.Col = 5
	eff := c.Paste(b, " a\r\nb")
	if b.Text() != "hello a\nb" || b.Row != 1 || b.Col != 1 || !has(eff, keymap.KindChanged) {
		t.Errorf("paste: %q %d,%d", b.Text(), b.Row, b.Col)
	}
	b.SelectAll()
	c.Paste(b, "X")
	if b.Text() != "X" {
		t.Errorf("paste over selection: %q", b.Text())
	}
	c.Handle(b, keymap.Key{Name: "ctrl+z"}, 40, 10)
	if b.Text() != "hello a\nb" {
		t.Errorf("undo paste: %q", b.Text())
	}
	if eff := c.Paste(b, ""); len(eff) != 0 {
		t.Errorf("empty paste: %v", eff)
	}
}

func TestEffects(t *testing.T) {
	c, b := New(4), buffer.New("a")
	for name, kind := range map[string]keymap.Kind{
		"ctrl+s": keymap.KindSave,
		"esc":    keymap.KindExitToView,
		"ctrl+g": keymap.KindSwitchFlavor,
		"ctrl+f": keymap.KindOpenFindBox,
	} {
		eff := send(c, b, name)
		if len(eff) != 1 || eff[0].Kind != kind {
			t.Errorf("%s: %v", name, eff)
		}
	}
	if c.ModeLabel() != "EDIT" || c.Hints() != "esc view · ctrl+s save · ctrl+f find · ctrl+g vim" {
		t.Error("label or hints")
	}
	if _, _, active := c.StatusLine(); active {
		t.Error("status line prompt")
	}
	c.Reset()
}

func TestSearchRefreshOnEdit(t *testing.T) {
	c, b := New(4), buffer.New("ab")
	b.SetSearch("ab", false)
	b.Col = 2
	send(c, b, "space", "a", "b")
	if len(b.Search.Matches) != 2 {
		t.Errorf("matches = %d", len(b.Search.Matches))
	}
}

func key(s string) tea.KeyPressMsg {
	switch {
	case s == "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case s == "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case s == "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case s == "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case s == "shift+enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift}
	case s == "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case strings.HasPrefix(s, "ctrl+"):
		return tea.KeyPressMsg{Code: rune(s[5]), Mod: tea.ModCtrl}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

func typeInto(f *FindBox, b *buffer.Buffer, keys ...string) (effects []keymap.Effect, closed bool) {
	for _, k := range keys {
		msg := key(k)
		_, e, c := f.HandleKey(b, msg)
		effects = append(effects, e...)
		closed = closed || c
	}
	return effects, closed
}

func TestFindBox(t *testing.T) {
	b := buffer.New("foo Foo\nbar foo")
	f := NewFindBox()
	f.Open(b, 60)
	typeInto(f, b, "f", "o", "o")
	if len(f.Matches) != 3 || f.Cur != 0 || b.Row != 0 || b.Col != 0 {
		t.Fatalf("matches %d cur %d at %d,%d", len(f.Matches), f.Cur, b.Row, b.Col)
	}
	typeInto(f, b, "enter")
	if f.Cur != 1 || b.Col != 4 {
		t.Errorf("enter: cur %d col %d", f.Cur, b.Col)
	}
	typeInto(f, b, "enter", "enter") // wraps around
	if f.Cur != 0 {
		t.Errorf("wrap: cur %d", f.Cur)
	}
	typeInto(f, b, "shift+enter")
	if f.Cur != 2 || b.Row != 1 || b.Col != 4 {
		t.Errorf("shift+enter: cur %d at %d,%d", f.Cur, b.Row, b.Col)
	}

	// Tab to the case field and toggle it with space.
	typeInto(f, b, "tab", "tab", "space")
	if !f.MatchCase || len(f.Matches) != 2 || f.Focus != FieldCase {
		t.Errorf("match case: %v %d", f.MatchCase, len(f.Matches))
	}
	typeInto(f, b, "shift+tab", "shift+tab", "backspace", "backspace", "backspace", "F")
	if len(f.Matches) != 1 || f.Matches[0].A != 4 || f.Matches[0].Row != 0 {
		t.Errorf("new query: %v", f.Matches)
	}

	// Replace the current match, then all.
	typeInto(f, b, "tab", "X", "Y")
	if f.Repl.Value() != "XY" {
		t.Fatalf("replace field = %q", f.Repl.Value())
	}
	eff, closed := typeInto(f, b, "ctrl+r")
	if b.Text() != "foo XYoo\nbar foo" || !has(eff, keymap.KindChanged) || closed {
		t.Errorf("ctrl+r: %q %v", b.Text(), eff)
	}
	if len(f.Matches) != 0 {
		t.Errorf("matches after replace: %v", f.Matches)
	}
	typeInto(f, b, "shift+tab")
	typeInto(f, b, "backspace")
	typeInto(f, b, "o") // query "o", match case: three in "foo XYoo\nbar foo"
	if len(f.Matches) != 6 {
		t.Fatalf("matches for o = %d", len(f.Matches))
	}
	eff, closed = typeInto(f, b, "ctrl+a")
	if b.Text() != "fXYXY XYXYXY\nbar fXYXY" || !closed || !has(eff, keymap.KindChanged) {
		t.Errorf("ctrl+a: %q closed %v", b.Text(), closed)
	}
	if b.UndoDepth() != 2 {
		t.Errorf("undo steps = %d", b.UndoDepth())
	}
	b.Undo()
	if b.Text() != "foo XYoo\nbar foo" {
		t.Errorf("undo replace all: %q", b.Text())
	}
}

func TestFindBoxEmptyReplaceRemoves(t *testing.T) {
	b := buffer.New("a-b-c")
	f := NewFindBox()
	f.Open(b, 60)
	typeInto(f, b, "-")
	_, closed := typeInto(f, b, "ctrl+a")
	if b.Text() != "abc" || !closed {
		t.Errorf("%q closed %v", b.Text(), closed)
	}
}

func TestFindBoxEscAndNoMatch(t *testing.T) {
	b := buffer.New("abc")
	f := NewFindBox()
	f.Open(b, 60)
	typeInto(f, b, "z")
	if f.Cur != -1 || len(f.Matches) != 0 {
		t.Error("match for z")
	}
	if eff, _ := typeInto(f, b, "ctrl+r", "enter"); len(eff) != 0 {
		t.Errorf("effects without match: %v", eff)
	}
	if _, closed := typeInto(f, b, "ctrl+a"); !closed || b.UndoDepth() != 0 {
		t.Error("ctrl+a without match")
	}
	if _, closed := typeInto(f, b, "esc"); !closed {
		t.Error("esc did not close")
	}
}
