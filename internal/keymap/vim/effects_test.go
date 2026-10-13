package vim

import (
	"testing"

	"github.com/biomassa/mded/internal/buffer"
	"github.com/biomassa/mded/internal/keymap"
)

func TestExEffects(t *testing.T) {
	tests := []struct {
		name   string
		dirty  bool
		script string
		kind   keymap.Kind
		path   string
		status string // expected status message, "" for none
	}{
		{"w", false, ":w<cr>", keymap.KindSave, "", ""},
		{"w file", false, ":w out.md<cr>", keymap.KindSaveAs, "out.md", ""},
		{"q clean", false, ":q<cr>", keymap.KindQuit, "", ""},
		{"q dirty refuses", true, ":q<cr>", -1, "", "unsaved changes · :q! discards · :wq saves"},
		{"q bang", true, ":q!<cr>", keymap.KindForceQuit, "", ""},
		{"wq", true, ":wq<cr>", keymap.KindSaveQuit, "", ""},
		{"x", true, ":x<cr>", keymap.KindSaveQuit, "", ""},
		{"e", false, ":e other.md<cr>", keymap.KindEditFile, "other.md", ""},
		{"ctrl+s", false, "<c-s>", keymap.KindSave, "", ""},
		{"esc exits", false, "<esc>", keymap.KindExitToView, "", ""},
		{"ctrl+g", false, "<c-g>", keymap.KindSwitchFlavor, "", ""},
		{"unknown", false, ":foo bar<cr>", -1, "", "not an editor command: foo bar"},
		{"no match", false, ":s/zzz/x/<cr>", -1, "", "pattern not found: zzz"},
		{"bad flag", false, ":s/a/b/gz<cr>", -1, "", "trailing characters: gz"},
		{"substitutions", false, ":%s/a/x/g<cr>", -1, "", "3 substitutions on 2 lines"},
		{"one substitution", false, ":s/a/x/<cr>", -1, "", "1 substitutions on 1 lines"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v, b := New(4), buffer.New("aa\na")
			if tc.dirty {
				run(v, b, "x")
			}
			fx := run(v, b, tc.script)
			if tc.kind >= 0 {
				var found *keymap.Effect
				for i := range fx {
					if fx[i].Kind == tc.kind {
						found = &fx[i]
					}
				}
				if found == nil || found.Path != tc.path {
					t.Errorf("effects %v, want kind %v path %q", fx, tc.kind, tc.path)
				}
			}
			if tc.status != "" && !hasStatus(fx, tc.status) {
				t.Errorf("statuses %q, want %q", statuses(fx), tc.status)
			}
		})
	}
}

func TestQuitClean(t *testing.T) {
	v, b := New(4), buffer.New("a")
	run(v, b, "x:q<cr>")
	if keymap.Has(run(v, b, ""), keymap.KindQuit) {
		t.Error("unexpected quit")
	}
}

func TestSearchMessages(t *testing.T) {
	b := buffer.New("x\na\nx")
	v := New(4)
	fx := run(v, b, "/x<cr>")
	if b.Row != 2 || len(statuses(fx)) != 0 {
		t.Errorf("row %d statuses %q", b.Row, statuses(fx))
	}
	if fx = run(v, b, "n"); !hasStatus(fx, "search hit BOTTOM, continuing at TOP") || b.Row != 0 {
		t.Errorf("wrap forward: row %d statuses %q", b.Row, statuses(fx))
	}
	if fx = run(v, b, "N"); !hasStatus(fx, "search hit TOP, continuing at BOTTOM") || b.Row != 2 {
		t.Errorf("wrap backward: row %d statuses %q", b.Row, statuses(fx))
	}
	fx = run(v, b, "/nope<cr>")
	if !hasStatus(fx, "pattern not found: nope") {
		t.Errorf("not found: %q", statuses(fx))
	}
	fx = run(v, b, "/(<cr>")
	if len(statuses(fx)) != 1 || statuses(fx)[0][:12] != "bad pattern:" {
		t.Errorf("bad pattern: %q", statuses(fx))
	}
	if b.Search.Highlight {
		t.Error("bad pattern left the highlight on")
	}
	// ? wraps with the backward message and n keeps the direction.
	b = buffer.New("x\na\nx")
	fx = run(v, b, "?x<cr>")
	if !hasStatus(fx, "search hit TOP, continuing at BOTTOM") || b.Row != 2 {
		t.Errorf("? wrap: row %d statuses %q", b.Row, statuses(fx))
	}
}

func TestIncrementalSearch(t *testing.T) {
	b := buffer.New("a\nb\nc")
	b.Row = 0
	v := New(4)
	run(v, b, "/c")
	if b.Row != 2 || !b.Search.Highlight || len(b.Search.Matches) != 1 {
		t.Errorf("while typing: row %d highlight %v", b.Row, b.Search.Highlight)
	}
	if text, col, active := v.StatusLine(); text != "/c" || col != 2 || !active {
		t.Errorf("status line = %q %d %v", text, col, active)
	}
	run(v, b, "<esc>")
	if b.Row != 0 || b.Search.Highlight {
		t.Errorf("after esc: row %d highlight %v", b.Row, b.Search.Highlight)
	}
	if _, _, active := v.StatusLine(); active {
		t.Error("prompt still active")
	}
	// esc restores the earlier search.
	run(v, b, "/b<cr>")
	run(v, b, "/c<esc>")
	if b.Search.Pattern != "b" || !b.Search.Highlight {
		t.Errorf("search after esc = %q %v", b.Search.Pattern, b.Search.Highlight)
	}
	// :noh clears the highlight and n brings it back.
	run(v, b, ":noh<cr>")
	if b.Search.Highlight {
		t.Error("noh kept the highlight")
	}
	run(v, b, "n")
	if !b.Search.Highlight {
		t.Error("n did not turn the highlight on")
	}
}

func TestSearchPromptEdit(t *testing.T) {
	b := buffer.New("abc")
	v := New(4)
	run(v, b, "/abx<bs>")
	if text, _, _ := v.StatusLine(); text != "/ab" {
		t.Errorf("text = %q", text)
	}
	run(v, b, "<left>X")
	if text, col, _ := v.StatusLine(); text != "/aXb" || col != 3 {
		t.Errorf("text = %q col %d", text, col)
	}
	run(v, b, "<a-bs>")
	if text, _, _ := v.StatusLine(); text != "/b" {
		t.Errorf("after alt+backspace = %q", text)
	}
	run(v, b, "<del><bs>") // backspace on an empty prompt cancels it
	if _, _, active := v.StatusLine(); active {
		t.Error("empty backspace kept the prompt")
	}
}

func TestSubstituteConfirmPrompt(t *testing.T) {
	b := buffer.New("a a")
	v := New(4)
	run(v, b, ":s/a/x/gc<cr>")
	text, _, active := v.StatusLine()
	if text != "replace with x (y/n/a/q)?" || !active {
		t.Errorf("question = %q %v", text, active)
	}
	if b.Col != 0 {
		t.Errorf("cursor at the match: col %d", b.Col)
	}
	fx := run(v, b, "yy")
	if !hasStatus(fx, "2 substitutions on 1 lines") || b.Text() != "x x" {
		t.Errorf("text %q statuses %q", b.Text(), statuses(fx))
	}
	if _, _, active := v.StatusLine(); active {
		t.Error("question still active")
	}
	// All matches skipped: no match counts as found.
	b = buffer.New("a")
	fx = run(v, b, ":s/a/x/c<cr>n")
	if b.Text() != "a" || len(statuses(fx)) != 0 {
		t.Errorf("skip all: text %q statuses %q", b.Text(), statuses(fx))
	}
}

func TestSubstituteSetsSearch(t *testing.T) {
	b := buffer.New("foo bar foo")
	v := New(4)
	run(v, b, ":s/foo/baz/<cr>")
	if b.Search.Pattern != "foo" || !b.Search.Highlight || len(b.Search.Matches) != 1 {
		t.Errorf("search %q highlight %v matches %d", b.Search.Pattern, b.Search.Highlight,
			len(b.Search.Matches))
	}
	run(v, b, "n")
	if b.Col != 8 {
		t.Errorf("n after :s: col %d", b.Col)
	}
}

func TestUndoSteps(t *testing.T) {
	b := buffer.New("a")
	v := New(4)
	run(v, b, "Abc<left>def<bs><esc>")
	if b.UndoDepth() != 1 {
		t.Errorf("insert session undo steps = %d", b.UndoDepth())
	}
	run(v, b, "u")
	if b.Text() != "a" {
		t.Errorf("undo: %q", b.Text())
	}
	run(v, b, "xx")
	if b.UndoDepth() != 1 { // the second x finds an empty line and does nothing
		t.Errorf("steps after x x = %d", b.UndoDepth())
	}
	b = buffer.New("abc def")
	run(v, b, "cwX<esc>")
	if b.UndoDepth() != 1 {
		t.Errorf("cw steps = %d", b.UndoDepth())
	}
	if b.InChange() {
		t.Error("undo group left open")
	}
}

func TestResetClosesInsert(t *testing.T) {
	b := buffer.New("a")
	v := New(4)
	run(v, b, "Ab")
	if v.Mode() != "insert" || !b.InChange() {
		t.Fatalf("mode %s inChange %v", v.Mode(), b.InChange())
	}
	v.Reset()
	if v.Mode() != "normal" || b.InChange() || b.UndoDepth() != 1 {
		t.Errorf("after reset: mode %s inChange %v steps %d", v.Mode(), b.InChange(), b.UndoDepth())
	}
	// ctrl+g ends the insert as well.
	run(v, b, "Ac")
	fx := run(v, b, "<c-g>")
	if !keymap.Has(fx, keymap.KindSwitchFlavor) || b.InChange() || v.Mode() != "normal" {
		t.Errorf("ctrl+g: effects %v inChange %v mode %s", fx, b.InChange(), v.Mode())
	}
}

func TestModes(t *testing.T) {
	b := buffer.New("abc\ndef")
	v := New(4)
	check := func(script, mode, label string) {
		t.Helper()
		run(v, b, script)
		if v.Mode() != mode || v.ModeLabel() != label {
			t.Errorf("after %q: mode %s label %s, want %s %s", script, v.Mode(), v.ModeLabel(), mode, label)
		}
	}
	check("", "normal", "NORMAL")
	check("i", "insert", "INSERT")
	check("<esc>", "normal", "NORMAL")
	check("v", "visual", "VISUAL")
	check("V", "vline", "V-LINE")
	check("<esc>", "normal", "NORMAL")
	check("R", "replace", "REPLACE")
	check("<esc>", "normal", "NORMAL")
	if v.Hints() != "esc view · :w save · ctrl+g conventional" {
		t.Errorf("hints = %q", v.Hints())
	}
	// esc in visual does not exit to view mode.
	run(v, b, "v")
	if fx := run(v, b, "<esc>"); keymap.Has(fx, keymap.KindExitToView) {
		t.Error("esc in visual exits to view")
	}
}

func TestNormalState(t *testing.T) {
	b := buffer.New("abc")
	v := New(4)
	if !v.Normal() {
		t.Error("start state is not normal")
	}
	for _, s := range []string{"2", "d", "\"", ":", "/", "i", "v"} {
		v.Reset()
		run(v, b, s)
		if v.Normal() {
			t.Errorf("Normal() true after %q", s)
		}
	}
	v.Reset()
	run(v, b, "d<esc>")
	if !v.Normal() {
		t.Error("esc did not clear the pending key")
	}
}

func TestClipboard(t *testing.T) {
	b := buffer.New("foo\nbar")
	v := New(4)
	fx := run(v, b, "yiw")
	if c := clipboards(fx); len(c) != 1 || c[0] != "foo" {
		t.Errorf("yank clipboard = %q", c)
	}
	fx = run(v, b, "dd")
	if c := clipboards(fx); len(c) != 1 || c[0] != "foo\n" {
		t.Errorf("delete clipboard = %q", c)
	}
	if fx = run(v, b, "\"_dd"); len(clipboards(fx)) != 0 {
		t.Errorf("black hole wrote the clipboard: %q", clipboards(fx))
	}

	// "+p asks the terminal, and the answer arrives through Paste.
	b = buffer.New("a\nb")
	fx = run(v, b, "\"+p")
	if !keymap.Has(fx, keymap.KindReadClipboard) {
		t.Fatalf("no clipboard read: %v", fx)
	}
	fx = v.Paste(b, "XY")
	if b.Text() != "aXY\nb" || !keymap.Has(fx, keymap.KindChanged) {
		t.Errorf("charwise paste: %q", b.Text())
	}
	run(v, b, "u\"*P")
	v.Paste(b, "line\n")
	if b.Text() != "line\na\nb" || b.Row != 0 {
		t.Errorf("linewise paste: %q row %d", b.Text(), b.Row)
	}
	if b.UndoDepth() < 1 {
		t.Error("paste has no undo step")
	}
	// Paste in insert mode types the text.
	b = buffer.New("a")
	run(v, b, "A")
	v.Paste(b, "x\ny")
	run(v, b, "<esc>")
	if b.Text() != "ax\ny" {
		t.Errorf("insert paste: %q", b.Text())
	}
}

func clipboards(fx []keymap.Effect) []string {
	var out []string
	for _, e := range fx {
		if e.Kind == keymap.KindSetClipboard {
			out = append(out, e.Text)
		}
	}
	return out
}

func TestRecordingStatus(t *testing.T) {
	b := buffer.New("a")
	v := New(4)
	run(v, b, "qa")
	if text, _, active := v.StatusLine(); text != "recording @a" || !active {
		t.Errorf("status = %q %v", text, active)
	}
	run(v, b, "q")
	if _, _, active := v.StatusLine(); active {
		t.Error("status stays after q")
	}
	// A macro that is not set does nothing.
	run(v, b, "@z")
}

func TestChangedEffect(t *testing.T) {
	b := buffer.New("abc")
	v := New(4)
	if fx := run(v, b, "x"); !keymap.Has(fx, keymap.KindChanged) {
		t.Error("x gave no Changed")
	}
	if fx := run(v, b, "l"); keymap.Has(fx, keymap.KindChanged) {
		t.Error("l gave Changed")
	}
}

func TestSearchHighlightRefresh(t *testing.T) {
	b := buffer.New("ab ab")
	v := New(4)
	run(v, b, "/ab<cr>")
	if len(b.Search.Matches) != 2 {
		t.Fatalf("matches = %d", len(b.Search.Matches))
	}
	run(v, b, "x")
	if len(b.Search.Matches) != 1 {
		t.Errorf("matches after change = %d", len(b.Search.Matches))
	}
}

func TestMarksInVisualAndEx(t *testing.T) {
	b := buffer.New("a\nb\nc\nd")
	v := New(4)
	run(v, b, "jVj")
	run(v, b, ":")
	if text, _, _ := v.StatusLine(); text != ":'<,'>" {
		t.Errorf("visual colon = %q", text)
	}
	run(v, b, "s/./x/<cr>")
	if b.Text() != "a\nx\nx\nd" {
		t.Errorf("text = %q", b.Text())
	}
}

func TestMacroRegisterAndPut(t *testing.T) {
	b := buffer.New("a")
	v := New(4)
	run(v, b, "qaxq")
	if r := v.regs['a']; r.text != "x" {
		t.Errorf("macro register = %q", r.text)
	}
	// A yank into the register replaces the macro.
	b = buffer.New("xb\nc")
	run(v, b, "\"ayl")
	run(v, b, "@a")
	if b.Text() != "b\nc" {
		t.Errorf("register as macro: %q", b.Text())
	}
}

// TestRandomKeys feeds random keys. The engine must not panic and must keep the cursor in
// the text.
func TestRandomKeys(t *testing.T) {
	pool := []string{"d", "c", "y", "w", "b", "e", "j", "k", "h", "l", "i", "a", "o", "x", "p",
		"u", "v", "V", ".", "$", "0", "%", "f", "t", ";", "2", "3", "\"", "a", "q", "@", "m",
		"'", "`", "/", ":", "s", "n", "g", "U", "J", "r", "R", "(", ")", "{", "}", " ", "<esc>",
		"<cr>", "<bs>", "<c-r>", "<up>", "<down>", "<left>", "<right>", "G", "H", "L", "M", ">",
		"<lt>", "~", "*", "#", "I", "A", "O", "D", "C", "S", "Y", "P", "X", "W", "B", "E", "[", "]"}
	seed := uint32(12345)
	next := func() uint32 {
		seed = seed*1664525 + 1013904223
		return seed >> 8
	}
	for round := 0; round < 300; round++ {
		b := buffer.New("foo (bar) \"baz\"\n\n  - item one\nx.y z\n")
		v := New(4)
		script := ""
		for i := 0; i < 60; i++ {
			script += pool[int(next())%len(pool)]
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic %v on %q", r, script)
				}
			}()
			run(v, b, script)
			v.Reset()
			if b.Row < 0 || b.Row >= len(b.Lines) || b.Col < 0 || b.Col > len(b.Lines[b.Row]) {
				t.Fatalf("cursor %d,%d outside the text after %q", b.Row, b.Col, script)
			}
			if b.InChange() {
				t.Fatalf("undo group open after reset: %q", script)
			}
		}()
	}
}

func TestOpenImagePickerEffects(t *testing.T) {
	for _, script := range []string{"<c-p>", ":img<cr>", "i<c-p>", "ihi<c-p>there"} {
		v, b := New(4), buffer.New("aa")
		fx := run(v, b, script)
		if !keymap.Has(fx, keymap.KindOpenImagePicker) {
			t.Errorf("%q: no OpenImagePicker effect", script)
		}
	}
	// Insert mode stays insert, and the key does not type.
	v, b := New(4), buffer.New("aa")
	run(v, b, "i<c-p>")
	if v.ModeLabel() != "INSERT" || b.Text() != "aa" {
		t.Errorf("mode %s text %q", v.ModeLabel(), b.Text())
	}
}
