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
