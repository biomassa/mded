package buffer

import (
	"reflect"
	"testing"
)

// TestOnShift checks the line shifts that the edits report.
func TestOnShift(t *testing.T) {
	tests := []struct {
		name string
		text string
		row  int
		col  int
		do   func(b *Buffer)
		want []Shift
	}{
		{"split", "abc\nd", 0, 1, func(b *Buffer) { b.SplitLine() }, []Shift{{At: 1, N: 1}}},
		{"insert text lines", "a", 0, 1, func(b *Buffer) { b.InsertText("x\ny\nz") },
			[]Shift{{At: 1, N: 1}, {At: 2, N: 1}}},
		{"insert lines", "a\nb", 0, 0, func(b *Buffer) { b.InsertLines(1, "x\ny") }, []Shift{{At: 1, N: 2}}},
		{"delete lines", "a\nb\nc\nd", 0, 0, func(b *Buffer) { b.DeleteLines(1, 2) }, []Shift{{At: 1, N: -2}}},
		{"delete range", "ab\ncd\nef\ngh", 0, 1, func(b *Buffer) { b.DeleteRange(0, 1, 2, 1) },
			[]Shift{{At: 1, N: -1}, {At: 1, N: -1, Join: true, Col: 0}}},
		{"delete range one line", "abc", 0, 0, func(b *Buffer) { b.DeleteRange(0, 0, 0, 2) }, nil},
		{"join", "a\nb\nc", 0, 0, func(b *Buffer) { b.JoinLines(3, true) },
			[]Shift{{At: 1, N: -2, Join: true}}},
		{"backspace join", "ab\ncd", 1, 0, func(b *Buffer) { b.Backspace() },
			[]Shift{{At: 1, N: -1, Join: true, Col: 2}}},
		{"delete forward join", "ab\ncd", 0, 2, func(b *Buffer) { b.DeleteForward() },
			[]Shift{{At: 1, N: -1, Join: true, Col: 2}}},
		{"no line change", "ab", 0, 0, func(b *Buffer) { b.Insert([]rune("x")) }, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := New(tc.text)
			b.Row, b.Col = tc.row, tc.col
			var got []Shift
			b.OnShift = func(s Shift) { got = append(got, s) }
			tc.do(b)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("shifts %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestOnShiftNil checks that edits work without a callback.
func TestOnShiftNil(t *testing.T) {
	b := New("a\nb")
	b.SplitLine()
	b.DeleteLines(0, 0)
	if b.Text() != "a\nb" {
		t.Errorf("text %q", b.Text())
	}
}

// TestUndoShift checks the line shifts that undo and redo report.
func TestUndoShift(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		do       func(b *Buffer)
		undo, re []Shift
	}{
		{"delete lines", "a\nb\nc\nd", func(b *Buffer) { b.DeleteLines(1, 2) },
			[]Shift{{At: 1, N: 2}}, []Shift{{At: 1, N: -2}}},
		{"insert lines", "a\nb", func(b *Buffer) { b.InsertLines(1, "x\ny") },
			[]Shift{{At: 1, N: -2}}, []Shift{{At: 1, N: 2}}},
		{"split", "abc\nd", func(b *Buffer) { b.Col = 1; b.SplitLine() },
			[]Shift{{At: 0, N: -2}, {At: 0, N: 1}}, []Shift{{At: 0, N: -1}, {At: 0, N: 2}}},
		{"join", "a\nb\nc\nd", func(b *Buffer) { b.JoinLines(3, true) },
			[]Shift{{At: 0, N: -1}, {At: 0, N: 3}}, []Shift{{At: 0, N: -3}, {At: 0, N: 1}}},
		{"same line count", "a\nb", func(b *Buffer) { b.SetLine(1, []rune("x")) }, nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := New(tc.text)
			b.PushUndo(false)
			tc.do(b)
			var got []Shift
			b.OnShift = func(s Shift) { got = append(got, s) }
			b.Undo()
			if !reflect.DeepEqual(got, tc.undo) {
				t.Errorf("undo shifts %+v, want %+v", got, tc.undo)
			}
			if b.Text() != tc.text {
				t.Errorf("text after undo %q", b.Text())
			}
			got = nil
			b.Redo()
			if !reflect.DeepEqual(got, tc.re) {
				t.Errorf("redo shifts %+v, want %+v", got, tc.re)
			}
		})
	}
}

// TestUndoAux checks that OnSnap values go with the undo steps and come back through
// OnRestore.
func TestUndoAux(t *testing.T) {
	b := New("a\nb")
	n := 0
	b.OnSnap = func() any { n++; return n }
	var restored []any
	b.OnRestore = func(a any) { restored = append(restored, a) }
	b.PushUndo(false) // value 1
	b.InsertLines(0, "x")
	b.Undo() // snapshot for redo has value 2
	b.Redo() // snapshot for undo has value 3
	want := []any{1, 2}
	if !reflect.DeepEqual(restored, want) {
		t.Errorf("restored %v, want %v", restored, want)
	}
}
