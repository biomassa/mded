package buffer

import "testing"

func TestVimToGo(t *testing.T) {
	tests := []struct{ in, want string }{
		{`\(a\|b\)c`, `(a|b)c`},
		{`\<word\>`, `\bword\b`},
		{`a\.b`, `a\.b`},
		{`a\{2,3}`, `a{2,3}`},
		{`a\{2,3\}`, `a{2,3}`},
		{`a\{-}`, `a\{-}`},
		{`\\(`, `\\(`},
		{`x\`, `x\`},
		{`(a|b)`, `(a|b)`},
	}
	for _, tc := range tests {
		if got := VimToGo(tc.in); got != tc.want {
			t.Errorf("VimToGo(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseSubFlags(t *testing.T) {
	f, err := ParseSubFlags("gIc")
	if err != nil || !f.Global || !f.MatchCase || !f.Confirm || f.IgnoreCase {
		t.Errorf("flags = %+v %v", f, err)
	}
	if _, err := ParseSubFlags("gz"); err == nil {
		t.Error("no error for z")
	}
}

func TestSubstitute(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		r1, r2       int
		pat, rep, fl string
		want         string
		count, lines int
	}{
		{"first per line", "aa aa\naa", 0, 1, "a", "b", "", "ba aa\nba", 2, 2},
		{"global", "aa aa\naa", 0, 1, "a", "b", "g", "bb bb\nbb", 6, 2},
		{"range", "a\na\na", 1, 1, "a", "b", "g", "a\nb\na", 1, 1},
		{"reversed range", "a\na\na", 2, 1, "a", "b", "", "a\nb\nb", 2, 2},
		{"range beyond end", "a\na", 0, 99, "a", "b", "", "b\nb", 2, 2},
		{"groups", "john smith", 0, 0, `\(\w\+\) \(\w\+\)`, `\2, \1`, "", "john smith", 0, 0},
		{"groups go style", "john smith", 0, 0, `(\w+) (\w+)`, `\2, \1`, "", "smith, john", 1, 1},
		{"vim groups", "ab12", 0, 0, `\(ab\)\(12\)`, `\2\1`, "", "12ab", 1, 1},
		{"alternation", "cat dog cow", 0, 0, `cat\|dog`, "pet", "g", "pet pet cow", 2, 1},
		{"amp", "foo", 0, 0, "o+", "[&]", "", "f[oo]", 1, 1},
		{"escaped amp", "foo", 0, 0, "o+", `\&`, "", "f&", 1, 1},
		{"backslash", "a", 0, 0, "a", `\\`, "", `\`, 1, 1},
		{"zero group", "ab", 0, 0, "b", `<\0>`, "", "a<b>", 1, 1},
		{"newline", "a,b,c", 0, 0, ",", `\n`, "g", "a\nb\nc", 2, 1},
		{"newline then rest", "a,b\nx,y", 0, 1, ",", `\n`, "g", "a\nb\nx\ny", 2, 2},
		{"newline with text", "k=v", 0, 0, "=", `\n  `, "", "k\n  v", 1, 1},
		{"word boundary", "cat concat cat", 0, 0, `\<cat\>`, "X", "g", "X concat X", 2, 1},
		{"empty match end", "abc", 0, 0, "$", "!", "", "abc!", 1, 1},
		{"empty match start", "abc", 0, 0, "^", "# ", "", "# abc", 1, 1},
		{"star empty", "abc", 0, 0, "x*", "-", "g", "-a-b-c-", 4, 1},
		{"smartcase lower", "Foo foo", 0, 0, "foo", "x", "g", "x x", 2, 1},
		{"smartcase upper", "Foo foo", 0, 0, "Foo", "x", "g", "x foo", 1, 1},
		{"flag i", "Foo foo", 0, 0, "Foo", "x", "gi", "x x", 2, 1},
		{"flag I", "Foo foo", 0, 0, "foo", "x", "gI", "Foo x", 1, 1},
		{"unicode", "żółw żółw", 0, 0, "ó", "o", "g", "żołw żołw", 2, 1},
		{"unicode then ascii", "żółw x", 0, 0, `w|x`, "Y", "g", "żółY Y", 2, 1},
		{"no match", "abc", 0, 0, "z", "y", "g", "abc", 0, 0},
		{"empty replacement", "a-b", 0, 0, "-", "", "", "ab", 1, 1},
		{"tab", "a b", 0, 0, " ", `\t`, "", "a\tb", 1, 1},
		{"adjacent global", "aaa", 0, 0, "a", "bb", "g", "bbbbbb", 3, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := New(tc.text)
			fl, err := ParseSubFlags(tc.fl)
			if err != nil {
				t.Fatal(err)
			}
			res, err := b.Substitute(tc.r1, tc.r2, tc.pat, tc.rep, fl)
			if err != nil {
				t.Fatal(err)
			}
			if b.Text() != tc.want || res.Count != tc.count || res.Lines != tc.lines {
				t.Errorf("text %q count %d lines %d, want %q %d %d",
					b.Text(), res.Count, res.Lines, tc.want, tc.count, tc.lines)
			}
			if tc.count > 0 && b.UndoDepth() != 1 {
				t.Errorf("undo steps = %d", b.UndoDepth())
			}
			if tc.count == 0 && b.UndoDepth() != 0 {
				t.Errorf("a failed :s made an undo step")
			}
		})
	}
}

func TestSubstituteErrors(t *testing.T) {
	b := New("abc")
	if _, err := b.Substitute(0, 0, `\(`, "x", SubFlags{}); err == nil {
		t.Error("no error for a bad pattern")
	}
	if b.InChange() {
		t.Error("undo group left open")
	}
	if _, err := b.Substitute(0, 0, "", "x", SubFlags{}); err == nil {
		t.Error("no error without a previous pattern")
	}
	b.SetSearch("b", false)
	if res, err := b.Substitute(0, 0, "", "X", SubFlags{}); err != nil || res.Count != 1 || b.Text() != "aXc" {
		t.Errorf("last pattern: %v %q", err, b.Text())
	}
}

func TestSubstituteUndo(t *testing.T) {
	b := New("a\na\na")
	b.Row, b.Col = 1, 0
	b.Substitute(0, 2, "a", "bb", SubFlags{Global: true})
	if b.Text() != "bb\nbb\nbb" || b.Row != 2 || b.Col != 0 {
		t.Fatalf("text %q cursor %d,%d", b.Text(), b.Row, b.Col)
	}
	b.Undo()
	if b.Text() != "a\na\na" || b.Row != 1 {
		t.Errorf("undo: %q row %d", b.Text(), b.Row)
	}
	// Inside a vim command group, :s joins the group step.
	b.BeginChange()
	b.Substitute(0, 0, "a", "x", SubFlags{})
	b.EndChange()
	if b.UndoDepth() != 1 {
		t.Errorf("steps = %d", b.UndoDepth())
	}
}

// confirm runs a :s with answers per match: y replace, n skip, a all, q quit.
func confirm(t *testing.T, text, pat, rep, flags, answers string) (string, SubResult, []Match) {
	t.Helper()
	b := New(text)
	fl, _ := ParseSubFlags(flags)
	j, err := b.NewSubstitute(0, b.LineCount()-1, pat, rep, fl)
	if err != nil {
		t.Fatal(err)
	}
	var seen []Match
	for _, a := range answers {
		m, ok := j.Next()
		if !ok {
			t.Fatalf("no match for answer %q", a)
		}
		seen = append(seen, m)
		if b.Row != m.Row || b.Col != m.A {
			t.Errorf("cursor %d,%d not at match %v", b.Row, b.Col, m)
		}
		switch a {
		case 'y':
			j.Replace()
		case 'n':
			j.Skip()
		case 'a':
			j.ReplaceRest()
		}
		if a == 'q' || a == 'a' {
			break
		}
	}
	res := j.Finish()
	if b.UndoDepth() > 1 {
		t.Errorf("undo steps = %d", b.UndoDepth())
	}
	return b.Text(), res, seen
}

func TestSubstituteConfirm(t *testing.T) {
	got, res, seen := confirm(t, "a a a\na", "a", "bb", "gc", "ynyy")
	if got != "bb a bb\nbb" || res.Count != 3 || res.Lines != 2 {
		t.Errorf("ynyy: %q %+v", got, res)
	}
	// The reported positions follow the changes of earlier answers.
	want := []Match{{0, 0, 1}, {0, 3, 4}, {0, 5, 6}, {1, 0, 1}}
	for i, m := range seen {
		if m != want[i] {
			t.Errorf("match %d = %v, want %v", i, m, want[i])
		}
	}
	got, res, _ = confirm(t, "a a a", "a", "b", "gc", "na")
	if got != "a b b" || res.Count != 2 {
		t.Errorf("na: %q %+v", got, res)
	}
	got, res, _ = confirm(t, "a a a", "a", "b", "gc", "yq")
	if got != "b a a" || res.Count != 1 {
		t.Errorf("yq: %q %+v", got, res)
	}
	// A replacement with a line break moves the later matches.
	got, res, seen = confirm(t, "a,a,a\nx", "a", `\n`, "gc", "yyy")
	if got != "\n,\n,\n\nx" {
		t.Errorf("newline: %q", got)
	}
	if want := []Match{{0, 0, 1}, {1, 1, 2}, {2, 1, 2}}; len(seen) != 3 || seen[1] != want[1] || seen[2] != want[2] {
		t.Errorf("newline matches: %v", seen)
	}
	// Next reports false at the end, and Finish twice is harmless.
	b := New("a")
	j, _ := b.NewSubstitute(0, 0, "a", "b", SubFlags{})
	j.Next()
	j.Skip()
	if _, ok := j.Next(); ok {
		t.Error("match after the last")
	}
	if r := j.Finish(); r.Count != 0 || j.Finish().Count != 0 || b.InChange() {
		t.Error("finish state")
	}
	if j.Rep() != "b" {
		t.Error("Rep")
	}
}
