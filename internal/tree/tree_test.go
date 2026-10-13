package tree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// key makes a key press message from its name.
func key(s string) tea.KeyPressMsg {
	switch s {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "ctrl+n":
		return tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl}
	case "ctrl+h":
		return tea.KeyPressMsg{Code: 'h', Mod: tea.ModCtrl}
	case "f2":
		return tea.KeyPressMsg{Code: tea.KeyF2}
	case "delete":
		return tea.KeyPressMsg{Code: tea.KeyDelete}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

func press(t *Tree, keys ...string) Action {
	var a Action
	for _, k := range keys {
		a, _ = t.HandleKey(key(k))
	}
	return a
}

func typ(t *Tree, s string) {
	for _, r := range s {
		t.HandleKey(key(string(r)))
	}
}

func names(t *Tree) []string {
	var out []string
	for _, r := range t.vis() {
		out = append(out, r.n.name)
	}
	return out
}

func setup(t *testing.T) string {
	d := t.TempDir()
	for _, p := range []string{"b_dir/inner", "A_dir", ".hid"} {
		os.MkdirAll(filepath.Join(d, p), 0o755)
	}
	for _, p := range []string{"zeta.md", "Alpha.md", ".dot", "b_dir/inner/deep.txt", "b_dir/note.md", "A_dir/x.go"} {
		os.WriteFile(filepath.Join(d, p), nil, 0o644)
	}
	return d
}

func eq(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestListingAndHidden(t *testing.T) {
	d := setup(t)
	tr, err := New(d, false)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, names(tr), filepath.Base(d), "A_dir", "b_dir", "Alpha.md", "zeta.md")
	press(tr, "ctrl+h")
	eq(t, names(tr), filepath.Base(d), ".hid", "A_dir", "b_dir", ".dot", "Alpha.md", "zeta.md")
	press(tr, "ctrl+h")
	eq(t, names(tr), filepath.Base(d), "A_dir", "b_dir", "Alpha.md", "zeta.md")
}

func TestExpandCollapse(t *testing.T) {
	d := setup(t)
	tr, _ := New(d, false)
	press(tr, "down", "down", "right") // b_dir
	eq(t, names(tr), filepath.Base(d), "A_dir", "b_dir", "inner", "note.md", "Alpha.md", "zeta.md")
	press(tr, "down", "right")
	if len(tr.vis()) != 8 {
		t.Fatalf("rows %v", names(tr))
	}
	press(tr, "left") // collapse inner
	if len(tr.vis()) != 7 {
		t.Fatal("collapse")
	}
	press(tr, "left") // to parent
	if tr.current().name != "b_dir" {
		t.Fatalf("cursor on %s", tr.current().name)
	}
	press(tr, "left")
	if len(tr.vis()) != 5 {
		t.Fatal("collapse b_dir")
	}
	press(tr, "down")
	a := press(tr, "enter")
	if a.Kind != Open || a.Path != filepath.Join(d, "Alpha.md") {
		t.Fatalf("action %+v", a)
	}
}

func TestLeftOnRoot(t *testing.T) {
	d := setup(t)
	tr, _ := New(d, false)
	press(tr, "left")
	if tr.Root() != filepath.Dir(d) {
		t.Fatalf("root %s", tr.Root())
	}
	if tr.current().path != d || !tr.current().expanded {
		t.Fatal("old root should stay expanded and selected")
	}
	if n := names(tr); n[0] != filepath.Base(filepath.Dir(d)) {
		t.Fatalf("rows %v", n)
	}
}

func TestFilter(t *testing.T) {
	d := setup(t)
	tr, _ := New(d, false)
	press(tr, "/")
	if _, ok := tr.Prompt(); !ok {
		t.Fatal("no prompt")
	}
	typ(tr, "DEEP")
	eq(t, names(tr), filepath.Base(d), "b_dir", "inner", "deep.txt")
	if !strings.HasSuffix(tr.Title(), " · /DEEP") {
		t.Fatal(tr.Title())
	}
	press(tr, "enter")
	if _, ok := tr.Prompt(); ok || tr.filter != "DEEP" {
		t.Fatal("enter keeps the filter")
	}
	press(tr, "down", "down", "down", "down")
	press(tr, "esc")
	if tr.filter != "" {
		t.Fatal("esc clears")
	}
	eq(t, names(tr), filepath.Base(d), "A_dir", "b_dir", "inner", "deep.txt", "note.md", "Alpha.md", "zeta.md")
	if tr.current().name != "deep.txt" {
		t.Fatal("cursor")
	}
	press(tr, "/")
	typ(tr, "zzz")
	press(tr, "esc")
	if tr.filter != "" || len(names(tr)) == 0 {
		t.Fatal("esc in input")
	}
}

func TestCreateRenameDelete(t *testing.T) {
	d := setup(t)
	tr, _ := New(d, false)
	press(tr, "ctrl+n")
	if l, ok := tr.Prompt(); !ok || !strings.HasPrefix(l, "new file: ") {
		t.Fatalf("prompt %q", l)
	}
	typ(tr, "new.md")
	a := press(tr, "enter")
	p := filepath.Join(d, "new.md")
	if a.Kind != Open || a.Path != p {
		t.Fatalf("action %+v", a)
	}
	st, err := os.Stat(p)
	if err != nil || st.Size() != 0 || st.Mode().Perm() != 0o644&^umask() {
		t.Fatalf("file %v %v", st, err)
	}
	if tr.current().path != p {
		t.Fatal("new entry not selected")
	}
	// Exists gives an error.
	press(tr, "ctrl+n")
	typ(tr, "new.md")
	if a := press(tr, "enter"); a.Kind != Error {
		t.Fatalf("action %+v", a)
	}
	// Bad name stays in the prompt.
	press(tr, "ctrl+n")
	typ(tr, "a/b")
	if a := press(tr, "enter"); a.Kind != Error {
		t.Fatal("bad name")
	}
	press(tr, "esc")
	// Rename.
	press(tr, "f2")
	if l, _ := tr.Prompt(); !strings.Contains(l, "new.md") {
		t.Fatalf("prefill %q", l)
	}
	for range "new.md" {
		tr.HandleKey(tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	typ(tr, "ren.md")
	press(tr, "enter")
	q := filepath.Join(d, "ren.md")
	if _, err := os.Stat(q); err != nil || tr.current().path != q {
		t.Fatalf("rename: %v", err)
	}
	// Delete: other key cancels.
	press(tr, "delete")
	if l, ok := tr.Prompt(); !ok || l != "Delete ren.md?  y delete · esc cancel" {
		t.Fatalf("dialog %q", l)
	}
	press(tr, "x")
	if _, err := os.Stat(q); err != nil {
		t.Fatal("cancel deleted")
	}
	press(tr, "delete", "y")
	if _, err := os.Stat(q); !os.IsNotExist(err) {
		t.Fatal("not deleted")
	}
	// New directory in the directory of a file, then delete a directory.
	press(tr, "up", "up", "up", "up") // to b_dir? cursor moves to a dir row
	tr.moveTo(1)                      // A_dir
	press(tr, "delete")
	if l, _ := tr.Prompt(); l != "Delete A_dir and all its content?  y delete · esc cancel" {
		t.Fatalf("dialog %q", l)
	}
	press(tr, "y")
	if _, err := os.Stat(filepath.Join(d, "A_dir")); !os.IsNotExist(err) {
		t.Fatal("dir not deleted")
	}
}

func TestNewDirInFileDir(t *testing.T) {
	d := setup(t)
	tr, _ := New(d, false)
	tr.SetOpen(filepath.Join(d, "b_dir", "note.md"))
	if tr.current().name != "note.md" {
		t.Fatalf("cursor %s", tr.current().name)
	}
	tr.HandleKey(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl | tea.ModShift})
	l, _ := tr.Prompt()
	if l != "new directory: "+tr.input.View() {
		t.Fatal(l)
	}
	if tr.target != filepath.Join(d, "b_dir") {
		t.Fatalf("target %s", tr.target)
	}
}

func TestSetOpenReveals(t *testing.T) {
	d := setup(t)
	tr, _ := New(d, false)
	f := filepath.Join(d, "b_dir", "inner", "deep.txt")
	tr.SetOpen(f)
	eq(t, names(tr), filepath.Base(d), "A_dir", "b_dir", "inner", "deep.txt", "note.md", "Alpha.md", "zeta.md")
	if tr.current().path != f {
		t.Fatal("cursor")
	}
}

func TestViewSize(t *testing.T) {
	d := setup(t)
	tr, _ := New(d, true)
	tr.SetOpen(filepath.Join(d, "Alpha.md"))
	for _, c := range [][2]int{{20, 3}, {1, 10}, {40, 12}, {5, 1}} {
		for _, f := range []bool{true, false} {
			lines := tr.View(c[0], c[1], f)
			if len(lines) != c[1] {
				t.Fatalf("lines %d", len(lines))
			}
			for _, l := range lines {
				if lipgloss.Width(l) != c[0] {
					t.Fatalf("width %d want %d: %q", lipgloss.Width(l), c[0], l)
				}
			}
		}
	}
	tr.Scroll(5)
	tr.Click(0, false)
	if a := tr.Click(0, true); a.Kind != Open && a.Kind != None {
		t.Fatal("click")
	}
}

func TestCursorKeepsOnRefresh(t *testing.T) {
	d := setup(t)
	tr, _ := New(d, false)
	tr.SetOpen(filepath.Join(d, "b_dir", "note.md"))
	os.WriteFile(filepath.Join(d, "b_dir", "aaa.md"), nil, 0o644)
	tr.Refresh()
	if tr.current().name != "note.md" {
		t.Fatal(tr.current().name)
	}
	eq(t, names(tr)[:5], filepath.Base(d), "A_dir", "b_dir", "inner", "aaa.md")
}

// umask returns the process umask.
func umask() os.FileMode {
	old := syscallUmask(0)
	syscallUmask(old)
	return os.FileMode(old)
}

func TestEnterOnExpandedMovesToChild(t *testing.T) {
	d := setup(t)
	tr, _ := New(d, false)
	if a := press(tr, "enter"); a.Kind != None || tr.current().name != "A_dir" {
		t.Fatalf("root: %+v on %s", a, tr.current().name)
	}
	press(tr, "down", "right") // b_dir, expand
	press(tr, "enter")
	if tr.current().name != "inner" {
		t.Fatalf("cursor on %s", tr.current().name)
	}
	empty := filepath.Join(d, "A_dir", "empty")
	os.Mkdir(empty, 0o755)
	tr.Refresh()
	tr.reveal(empty)
	tr.cursorPath = empty
	tr.vis()
	press(tr, "enter") // expand the empty directory
	press(tr, "enter")
	if tr.current().name != "empty" {
		t.Fatalf("cursor on %s", tr.current().name)
	}
}

func TestNoMatchesRow(t *testing.T) {
	d := setup(t)
	tr, _ := New(d, false)
	press(tr, "/")
	typ(tr, "zzzz")
	out := tr.View(30, 4, true)
	if !strings.Contains(out[1], "no matches") || strings.Contains(out[0], "no matches") {
		t.Fatalf("view %q", out)
	}
	typ(tr, "\b")
	press(tr, "esc")
	if strings.Contains(strings.Join(tr.View(30, 4, true), ""), "no matches") {
		t.Fatal("note without filter")
	}
}
