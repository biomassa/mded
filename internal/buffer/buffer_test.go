package buffer

import (
	"reflect"
	"strings"
	"testing"
)

// at makes a buffer from text with the cursor at row, col.
func at(text string, row, col int) *Buffer {
	b := New(text)
	b.Row, b.Col = row, col
	return b
}

func TestEditOps(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		row, col int
		op       func(b *Buffer)
		want     string
		wr, wc   int
	}{
		{"insert", "ab", 0, 1, func(b *Buffer) { b.Insert([]rune("XY")) }, "aXYb", 0, 3},
		{"insert unicode", "żó", 0, 1, func(b *Buffer) { b.Insert([]rune("é")) }, "żéó", 0, 2},
		{"insert text multiline", "ab", 0, 1, func(b *Buffer) { b.InsertText("1\r\n2\n3") }, "a1\n2\n3b", 2, 1},
		{"split", "abcd", 0, 2, func(b *Buffer) { b.SplitLine() }, "ab\ncd", 1, 0},
		{"newline plain", "abcd", 0, 2, func(b *Buffer) { b.Newline() }, "ab\ncd", 1, 0},
		{"newline bullet", "- item", 0, 6, func(b *Buffer) { b.Newline() }, "- item\n- ", 1, 2},
		{"newline numbered", "9. item", 0, 7, func(b *Buffer) { b.Newline() }, "9. item\n10. ", 1, 4},
		{"newline paren number", "1) a", 0, 4, func(b *Buffer) { b.Newline() }, "1) a\n2) ", 1, 3},
		{"newline checkbox", "  - [x] done", 0, 12, func(b *Buffer) { b.Newline() }, "  - [x] done\n  - [ ] ", 1, 8},
		{"newline empty item ends list", "- ", 0, 2, func(b *Buffer) { b.Newline() }, "", 0, 0},
		{"newline inside marker", "- item", 0, 1, func(b *Buffer) { b.Newline() }, "-\n item", 1, 0},
		{"newline mid item", "- ab", 0, 3, func(b *Buffer) { b.Newline() }, "- a\n- b", 1, 2},
		{"backspace", "abc", 0, 2, func(b *Buffer) { b.Backspace() }, "ac", 0, 1},
		{"backspace join", "ab\ncd", 1, 0, func(b *Buffer) { b.Backspace() }, "abcd", 0, 2},
		{"backspace at start", "ab", 0, 0, func(b *Buffer) { b.Backspace() }, "ab", 0, 0},
		{"delete", "abc", 0, 1, func(b *Buffer) { b.DeleteForward() }, "ac", 0, 1},
		{"delete join", "ab\ncd", 0, 2, func(b *Buffer) { b.DeleteForward() }, "abcd", 0, 2},
		{"delete at end", "ab", 0, 2, func(b *Buffer) { b.DeleteForward() }, "ab", 0, 2},
		{"delete word back", "foo bar", 0, 7, func(b *Buffer) { b.DeleteWordBack() }, "foo ", 0, 4},
		{"delete word back punct", "foo, bar", 0, 5, func(b *Buffer) { b.DeleteWordBack() }, "bar", 0, 0},
		{"delete word back join", "a\nb", 1, 0, func(b *Buffer) { b.DeleteWordBack() }, "ab", 0, 1},
		{"word left", "foo bar", 0, 7, func(b *Buffer) { b.WordLeft() }, "foo bar", 0, 4},
		{"word left wrap", "foo\nbar", 1, 0, func(b *Buffer) { b.WordLeft() }, "foo\nbar", 0, 3},
		{"word right", "foo bar", 0, 0, func(b *Buffer) { b.WordRight() }, "foo bar", 0, 3},
		{"word right wrap", "foo\nbar", 0, 3, func(b *Buffer) { b.WordRight() }, "foo\nbar", 1, 0},
		{"move left wrap", "a\nb", 1, 0, func(b *Buffer) { b.MoveLeft() }, "a\nb", 0, 1},
		{"move right wrap", "a\nb", 0, 1, func(b *Buffer) { b.MoveRight() }, "a\nb", 1, 0},
		{"wrap word bold", "say hello now", 0, 6, func(b *Buffer) { b.WrapWord("**", "**") }, "say **hello** now", 0, 13},
		{"wrap empty italic", "a  b", 0, 2, func(b *Buffer) { b.WrapWord("*", "*") }, "a ** b", 0, 3},
		{"wrap word link", "see docs", 0, 5, func(b *Buffer) { b.WrapWord("[", "]()") }, "see [docs]()", 0, 11},
		{"wrap empty link", "x ", 0, 2, func(b *Buffer) { b.WrapWord("[", "]()") }, "x []()", 0, 3},
		{"checkbox new", "task", 0, 2, func(b *Buffer) { b.ToggleLineCheckbox() }, "- [ ] task", 0, 8},
		{"checkbox on list item", "- task", 0, 0, func(b *Buffer) { b.ToggleLineCheckbox() }, "- [ ] task", 0, 4},
		{"checkbox check", "- [ ] task", 0, 0, func(b *Buffer) { b.ToggleLineCheckbox() }, "- [x] task", 0, 0},
		{"checkbox uncheck", "1. [X] task", 0, 0, func(b *Buffer) { b.ToggleLineCheckbox() }, "1. [ ] task", 0, 0},
		{"indent", "a\n\nb", 0, 0, func(b *Buffer) { b.Indent(0, 2, 4) }, "    a\n\n    b", 0, 0},
		{"outdent", "      a\n  b\nc\n\td", 0, 0, func(b *Buffer) { b.Outdent(0, 3, 4) }, "  a\nb\nc\nd", 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := at(tc.text, tc.row, tc.col)
			tc.op(b)
			if got := b.Text(); got != tc.want {
				t.Errorf("text = %q, want %q", got, tc.want)
			}
			if b.Row != tc.wr || b.Col != tc.wc {
				t.Errorf("cursor = %d,%d, want %d,%d", b.Row, b.Col, tc.wr, tc.wc)
			}
		})
	}
}

func TestWrapLine(t *testing.T) {
	tests := []struct {
		l    string
		w    int
		want []int
	}{
		{"short", 10, []int{0}},
		{"aaaa bbbb cccc", 8, []int{0, 5, 10}},
		{"aaaaaaaaaaaa", 5, []int{0, 5, 10}},
		{"", 5, []int{0}},
	}
	for _, tc := range tests {
		if got := WrapLine([]rune(tc.l), tc.w); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("WrapLine(%q, %d) = %v, want %v", tc.l, tc.w, got, tc.want)
		}
	}
}

func TestMoveVertical(t *testing.T) {
	b := at("abcdefgh\nab\nabcdefgh", 0, 6)
	b.MoveVertical(1, 20)
	if b.Row != 1 || b.Col != 2 || b.Want != 6 {
		t.Fatalf("down: %d,%d want=%d", b.Row, b.Col, b.Want)
	}
	b.MoveVertical(1, 20) // the wanted column comes back on the long line
	if b.Row != 2 || b.Col != 6 {
		t.Fatalf("down 2: %d,%d", b.Row, b.Col)
	}
	b.MoveVertical(1, 20) // no line below
	if b.Row != 2 {
		t.Fatalf("below the end: %d", b.Row)
	}
	// Wrapped line: width 5 splits "aaaa bbbb" into two screen lines.
	b = at("aaaa bbbb\nxx", 0, 1)
	b.MoveVertical(1, 5)
	if b.Row != 0 || b.Col != 6 {
		t.Fatalf("wrapped down: %d,%d", b.Row, b.Col)
	}
	b.MoveVertical(1, 5)
	if b.Row != 1 || b.Col != 1 {
		t.Fatalf("next line: %d,%d", b.Row, b.Col)
	}
	if y, x := b.ScreenPos(5); y != 2 || x != 1 {
		t.Fatalf("ScreenPos = %d,%d", y, x)
	}
}

func TestRangeOps(t *testing.T) {
	b := at("one two\nthree\nfour five", 0, 0)
	if got := b.TextRange(0, 4, 2, 4); got != "two\nthree\nfour" {
		t.Errorf("TextRange = %q", got)
	}
	if got := b.TextRange(2, 4, 0, 4); got != "two\nthree\nfour" { // reversed
		t.Errorf("reversed TextRange = %q", got)
	}
	if got := b.DeleteRange(0, 4, 2, 4); got != "two\nthree\nfour" || b.Text() != "one  five" {
		t.Errorf("DeleteRange = %q, text %q", got, b.Text())
	}
	if b.Row != 0 || b.Col != 4 {
		t.Errorf("cursor %d,%d", b.Row, b.Col)
	}
	old := b.ReplaceRange(0, 0, 0, 3, "1\n2")
	if old != "one" || b.Text() != "1\n2  five" || b.Row != 1 || b.Col != 1 {
		t.Errorf("ReplaceRange: old %q text %q cursor %d,%d", old, b.Text(), b.Row, b.Col)
	}
	b.DeleteRange(0, 0, 0, 0) // an empty range changes nothing
	if b.Text() != "1\n2  five" {
		t.Errorf("empty range changed the text")
	}
	b.DeleteRange(-5, -5, 99, 99) // out of range positions are limited
	if b.Text() != "" || b.LineCount() != 1 {
		t.Errorf("clamped delete: %q", b.Text())
	}
}

func TestLineOps(t *testing.T) {
	b := at("a\nb\nc\nd", 1, 1)
	if got := b.YankLines(1, 2); got != "b\nc" {
		t.Errorf("YankLines = %q", got)
	}
	if got := b.DeleteLines(1, 2); got != "b\nc" || b.Text() != "a\nd" || b.Row != 1 || b.Col != 0 {
		t.Errorf("DeleteLines = %q, %q, row %d", got, b.Text(), b.Row)
	}
	b.DeleteLines(1, 1)
	if b.Text() != "a" || b.Row != 0 {
		t.Errorf("delete last: %q row %d", b.Text(), b.Row)
	}
	b.DeleteLines(0, 0)
	if b.Text() != "" || b.LineCount() != 1 {
		t.Errorf("delete all: %q", b.Text())
	}
	b = at("x\ny", 0, 0)
	b.InsertLines(2, "p\nq")
	if b.Text() != "x\ny\np\nq" || b.Row != 2 {
		t.Errorf("InsertLines end: %q row %d", b.Text(), b.Row)
	}
	b.InsertLines(0, "top")
	if b.Text() != "top\nx\ny\np\nq" || b.Row != 0 {
		t.Errorf("InsertLines top: %q", b.Text())
	}
}

func TestJoinLines(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		n     int
		space bool
		want  string
		col   int
		ok    bool
	}{
		{"two with space", "foo\n   bar", 2, true, "foo bar", 3, true},
		{"count 1 acts as 2", "a\nb", 1, true, "a b", 1, true},
		{"three", "a\nb\nc\nd", 3, true, "a b c\nd", 3, true},
		{"no space", "foo\n  bar", 2, false, "foo  bar", 3, true},
		{"trailing blank", "foo \nbar", 2, true, "foo bar", 3, true},
		{"close paren", "f(a\n)", 2, true, "f(a)", 3, true},
		{"empty next", "foo\n", 2, true, "foo", 3, true},
		{"empty current", "\nbar", 2, true, "bar", 0, true},
		{"last line", "a", 2, true, "a", 0, false},
		{"count past end", "a\nb", 5, true, "a b", 1, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := at(tc.text, 0, 0)
			if ok := b.JoinLines(tc.n, tc.space); ok != tc.ok {
				t.Errorf("ok = %v", ok)
			}
			if b.Text() != tc.want || b.Col != tc.col {
				t.Errorf("got %q col %d, want %q col %d", b.Text(), b.Col, tc.want, tc.col)
			}
		})
	}
}

func TestChangeCase(t *testing.T) {
	tests := []struct {
		mode CaseMode
		want string
	}{
		{CaseUpper, "HELLO ŻÓŁW\nWORLD x"},
		{CaseLower, "hello żółw\nworld x"},
		{CaseToggle, "HELLO żółw\nwORLD x"},
	}
	for _, tc := range tests {
		b := at("hello żółw\nWorld x", 1, 3)
		b.ChangeCase(0, 0, 1, 5, tc.mode) // from line 0 col 0 to line 1 col 5: " x" stays
		if tc.mode == CaseUpper {
			// Upper case of the whole range, but only to col 5 of line 1.
			if b.Text() != "HELLO ŻÓŁW\nWORLD x" {
				t.Errorf("upper: %q", b.Text())
			}
			continue
		}
		if tc.mode == CaseToggle {
			// Toggle: lower letters go up, upper letters go down.
			if b.Text() != "HELLO ŻÓŁW\nwORLD x" {
				t.Errorf("toggle: %q", b.Text())
			}
			continue
		}
		if b.Text() != tc.want {
			t.Errorf("lower: %q, want %q", b.Text(), tc.want)
		}
		if b.Row != 1 || b.Col != 3 {
			t.Errorf("the cursor moved")
		}
	}
	b := at("abc\ndef", 0, 0)
	b.ChangeCaseLines(0, 1, CaseUpper)
	if b.Text() != "ABC\nDEF" {
		t.Errorf("ChangeCaseLines = %q", b.Text())
	}
}

func TestSelection(t *testing.T) {
	b := at("abc\ndef\nghi", 0, 1)
	if _, _, _, _, ok := b.SelRange(); ok {
		t.Fatal("selection without anchor")
	}
	b.StartSel()
	b.Row, b.Col = 2, 2
	r1, c1, r2, c2, ok := b.SelRange()
	if !ok || r1 != 0 || c1 != 1 || r2 != 2 || c2 != 2 {
		t.Fatalf("SelRange = %d %d %d %d %v", r1, c1, r2, c2, ok)
	}
	if got := b.SelectedText(); got != "bc\ndef\ngh" {
		t.Errorf("SelectedText = %q", got)
	}
	for _, p := range []struct {
		r, c int
		in   bool
	}{{0, 0, false}, {0, 1, true}, {1, 5, true}, {2, 1, true}, {2, 2, false}} {
		if b.InSel(p.r, p.c) != p.in {
			t.Errorf("InSel(%d,%d) != %v", p.r, p.c, p.in)
		}
	}
	if !b.SelLine(1) || b.SelLine(3) {
		t.Error("SelLine")
	}
	// A reversed selection gives the same range.
	b.SelRow, b.SelCol, b.Row, b.Col = 2, 2, 0, 1
	if got := b.SelectedText(); got != "bc\ndef\ngh" {
		t.Errorf("reversed SelectedText = %q", got)
	}
	if !b.DeleteSelection() || b.Text() != "ai" || b.Row != 0 || b.Col != 1 || b.SelOn {
		t.Errorf("DeleteSelection: %q %d,%d", b.Text(), b.Row, b.Col)
	}
	if b.DeleteSelection() {
		t.Error("deleted without selection")
	}
	b.SelectAll()
	if b.SelectedText() != "ai" {
		t.Errorf("SelectAll = %q", b.SelectedText())
	}
	// An anchor equal to the cursor selects nothing.
	b.Row, b.Col = 0, 1
	b.SelRow, b.SelCol = 0, 1
	if b.ReplaceSelection() {
		t.Error("replaced an empty selection")
	}
}

func TestSelectionModes(t *testing.T) {
	b := at("abc\ndef\nghi", 0, 1)
	b.StartSel()
	b.Row, b.Col = 1, 1
	b.SelLinewise = true
	if got := b.SelectedText(); got != "abc\ndef" {
		t.Errorf("linewise = %q", got)
	}
	if !b.InSel(0, 0) || !b.InSel(1, 2) || b.InSel(2, 0) {
		t.Error("linewise InSel")
	}
	b.PushUndo(false)
	if !b.DeleteSelection() || b.Text() != "ghi" {
		t.Errorf("linewise delete = %q", b.Text())
	}
	b = at("abc\ndef", 0, 1)
	b.StartSel()
	b.SelInclusive = true
	if got := b.SelectedText(); got != "b" { // the rune under the cursor
		t.Errorf("inclusive one rune = %q", got)
	}
	b.Col = 3 // at the line end: the line break is included
	if got := b.SelectedText(); got != "bc\n" {
		t.Errorf("inclusive line end = %q", got)
	}
	b.SelRow, b.SelCol, b.Row, b.Col = 1, 2, 0, 0
	if got := b.SelectedText(); got != "abc\ndef" {
		t.Errorf("inclusive reversed = %q", got)
	}
}

func TestUndoConventional(t *testing.T) {
	b := New("")
	for _, r := range "abc" { // typing: one step
		b.PushUndo(true)
		b.Insert([]rune{r})
	}
	b.PushUndo(false)
	b.Newline()
	b.PushUndo(true)
	b.Insert([]rune("x"))
	if b.UndoDepth() != 3 {
		t.Fatalf("steps = %d", b.UndoDepth())
	}
	if !b.Undo() || b.Text() != "abc\n" || b.Row != 1 {
		t.Errorf("undo 1: %q", b.Text())
	}
	if !b.Undo() || b.Text() != "abc" || b.Row != 0 || b.Col != 3 {
		t.Errorf("undo 2: %q %d,%d", b.Text(), b.Row, b.Col)
	}
	if !b.Undo() || b.Text() != "" {
		t.Errorf("undo 3: %q", b.Text())
	}
	if b.Undo() {
		t.Error("undo past the start")
	}
	if !b.Redo() || b.Text() != "abc" {
		t.Errorf("redo: %q", b.Text())
	}
	b.PushUndo(false) // a new change clears redo
	b.Insert([]rune("!"))
	if b.RedoDepth() != 0 {
		t.Error("redo not cleared")
	}
	if !b.Dirty() {
		t.Error("not dirty")
	}
}

func TestUndoLimit(t *testing.T) {
	b := New("")
	for i := 0; i < 250; i++ {
		b.PushUndo(false)
		b.Insert([]rune("x"))
	}
	if b.UndoDepth() != 200 {
		t.Errorf("steps = %d", b.UndoDepth())
	}
}

func TestUndoVim(t *testing.T) {
	b := at("hello world", 0, 6)
	b.BeginChange() // one command: three edits
	b.DeleteRange(0, 6, 0, 11)
	b.PushUndo(false) // does nothing inside a group
	b.Insert([]rune("there"))
	b.DeleteForward()
	b.EndChange()
	if b.UndoDepth() != 1 {
		t.Fatalf("steps = %d", b.UndoDepth())
	}
	b.BeginChange() // a command with no change makes no step
	b.MoveLeft()
	b.EndChange()
	if b.UndoDepth() != 1 {
		t.Fatalf("empty command made a step")
	}
	b.BeginChange() // nested: the insert session holds a command
	b.Insert([]rune("A"))
	b.BeginChange()
	b.Insert([]rune("B"))
	b.EndChange()
	if b.UndoDepth() != 1 {
		t.Fatal("inner EndChange made a step")
	}
	if b.Undo() {
		t.Fatal("undo inside a group")
	}
	b.EndChange()
	if b.UndoDepth() != 2 {
		t.Fatalf("steps = %d", b.UndoDepth())
	}
	b.Undo()
	if b.Text() != "hello there" || b.Row != 0 {
		t.Errorf("undo: %q", b.Text())
	}
	b.Undo()
	if b.Text() != "hello world" || b.Row != 0 || b.Col != 6 { // the cursor from before the command
		t.Errorf("undo 2: %q %d", b.Text(), b.Col)
	}
	b.Redo()
	b.Redo()
	if b.Text() != "hello therABe" {
		t.Errorf("redo: %q", b.Text())
	}
	b.EndChange() // an extra EndChange is harmless
}

func TestDirtyAndSave(t *testing.T) {
	b := New("a\nb")
	if b.Dirty() {
		t.Error("new buffer dirty")
	}
	b.PushUndo(false)
	b.Insert([]rune("x"))
	if !b.Dirty() {
		t.Error("not dirty after edit")
	}
	b.Undo()
	if b.Dirty() { // back at the saved text
		t.Error("dirty after undo")
	}
	b.Redo()
	b.MarkSaved()
	if b.Dirty() || b.Saved != "xa\nb" {
		t.Errorf("after save: %v %q", b.Dirty(), b.Saved)
	}
	b.SetText("new")
	if b.UndoDepth() != 0 || b.Text() != "new" || b.Dirty() {
		t.Error("SetText")
	}
	if got := strings.Count(New("a\n\nb\n").Text(), "\n"); got != 3 { // the trailing line is kept
		t.Errorf("newlines = %d", got)
	}
}

func TestLineImmutableUndo(t *testing.T) {
	// An undo step shares the line slices, so no edit may change a line in place.
	b := at("abc", 0, 1)
	b.PushUndo(false)
	b.Insert([]rune("X"))
	b.PushUndo(false)
	b.Backspace()
	b.PushUndo(false)
	b.WrapWord("*", "*")
	b.ToggleLineCheckbox()
	b.ChangeCase(0, 0, 0, 3, CaseUpper)
	b.Indent(0, 0, 2)
	for want := range []string{"", "", ""} {
		_ = want
		b.Undo()
	}
	if b.Text() != "abc" {
		t.Errorf("text after undo = %q", b.Text())
	}
}
