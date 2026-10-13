package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/biomassa/mded/internal/keymap"
)

// modeVim is a fake keymap with a Mode method.
type modeVim struct {
	fakeVim
	mode string
}

func (f *modeVim) Mode() string { return f.mode }

func TestCursorConventionalEdit(t *testing.T) {
	p := writeTemp(t, "a.md", "abc\n日本x\n", 0o644)
	m := testModelSize(t, p, 60, 20)
	if v := m.View(); v.Cursor != nil {
		t.Error("view mode must have no cursor")
	}
	press(m, "e")
	v := m.View()
	c := v.Cursor
	if c == nil {
		t.Fatal("no cursor in edit mode")
	}
	dr := m.docRect()
	if c.X != dr.x+3 || c.Y != 1 || c.Shape != tea.CursorUnderline || !c.Blink || c.Color == nil {
		t.Errorf("cursor %+v, want x %d y 1 underline", c, dr.x+3)
	}
	press(m, "down", "right", "right") // after two wide runes: 4 cells
	c = m.View().Cursor
	if c == nil || c.X != dr.x+3+4 || c.Y != 2 {
		t.Errorf("wide runes: cursor %+v, want x %d y 2", c, dr.x+7)
	}
	press(m, "end") // after the last rune
	c = m.View().Cursor
	if c == nil || c.X != dr.x+3+5 {
		t.Errorf("line end: cursor %+v, want x %d", c, dr.x+8)
	}
	press(m, "esc")
	if m.View().Cursor != nil {
		t.Error("back in view mode: the cursor must be hidden")
	}
}

// The cell under the cursor on the drawn screen is the rune at the cursor, also in the first
// column of a line.
func TestCursorOnScreenRune(t *testing.T) {
	p := writeTemp(t, "a.md", "abc\nxyz\n", 0o644)
	m := testModelSize(t, p, 40, 10)
	press(m, "e", "down", "home")
	c := m.View().Cursor
	if c == nil {
		t.Fatal("no cursor")
	}
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	got := []rune(lines[c.Y])
	if c.X >= len(got) || got[c.X] != 'x' {
		t.Errorf("cell under the cursor (%d,%d) in %q is not 'x'", c.X, c.Y, lines[c.Y])
	}
}

func TestCursorWrappedLineAndScroll(t *testing.T) {
	long := strings.Repeat("word ", 30)
	var sb strings.Builder
	for i := 0; i < 40; i++ {
		sb.WriteString("l\n")
	}
	p := writeTemp(t, "a.md", long+"\n"+sb.String(), 0o644)
	m := testModelSize(t, p, 50, 12)
	press(m, "e", "end")
	c := m.View().Cursor
	if c == nil {
		t.Fatal("no cursor")
	}
	cy := -1
	for i, r := range m.rows {
		if r.Line == 0 {
			cy = i
		}
	}
	last := m.rows[cy]
	if last.Start == 0 {
		t.Fatal("the line did not wrap")
	}
	dr := m.docRect()
	want := dr.x + 3 + len(last.Cells) // the border, the margin, the indent
	if c.X != want || c.Y != 1+cy-m.doc.off {
		t.Errorf("cursor %d,%d want %d,%d", c.X, c.Y, want, 1+cy-m.doc.off)
	}
	// A cursor row that is out of the pane gives no cursor.
	m.doc.off = len(m.rows) - 1
	if m.View().Cursor != nil {
		t.Error("a row that is scrolled out must have no cursor")
	}
}

func TestCursorVimShape(t *testing.T) {
	p := writeTemp(t, "a.md", "abc\n", 0o644)
	fv := &modeVim{mode: "normal"}
	m := testModel(t, p)
	m.opts.NewVim = func(int) keymap.Keymap { return fv }
	press(m, "ctrl+g", "e")
	want := map[string]tea.CursorShape{
		"normal": tea.CursorBlock, "visual": tea.CursorBlock, "vline": tea.CursorBlock,
		"insert": tea.CursorUnderline, "replace": tea.CursorUnderline,
	}
	for mode, shape := range want {
		fv.mode = mode
		c := m.View().Cursor
		if c == nil || c.Shape != shape {
			t.Errorf("mode %s: cursor %+v, want shape %v", mode, c, shape)
		}
	}
}

func TestCursorSearchPrompt(t *testing.T) {
	p := writeTemp(t, "a.md", "one\n", 0o644)
	m := testModelSize(t, p, 60, 20)
	press(m, "/")
	typeText(m, "ab")
	c := m.View().Cursor
	if c == nil || c.X != 3 || c.Y != 19 || c.Shape != tea.CursorUnderline {
		t.Errorf("cursor %+v, want 3,19 underline", c)
	}
	press(m, "left")
	if c = m.View().Cursor; c == nil || c.X != 2 {
		t.Errorf("after left: cursor %+v, want x 2", c)
	}
	press(m, "esc")
	if m.View().Cursor != nil {
		t.Error("no cursor after the prompt closed in view mode")
	}
}

func TestCursorFindBox(t *testing.T) {
	p := writeTemp(t, "a.md", "foo bar\n", 0o644)
	m := testModelSize(t, p, 80, 20)
	press(m, "e", "ctrl+f")
	typeText(m, "ab")
	dr := m.docRect()
	pw := len([]rune(" find    › "))
	c := m.View().Cursor
	if c == nil || c.X != dr.x+2+pw+2 || c.Y != 2 || c.Shape != tea.CursorUnderline {
		t.Errorf("find field: cursor %+v, want %d,2", c, dr.x+2+pw+2)
	}
	press(m, "tab")
	typeText(m, "x")
	c = m.View().Cursor
	if c == nil || c.X != dr.x+2+pw+1 || c.Y != 3 {
		t.Errorf("replace field: cursor %+v, want %d,3", c, dr.x+2+pw+1)
	}
	press(m, "tab")
	if m.View().Cursor != nil {
		t.Error("the check box has no text cursor")
	}
}

func TestCursorHiddenInDialogAndPopups(t *testing.T) {
	p := writeTemp(t, "a.md", "abc\n", 0o644)
	m := testModel(t, p)
	press(m, "e")
	if m.View().Cursor == nil {
		t.Fatal("no cursor")
	}
	press(m, "?")
	if m.help != nil && m.View().Cursor != nil {
		t.Error("help must hide the cursor")
	}
}

// In the search prompt and in the find box, the cursor is just after the typed text.
func TestPromptCursorOnScreen(t *testing.T) {
	p := writeTemp(t, "a.md", "abc\n", 0o644)
	m := testModelSize(t, p, 40, 10)
	after := func(name string, want rune) {
		t.Helper()
		c := m.View().Cursor
		if c == nil {
			t.Fatalf("%s: no cursor", name)
		}
		line := []rune(strings.Split(ansi.Strip(m.View().Content), "\n")[c.Y])
		if c.X < 1 || c.X > len(line) || line[c.X-1] != want {
			t.Errorf("%s: cell before the cursor (%d,%d) in %q is not %q", name, c.X, c.Y, string(line), want)
		}
	}
	press(m, "/")
	typeText(m, "bq")
	after("search prompt", 'q')
	press(m, "esc", "e", "ctrl+f")
	typeText(m, "bz")
	after("find box", 'z')
}
