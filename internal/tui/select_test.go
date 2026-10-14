package tui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

const selDoc = "# Title\n\nSome **bold word** here\n\n- [ ] task one\n- item two\n\n| A | B |\n|---|---|\n| x | y |\n\n```go\nfoo := 1\nbar := 2\n```\n\n> quote\n\n---\n\nlast\n"

func selModel(t *testing.T) *Model {
	t.Helper()
	m := testModelSize(t, writeTemp(t, "a.md", selDoc, 0o644), 60, 40)
	if m.showTree {
		m.showTree = false
		m.Update(tea.WindowSizeMsg{Width: 60, Height: 40})
	}
	return m
}

// screenLines returns the drawn screen without styles, one string per row.
func screenLines(m *Model) []string { return strings.Split(ansi.Strip(m.View().Content), "\n") }

// rowOf returns the screen row of the first row whose text contains s.
func rowOf(t *testing.T, m *Model, s string) int {
	t.Helper()
	for y, l := range screenLines(m) {
		if strings.Contains(l, s) {
			return y
		}
	}
	t.Fatalf("no row with %q in\n%s", s, strings.Join(screenLines(m), "\n"))
	return -1
}

// colOf returns the screen column of the first rune of s in the row.
func colOf(t *testing.T, m *Model, y int, s string) int {
	t.Helper()
	l := screenLines(m)[y]
	i := strings.Index(l, s)
	if i < 0 {
		t.Fatalf("no %q in %q", s, l)
	}
	return len([]rune(l[:i]))
}

// drag presses, moves and releases the left button, and returns the clipboard text that the
// release asked for.
func drag(m *Model, x1, y1, x2, y2 int) string {
	m.Update(tea.MouseClickMsg{X: x1, Y: y1, Button: tea.MouseLeft})
	m.Update(tea.MouseMotionMsg{X: x2, Y: y2, Button: tea.MouseLeft})
	_, cmd := m.Update(tea.MouseReleaseMsg{X: x2, Y: y2, Button: tea.MouseLeft})
	return clipboardOf(cmd)
}

// clipboardOf runs a command and its batches, and returns the text of a clipboard request.
// A command that waits (a timer) is not waited for.
func clipboardOf(cmd tea.Cmd) string {
	if cmd == nil {
		return ""
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	select {
	case msg := <-ch:
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				if s := clipboardOf(c); s != "" {
					return s
				}
			}
			return ""
		}
		if v := reflect.ValueOf(msg); v.Kind() == reflect.String && reflect.TypeOf(msg).Name() == "setClipboardMsg" {
			return v.String()
		}
	case <-time.After(30 * time.Millisecond):
	}
	return ""
}

// The text of a level 1 heading is in the middle of the pane, in view mode. As source (edit
// mode, cursor on it), it is at the left.
func TestLevelOneHeadingCentered(t *testing.T) {
	m := testModelSize(t, writeTemp(t, "c.md", "# Middle\n\ntext\n", 0o644), 60, 20)
	y := rowOf(t, m, "Middle")
	line := []rune(screenLines(m)[y])
	x := colOf(t, m, y, "Middle")
	left, right := x, 0
	for i := len(line) - 2; i >= 0 && line[i] == ' '; i-- { // the border is the last cell
		right++
	}
	if d := left - 1 - right; d < -2 || d > 2 { // the space before and after the text
		t.Errorf("not centered: %d cells before, %d after in %q", left, right, string(line))
	}
	press(m, "e", "ctrl+home")
	m.doc.buf.MoveTo(0, 0)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	y = rowOf(t, m, "# Middle")
	if x := colOf(t, m, y, "# Middle"); x > 4 {
		t.Errorf("source heading at column %d, want the left", x)
	}
}

// A paragraph starts in the same column as the text of a heading, in view mode.
func TestIndentParagraphAlignsWithHeadingText(t *testing.T) {
	// A level 2 heading is left-aligned: its text and a paragraph start in one column.
	m2 := testModelSize(t, writeTemp(t, "b.md", "## Second\n\nSome text\n", 0o644), 60, 20)
	hy2, py2 := rowOf(t, m2, "Second"), rowOf(t, m2, "Some ")
	if hx, px := colOf(t, m2, hy2, "Second"), colOf(t, m2, py2, "Some"); hx != px {
		t.Errorf("level 2 heading text at %d, paragraph at %d", hx, px)
	}
	m := selModel(t)
	py := rowOf(t, m, "Some ")
	px := colOf(t, m, py, "Some")
	// Every other kind of row has the same indent.
	for _, s := range []string{"task one", "item two", "quote", "foo := 1", "last"} {
		y := rowOf(t, m, s)
		if x := colOf(t, m, y, s); x < px {
			t.Errorf("%q at %d, left of %d", s, x, px)
		}
	}
	// The bar keeps its left edge: column 2, after the empty side column and the row margin.
	raw := strings.Split(m.View().Content, "\n")[rowOf(t, m, "Title")]
	if i := strings.Index(raw, "48;2;"); i < 0 || ansi.StringWidth(ansi.Strip(raw[:i])) != 2 {
		t.Errorf("heading bar does not start at column 2: %q", raw)
	}
}

// The cursor and a click map to the cell that is drawn, for a paragraph and a heading line
// that shows as source.
func TestIndentCursorAndClickMatchScreen(t *testing.T) {
	m := selModel(t)
	press(m, "e")
	m.doc.buf.MoveTo(0, 2) // "# Title": the source of a heading
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 40})
	c := m.View().Cursor
	if c == nil {
		t.Fatal("no cursor")
	}
	if got := []rune(screenLines(m)[c.Y])[c.X]; got != 'T' {
		t.Errorf("heading source: cell under the cursor is %q", got)
	}
	m.doc.buf.MoveTo(2, 0)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 40})
	c = m.View().Cursor
	if got := []rune(screenLines(m)[c.Y])[c.X]; got != 'S' {
		t.Errorf("paragraph: cell under the cursor is %q", got)
	}
	// A click on the 'b' of "bold" in the preview line moves the cursor to it.
	y := rowOf(t, m, "task one")
	x := colOf(t, m, y, "one")
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	if b := m.doc.buf; b.Row != 4 || string(b.Lines[4][b.Col:b.Col+3]) != "one" {
		t.Errorf("click: row %d col %d", b.Row, b.Col)
	}
}

func TestSelectCopiesSource(t *testing.T) {
	m := selModel(t)
	y := rowOf(t, m, "Some bold")
	// A partial word inside the bold text: no marks.
	x1 := colOf(t, m, y, "old")
	if got := drag(m, x1, y, x1+2, y); got != "old" {
		t.Errorf("partial: %q", got)
	}
	if m.msg != "copied 3 characters" {
		t.Errorf("status %q", m.msg)
	}
	// A range from before the bold text to after it: the marks are in the copy.
	x2 := colOf(t, m, y, "here")
	x1 = colOf(t, m, y, "Some") + 3
	if got := drag(m, x1, y, x2+1, y); got != "e **bold word** he" {
		t.Errorf("marks: %q", got)
	}
	// From the heading into a list item.
	hy, ly := rowOf(t, m, "Title"), rowOf(t, m, "item two")
	hx := colOf(t, m, hy, "Title")
	got := drag(m, hx+2, hy, colOf(t, m, ly, "item")+1, ly)
	want := "tle\n\nSome **bold word** here\n\n- [ ] task one\n- it"
	if got != want {
		t.Errorf("multi-line: %q want %q", got, want)
	}
	// A start in a code block row: the whole line.
	cy := rowOf(t, m, "foo := 1")
	ey := rowOf(t, m, "quote")
	got = drag(m, colOf(t, m, cy, "foo")+2, cy, colOf(t, m, ey, "quote")+1, ey)
	// The rows of a code block map to its code lines. glamour does not show the fences.
	if got != "foo := 1\nbar := 2\n```\n\n> qu" {
		t.Errorf("code start: %q", got)
	}
	// A single character.
	y = rowOf(t, m, "last")
	x := colOf(t, m, y, "last")
	if got := drag(m, x, y, x+1, y); got != "la" {
		t.Errorf("two: %q", got)
	}
}

func TestSelectOneCharacterStatus(t *testing.T) {
	m := selModel(t)
	y := rowOf(t, m, "last")
	x := colOf(t, m, y, "last")
	m.Update(tea.MouseClickMsg{X: x + 1, Y: y, Button: tea.MouseLeft})
	m.Update(tea.MouseMotionMsg{X: x, Y: y, Button: tea.MouseLeft})
	m.Update(tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
	if m.msg != "copied 2 characters" {
		t.Errorf("status %q", m.msg)
	}
}

func TestSelectHighlightClearsOnKey(t *testing.T) {
	m := selModel(t)
	y := rowOf(t, m, "last")
	x := colOf(t, m, y, "last")
	drag(m, x, y, x+3, y)
	if !m.vsel.on || !strings.Contains(m.View().Content, "48;2;54;54;54") {
		t.Errorf("no highlight: %q", m.View().Content)
	}
	press(m, "e")
	if m.vsel.on || strings.Contains(m.View().Content, "48;2;54;54;54") {
		t.Error("the highlight stays after a key")
	}
	if m.mode != modeEdit {
		t.Error("the key did not run")
	}
}

func TestSelectClickStillWorks(t *testing.T) {
	m := selModel(t)
	y := rowOf(t, m, "task one")
	clickAt(m, colOf(t, m, y, "☐"), y)
	if got := string(m.doc.buf.Lines[4]); got != "- [x] task one" {
		t.Errorf("line %q", got)
	}
	if m.vsel.on {
		t.Error("a click made a selection")
	}
	// A press that moves is a drag: no toggle.
	y = rowOf(t, m, "task one")
	x := colOf(t, m, y, "☑")
	drag(m, x, y, x+3, y)
	if got := string(m.doc.buf.Lines[4]); got != "- [x] task one" {
		t.Errorf("drag toggled: %q", got)
	}
}

func TestSelectDragScrolls(t *testing.T) {
	m := testModelSize(t, writeTemp(t, "a.md", strings.Repeat("line\n\n", 40), 0o644), 60, 12)
	m.showTree = false
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 12})
	m.Update(tea.MouseClickMsg{X: 3, Y: 2, Button: tea.MouseLeft})
	m.Update(tea.MouseMotionMsg{X: 3, Y: 11, Button: tea.MouseLeft}) // below the text
	if m.doc.off != 1 {
		t.Errorf("off %d", m.doc.off)
	}
	m.Update(tea.MouseWheelMsg{X: 3, Y: 5, Button: tea.MouseWheelDown})
	if m.doc.off != 4 || m.vsel.h.row != 4+4 {
		t.Errorf("wheel: off %d head %+v", m.doc.off, m.vsel.h)
	}
	m.Update(tea.MouseMotionMsg{X: 3, Y: 0, Button: tea.MouseLeft}) // above the text
	if m.doc.off != 3 {
		t.Errorf("off %d", m.doc.off)
	}
}
