package buffer

import (
	"reflect"
	"testing"
)

func TestSmartcase(t *testing.T) {
	tests := []struct {
		pat, in string
		want    bool
	}{
		{"foo", "FOO", true},
		{"Foo", "foo", false},
		{"Foo", "Foo", true},
		{`\S+`, "ABC", true}, // \S is no uppercase letter
		{`\Sx`, "AX", true},
		{"a.*B", "axb", false},
		{"żó", "ŻÓ", true},
		{"Żó", "żó", false},
	}
	for _, tc := range tests {
		re, err := CompilePattern(tc.pat)
		if err != nil {
			t.Fatal(err)
		}
		if got := re.MatchString(tc.in); got != tc.want {
			t.Errorf("%q on %q = %v, want %v", tc.pat, tc.in, got, tc.want)
		}
	}
	if _, err := CompilePattern("(unclosed"); err == nil {
		t.Error("no error for a bad pattern")
	}
}

func TestSetSearch(t *testing.T) {
	b := New("foo bar foo\nżółw foo\n\nFOO")
	if err := b.SetSearch("foo", false); err != nil {
		t.Fatal(err)
	}
	want := []Match{{0, 0, 3}, {0, 8, 11}, {1, 5, 8}, {3, 0, 3}} // unicode offsets are runes
	if !reflect.DeepEqual(b.Search.Matches, want) || !b.Search.Highlight {
		t.Errorf("matches = %v", b.Search.Matches)
	}
	b.SetSearch("Foo", false)
	if len(b.Search.Matches) != 0 {
		t.Errorf("uppercase pattern matched %v", b.Search.Matches)
	}
	b.SetSearch("x*", false) // empty matches do not count
	if len(b.Search.Matches) != 0 {
		t.Errorf("empty matches: %v", b.Search.Matches)
	}
	if err := b.SetSearch("(", false); err == nil || b.Search.Highlight || b.Search.Re != nil {
		t.Error("bad pattern state")
	}
	b.SetSearch("", false)
	if b.Search.Re != nil || b.Search.Highlight {
		t.Error("empty pattern state")
	}
}

func TestSearchNavigation(t *testing.T) {
	b := New("a x\nb\nx x\nc")
	b.SetSearch("x", false)
	steps := []struct {
		reverse bool
		row     int
		col     int
		wrapped bool
	}{
		{false, 0, 2, false},
		{false, 2, 0, false},
		{false, 2, 2, false},
		{false, 0, 2, true}, // wraps to the top
		{true, 2, 2, true},  // wraps to the bottom
		{true, 2, 0, false},
	}
	for i, s := range steps {
		wr, ok := b.SearchJump(s.reverse)
		if !ok || wr != s.wrapped || b.Row != s.row || b.Col != s.col {
			t.Errorf("step %d: ok=%v wrapped=%v at %d,%d, want %d,%d wrapped=%v",
				i, ok, wr, b.Row, b.Col, s.row, s.col, s.wrapped)
		}
	}
	// A backward search reverses n.
	b.SetSearch("x", true)
	b.MoveTo(3, 0)
	b.SearchJump(false)
	if b.Row != 2 || b.Col != 2 {
		t.Errorf("backward n at %d,%d", b.Row, b.Col)
	}
	if b.Search.Cur != 2 {
		t.Errorf("Cur = %d", b.Search.Cur)
	}
}

func TestSearchAtIncremental(t *testing.T) {
	b := New("one\ntwo three\nthree")
	b.SetSearch("t", false)
	if wr, ok := b.SearchAt(1, 0); !ok || wr || b.Row != 1 || b.Col != 0 { // inclusive
		t.Errorf("inclusive: %v %v %d,%d", ok, wr, b.Row, b.Col)
	}
	if wr, ok := b.SearchAt(2, 3); !ok || !wr || b.Row != 1 || b.Col != 0 {
		t.Errorf("wrap: %v %v %d,%d", ok, wr, b.Row, b.Col)
	}
	b.SetSearch("zzz", false)
	if _, ok := b.SearchAt(0, 0); ok || b.Search.Cur != -1 {
		t.Error("found a missing pattern")
	}
	if _, ok := b.SearchJump(false); ok {
		t.Error("jump without match")
	}
}

func TestSearchHighlightAndRefresh(t *testing.T) {
	b := New("ab ab")
	b.SetSearch("ab", false)
	b.PushUndo(false)
	b.Insert([]rune("ab ")) // text is now "ab ab ab"
	b.Refresh()
	if len(b.Search.Matches) != 3 {
		t.Errorf("matches after edit = %d", len(b.Search.Matches))
	}
	b.ClearHighlight()
	b.Refresh() // highlight off: no matches come back
	if len(b.Search.Matches) != 0 || b.Search.Pattern != "ab" {
		t.Error("ClearHighlight")
	}
	b.SearchJump(false) // n turns the highlight on again
	if !b.Search.Highlight || len(b.Search.Matches) != 3 {
		t.Error("n did not restore highlight")
	}
	b.Undo() // undo refreshes the matches
	if len(b.Search.Matches) != 2 {
		t.Errorf("matches after undo = %d", len(b.Search.Matches))
	}
}

func TestFindLiteral(t *testing.T) {
	b := New("Foo foo.*\naaaa")
	got := b.FindLiteral("foo", false)
	if !reflect.DeepEqual(got, []Match{{0, 0, 3}, {0, 4, 7}}) {
		t.Errorf("ignore case: %v", got)
	}
	if got := b.FindLiteral("foo", true); len(got) != 1 || got[0].A != 4 {
		t.Errorf("match case: %v", got)
	}
	if got := b.FindLiteral(".*", false); len(got) != 1 { // literal, no regexp
		t.Errorf("literal: %v", got)
	}
	if got := b.FindLiteral("aa", false); len(got) != 2 { // no overlap
		t.Errorf("overlap: %v", got)
	}
	if b.FindLiteral("", false) != nil {
		t.Error("empty query")
	}
}

func TestReplaceLiteral(t *testing.T) {
	b := New("a b a\nb a")
	if n := b.ReplaceAll("a", "xyz", true); n != 3 || b.Text() != "xyz b xyz\nb xyz" {
		t.Errorf("ReplaceAll = %d %q", n, b.Text())
	}
	if n := b.ReplaceAll("xyz", "", true); n != 3 || b.Text() != " b \nb " { // an empty field removes
		t.Errorf("remove = %d %q", n, b.Text())
	}
	b = New("one two")
	ms := b.FindLiteral("two", true)
	b.ReplaceMatch(ms[0], "three")
	if b.Text() != "one three" || b.Col != 9 {
		t.Errorf("ReplaceMatch: %q col %d", b.Text(), b.Col)
	}
	b.ReplaceMatch(Match{5, 0, 1}, "x") // a bad match changes nothing
	if b.Text() != "one three" {
		t.Error("bad match changed text")
	}
}
