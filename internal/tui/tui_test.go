package tui

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/biomassa/mded/internal/buffer"
	"github.com/biomassa/mded/internal/config"
	"github.com/biomassa/mded/internal/keymap"
	"github.com/biomassa/mded/internal/kitty"
)

// keyOf makes a key press from a key string such as "a", "ctrl+s", or "shift+tab".
func keyOf(s string) tea.KeyPressMsg {
	var mod tea.KeyMod
	for {
		switch {
		case strings.HasPrefix(s, "ctrl+") && len(s) > 5:
			mod |= tea.ModCtrl
			s = s[5:]
			continue
		case strings.HasPrefix(s, "alt+") && len(s) > 4:
			mod |= tea.ModAlt
			s = s[4:]
			continue
		case strings.HasPrefix(s, "shift+") && len(s) > 6:
			mod |= tea.ModShift
			s = s[6:]
			continue
		}
		break
	}
	names := map[string]rune{
		"enter": tea.KeyEnter, "esc": tea.KeyEscape, "tab": tea.KeyTab, "backspace": tea.KeyBackspace,
		"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
		"home": tea.KeyHome, "end": tea.KeyEnd, "pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown,
		"f1": tea.KeyF1, "delete": tea.KeyDelete, "space": tea.KeySpace,
	}
	if c, ok := names[s]; ok {
		k := tea.KeyPressMsg{Code: c, Mod: mod}
		if s == "space" && mod == 0 {
			k.Text = " "
		}
		return k
	}
	r := []rune(s)[0]
	k := tea.KeyPressMsg{Code: r, Mod: mod}
	if mod&^tea.ModShift == 0 {
		k.Text = s
	}
	return k
}

// testModel opens path in a model of 80x24 cells. The config goes to a temporary directory.
func testModel(t *testing.T, path string) *Model {
	t.Helper()
	m := testModelSize(t, path, 140, 24)
	return m
}

// testModelSize is testModel with a screen size.
func testModelSize(t *testing.T, path string, w, h int) *Model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := New(Options{
		Config:           config.Defaults(),
		Path:             path,
		SaveTheme:        func(string) error { return nil },
		SaveSidebarWidth: func(int) error { return nil },
		OpenURL:          func(string) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

// press sends keys to the model.
func press(m *Model, keys ...string) {
	for _, k := range keys {
		m.Update(keyOf(k))
	}
}

// typeText sends each rune of s as a key press.
func typeText(m *Model, s string) {
	for _, r := range s {
		press(m, string(r))
	}
}

// screenText returns the screen without styles.
func screenText(m *Model) string { return ansi.Strip(m.View().Content) }

func writeTemp(t *testing.T, name, content string, perm os.FileMode) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), perm); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestKeyNames(t *testing.T) {
	for _, tc := range []struct {
		msg  tea.KeyPressMsg
		want string
	}{
		{tea.KeyPressMsg{Code: '`', Mod: tea.ModCtrl}, "ctrl+`"},
		{tea.KeyPressMsg{Code: tea.KeySpace, Mod: tea.ModCtrl}, "ctrl+space"},
		{keyOf("T"), "T"},
		{keyOf("?"), "?"},
		{keyOf("shift+tab"), "shift+tab"},
		{keyOf("f1"), "f1"},
		{keyOf("space"), "space"},
		{keyOf("alt+h"), "alt+h"},
		{keyOf("ctrl+w"), "ctrl+w"},
	} {
		if got := tc.msg.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
	for _, name := range []string{"ctrl+`", "ctrl+space", "ctrl+@"} {
		if !sidebarKeys[name] {
			t.Errorf("%s is not a sidebar key", name)
		}
	}
}

func TestOpenSaveRoundTrip(t *testing.T) {
	// CRLF, no trailing newline, mode 0600.
	p := writeTemp(t, "a.md", "- [ ] one\r\n- [ ] two\r\ntail", 0o600)
	if err := os.Chmod(p, 0o600); err != nil {
		t.Fatal(err)
	}
	m := testModel(t, p)
	if m.doc.buf.Dirty() || !m.doc.crlf || m.doc.finalNL {
		t.Fatalf("after open: dirty=%v crlf=%v finalNL=%v", m.doc.buf.Dirty(), m.doc.crlf, m.doc.finalNL)
	}
	press(m, "tab", "space") // select the first checkbox, toggle it
	if !m.doc.buf.Dirty() {
		t.Fatal("the toggle did not modify the document")
	}
	press(m, "ctrl+s")
	got, _ := os.ReadFile(p)
	if string(got) != "- [x] one\r\n- [ ] two\r\ntail" {
		t.Fatalf("saved %q", got)
	}
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", st.Mode().Perm())
	}
	if m.msg != "saved "+p {
		t.Errorf("message %q", m.msg)
	}
	// LF with a trailing newline.
	q := writeTemp(t, "b.md", "x\ny\n", 0o644)
	m = testModel(t, q)
	press(m, "e")
	typeText(m, "z")
	press(m, "ctrl+s")
	got, _ = os.ReadFile(q)
	if string(got) != "zx\ny\n" {
		t.Fatalf("saved %q", got)
	}
	// No temporary file stays behind.
	ents, _ := os.ReadDir(filepath.Dir(q))
	if len(ents) != 1 {
		t.Errorf("directory has %d entries", len(ents))
	}
}

func TestNewFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "new.md")
	m := testModel(t, p)
	press(m, "e")
	typeText(m, "hi")
	press(m, "ctrl+s")
	got, err := os.ReadFile(p)
	if err != nil || string(got) != "hi\n" {
		t.Fatalf("saved %q, %v", got, err)
	}
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o644 {
		t.Errorf("mode %v, want 0644", st.Mode().Perm())
	}
}

func TestBinaryFile(t *testing.T) {
	bin := writeTemp(t, "x.bin", "ab\x00cd", 0o644)
	md := writeTemp(t, "a.md", "text\n", 0o644)
	m := testModel(t, md)
	m.openFromTree(bin)
	if m.msg != "binary file: x.bin" || !m.msgErr {
		t.Errorf("message %q", m.msg)
	}
	if m.doc.path != md {
		t.Error("a binary file replaced the document")
	}
	if _, err := loadDoc(bin); err == nil {
		t.Error("loadDoc accepted a binary file")
	}
}

func TestUnsavedDialog(t *testing.T) {
	p := writeTemp(t, "a.md", "- [ ] one\n", 0o644)
	m := testModel(t, p)
	press(m, "tab", "space", "q")
	if m.dlg == nil {
		t.Fatal("no dialog")
	}
	if line := ansi.Strip(m.statusLine()); !strings.Contains(line, "Save changes to a.md?") || !strings.Contains(line, "s save · d discard · esc cancel") {
		t.Errorf("status line %q", line)
	}
	press(m, "esc")
	if m.dlg != nil || m.quitting {
		t.Fatal("esc did not cancel")
	}
	press(m, "q", "d")
	if !m.quitting {
		t.Fatal("d did not quit")
	}
	if got, _ := os.ReadFile(p); string(got) != "- [ ] one\n" {
		t.Errorf("discard wrote the file: %q", got)
	}
	m = testModel(t, p)
	press(m, "tab", "space", "ctrl+q", "s")
	if !m.quitting {
		t.Fatal("s did not quit")
	}
	if got, _ := os.ReadFile(p); string(got) != "- [x] one\n" {
		t.Errorf("save: %q", got)
	}
}

func TestOpenAnotherFileAsks(t *testing.T) {
	a := writeTemp(t, "a.md", "- [ ] one\n", 0o644)
	b := writeTemp(t, "b.md", "other\n", 0o644)
	m := testModel(t, a)
	press(m, "tab", "space")
	m.openFromTree(b)
	if m.dlg == nil || m.doc.path != a {
		t.Fatal("no dialog before the switch")
	}
	press(m, "d")
	if m.doc.path != b {
		t.Fatalf("open after discard: %q", m.doc.path)
	}
}

func TestViewSearch(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 60; i++ {
		sb.WriteString("line\n")
	}
	sb.WriteString("Needle here\n")
	for i := 0; i < 10; i++ {
		sb.WriteString("line\n")
	}
	sb.WriteString("needle again\n")
	p := writeTemp(t, "a.md", sb.String(), 0o644)
	m := testModel(t, p)
	press(m, "/")
	typeText(m, "needle")
	if m.doc.off == 0 {
		t.Error("the incremental search did not scroll")
	}
	if b := m.doc.buf; b.Row != 60 || len(b.Search.Matches) != 2 {
		t.Errorf("row %d, matches %d", b.Row, len(b.Search.Matches))
	}
	if !strings.Contains(ansi.Strip(m.statusLine()), "/needle") {
		t.Errorf("status line %q", ansi.Strip(m.statusLine()))
	}
	press(m, "esc")
	if m.doc.off != 0 || m.doc.buf.Search.Highlight || m.search != nil {
		t.Error("esc did not restore")
	}
	press(m, "/")
	typeText(m, "needle")
	press(m, "enter")
	if m.search != nil || !m.doc.buf.Search.Highlight {
		t.Fatal("enter did not keep the highlight")
	}
	press(m, "n")
	if m.doc.buf.Row != 71 {
		t.Errorf("n went to row %d", m.doc.buf.Row)
	}
	press(m, "n")
	if m.doc.buf.Row != 60 || m.msg != "search hit BOTTOM, continuing at TOP" {
		t.Errorf("wrap: row %d message %q", m.doc.buf.Row, m.msg)
	}
	press(m, "N")
	if m.doc.buf.Row != 71 || m.msg != "search hit TOP, continuing at BOTTOM" {
		t.Errorf("N: row %d message %q", m.doc.buf.Row, m.msg)
	}
	press(m, "esc")
	if m.doc.buf.Search.Highlight {
		t.Error("esc did not clear the highlight")
	}
	press(m, "/")
	typeText(m, "zzz")
	if !strings.Contains(ansi.Strip(m.statusLine()), "pattern not found: zzz") {
		t.Errorf("status line %q", ansi.Strip(m.statusLine()))
	}
	press(m, "enter")
	if m.msg != "pattern not found: zzz" {
		t.Errorf("message %q", m.msg)
	}
	press(m, "/")
	typeText(m, "(")
	if !strings.Contains(ansi.Strip(m.statusLine()), "bad pattern: ") {
		t.Errorf("status line %q", ansi.Strip(m.statusLine()))
	}
	press(m, "esc", "/", "up")
	if string(m.search.text) != "zzz" {
		t.Errorf("history gave %q", string(m.search.text))
	}
}

// enter after a search edits the document at the current match. Without the highlight,
// enter does not edit.
func TestEnterEditsAtMatch(t *testing.T) {
	p := writeTemp(t, "a.md", "one\ntwo words here\nthree\n", 0o644)
	m := testModel(t, p)
	press(m, "enter")
	if m.mode == modeEdit {
		t.Fatal("enter with no search opened edit mode")
	}
	press(m, "/")
	typeText(m, "words")
	press(m, "enter")
	if m.mode == modeEdit {
		t.Fatal("the prompt enter opened edit mode")
	}
	press(m, "enter")
	if b := m.doc.buf; m.mode != modeEdit || b.Row != 1 || b.Col != 4 {
		t.Errorf("mode %v, cursor %d:%d", m.mode, b.Row, b.Col)
	}
	if !m.doc.buf.Search.Highlight {
		t.Error("the highlight went off")
	}
}

func TestSearchShowsSourceOnlyForMatches(t *testing.T) {
	p := writeTemp(t, "a.md", "# Title\n\nsome **bold** text\nother **bold** line\n", 0o644)
	m := testModel(t, p)
	press(m, "/")
	typeText(m, "some")
	press(m, "enter")
	s := screenText(m)
	if !strings.Contains(s, "some **bold** text") || strings.Contains(s, "other **bold**") {
		t.Errorf("screen:\n%s", s)
	}
}

func TestItemsTabAndToggle(t *testing.T) {
	p := writeTemp(t, "a.md", "- [ ] one\n\n[link](other.md)\n\n- [x] two\n", 0o644)
	m := testModel(t, p)
	press(m, "tab")
	it, ok := m.selectedItem()
	if !ok || it.Kind != 0 || it.Line != 0 {
		t.Fatalf("first tab: %+v %v", it, ok)
	}
	press(m, "tab")
	if it, _ = m.selectedItem(); it.Line != 2 {
		t.Errorf("second tab: %+v", it)
	}
	press(m, "tab", "tab")
	if it, _ = m.selectedItem(); it.Line != 0 {
		t.Errorf("wrap: %+v", it)
	}
	press(m, "shift+tab")
	if it, _ = m.selectedItem(); it.Line != 4 {
		t.Errorf("shift+tab: %+v", it)
	}
	press(m, "space")
	if got := string(m.doc.buf.Lines[4]); got != "- [ ] two" {
		t.Errorf("toggle: %q", got)
	}
	// The highlight must not leak into the cache: after esc the first row has no reverse.
	press(m, "tab", "esc")
	if m.itemSel != -1 {
		t.Error("esc kept the selection")
	}
	// Space without a selected checkbox pages down.
	press(m, "space")
	if m.doc.buf.Lines[4][3] != ' ' {
		t.Error("space toggled without a selection")
	}
}

func TestSidebarKeys(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.md")
	os.WriteFile(p, []byte("x\n"), 0o644)
	m := testModel(t, p)
	if m.showTree || m.focus != focusDoc {
		t.Fatal("start with a file: the sidebar must be hidden")
	}
	for _, name := range []string{"ctrl+`", "ctrl+space", "ctrl+@"} {
		m.showTree = false
		m.Update(keyOfNamed(name))
		if !m.showTree || m.focus != focusDoc {
			t.Errorf("%s: show=%v focus=%v", name, m.showTree, m.focus)
		}
	}
	press(m, "alt+h")
	if m.focus != focusTree {
		t.Fatal("alt+h")
	}
	m.Update(keyOfNamed("ctrl+`")) // hide while focused
	if m.showTree || m.focus != focusDoc {
		t.Errorf("hide: show=%v focus=%v", m.showTree, m.focus)
	}
	press(m, "ctrl+e") // shows and focuses the sidebar
	if m.focus != focusTree || !m.showTree {
		t.Errorf("ctrl+e: focus=%v", m.focus)
	}
	press(m, "ctrl+e")
	if m.focus != focusDoc {
		t.Error("ctrl+e back")
	}
	press(m, "alt+l")
	if m.focus != focusDoc {
		t.Error("alt+l")
	}
	// The sidebar shows the directory of the file, and the screen has both panes.
	m.focusPane(focusTree)
	if s := screenText(m); !strings.Contains(s, "a.md") {
		t.Errorf("screen:\n%s", s)
	}
	// A key in the sidebar goes to the tree, not to the document: "/" opens the filter.
	press(m, "/")
	if _, on := m.tree.Prompt(); !on {
		t.Error("the tree filter did not open")
	}
	if !strings.Contains(ansi.Strip(m.statusLine()), "/") {
		t.Error("status line does not show the tree prompt")
	}
}

// keyOfNamed makes the key press of a sidebar toggle name.
func keyOfNamed(name string) tea.KeyPressMsg {
	switch name {
	case "ctrl+space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Mod: tea.ModCtrl}
	case "ctrl+@":
		return tea.KeyPressMsg{Code: '@', Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: '`', Mod: tea.ModCtrl}
}

func TestStartWithDirectory(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("x\n"), 0o644)
	m := testModel(t, dir)
	if !m.showTree || m.focus != focusTree {
		t.Fatal("a directory start shows the sidebar with the focus")
	}
	s := screenText(m)
	if !strings.Contains(s, emptyHint) || !strings.Contains(s, "[new]") {
		t.Errorf("screen:\n%s", s)
	}
}

func TestLinkFollowAndBack(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.md")
	b := filepath.Join(dir, "b.md")
	var sb strings.Builder
	sb.WriteString("# B\n")
	for i := 0; i < 40; i++ {
		sb.WriteString("filler\n")
	}
	sb.WriteString("## Second Part\n\nend\n")
	os.WriteFile(a, []byte("[go](b.md#second-part)\n\n[web](https://example.com)\n\n[here](#top)\n\n# Top\n"), 0o644)
	os.WriteFile(b, []byte(sb.String()), 0o644)
	var opened string
	m := testModel(t, a)
	m.opts.OpenURL = func(u string) error { opened = u; return nil }
	press(m, "tab", "enter")
	if m.doc.path != b {
		t.Fatalf("doc is %q", m.doc.path)
	}
	if i := m.firstRowOf(41); i < m.doc.off || i >= m.doc.off+m.textH() {
		t.Errorf("the heading row %d is not in view (off %d)", i, m.doc.off)
	}
	press(m, "backspace")
	if m.doc.path != a || m.doc.off != 0 {
		t.Fatalf("back: %q off %d", m.doc.path, m.doc.off)
	}
	press(m, "tab", "tab", "enter")
	if opened != "https://example.com" || m.msg != "opened https://example.com" {
		t.Errorf("opened %q, message %q", opened, m.msg)
	}
	press(m, "tab", "enter") // #top in the same document
	if m.doc.path != a || m.msg == "no heading: #top" {
		t.Errorf("anchor: %q", m.msg)
	}
	press(m, "backspace")
	if m.msg != "no previous document" {
		t.Errorf("message %q", m.msg)
	}
}

// fakeVim is a keymap that stands for the vim engine in the tests.
type fakeVim struct {
	normal   bool
	resets   int
	visual   [2]int // the anchor of the last StartVisual
	visualOn bool
}

func (f *fakeVim) Handle(b *buffer.Buffer, k keymap.Key, w, rows int) []keymap.Effect {
	if k.Name == "esc" {
		return []keymap.Effect{keymap.ExitToView()}
	}
	return nil
}
func (f *fakeVim) Paste(*buffer.Buffer, string) []keymap.Effect { return nil }
func (f *fakeVim) ModeLabel() string                            { return "NORMAL" }
func (f *fakeVim) Hints() string                                { return "esc view · :w save · ctrl+g conventional" }
func (f *fakeVim) StatusLine() (string, int, bool)              { return "", 0, false }
func (f *fakeVim) Reset()                                       { f.resets++ }
func (f *fakeVim) Normal() bool                                 { return f.normal }
func (f *fakeVim) StartVisual(b *buffer.Buffer, row, col int) {
	f.visual = [2]int{row, col}
	f.visualOn = true
}
func (f *fakeVim) LeaveVisual(*buffer.Buffer) { f.visualOn = false }

func TestFlavorSwitch(t *testing.T) {
	p := writeTemp(t, "a.md", "x\n", 0o644)
	fv := &fakeVim{normal: true}
	m := testModel(t, p)
	m.opts.NewVim = func(int) keymap.Keymap { return fv }
	press(m, "ctrl+g")
	if !m.vimFlavor || m.msg != "edit mode: vim" {
		t.Errorf("vim=%v message %q", m.vimFlavor, m.msg)
	}
	press(m, "e")
	m.msg = ""
	if m.km != keymap.Keymap(fv) || !strings.Contains(ansi.Strip(m.statusLine()), "NORMAL") {
		t.Error("e did not open the vim keymap")
	}
	if got := ansi.Strip(m.statusLine()); !strings.Contains(got, "esc view · :w save · ctrl+g conventional") {
		t.Errorf("status line %q", got)
	}
	resets := fv.resets
	press(m, "ctrl+g")
	if m.vimFlavor || m.msg != "edit mode: conventional" || fv.resets <= resets {
		t.Errorf("vim=%v message %q resets %d", m.vimFlavor, m.msg, fv.resets)
	}
	if m.mode != modeEdit || m.km.ModeLabel() != "EDIT" {
		t.Error("the switch must keep edit mode with the conventional keymap")
	}
	// ctrl+e switches the focus in all edit modes.
	press(m, "ctrl+g")
	fv.normal = false
	press(m, "ctrl+e")
	if m.focus != focusTree {
		t.Error("ctrl+e in vim insert mode")
	}
	press(m, "alt+l")
	press(m, "esc")
	if m.mode != modeView {
		t.Error("esc did not leave edit mode")
	}
	press(m, "ctrl+g")
	if m.vimFlavor {
		t.Error("ctrl+g in view mode")
	}
}

func TestEditCursorIsKept(t *testing.T) {
	p := writeTemp(t, "a.md", "one\ntwo\nthree\n", 0o644)
	m := testModel(t, p)
	press(m, "e", "down", "down", "right", "esc")
	if m.mode != modeView {
		t.Fatal("not in view mode")
	}
	press(m, "/")
	typeText(m, "one")
	press(m, "enter", "e")
	if b := m.doc.buf; b.Row != 2 || b.Col != 1 {
		t.Errorf("cursor %d:%d, want 2:1", b.Row, b.Col)
	}
}

func TestEditModeFindBox(t *testing.T) {
	p := writeTemp(t, "a.md", "## head\nfoo bar foo\n", 0o644)
	m := testModel(t, p)
	press(m, "e", "ctrl+f")
	if m.find == nil {
		t.Fatal("no find box")
	}
	typeText(m, "foo")
	s := screenText(m)
	if !strings.Contains(s, "Find · 1 of 2") || !strings.Contains(s, "## head") {
		t.Errorf("screen:\n%s", s)
	}
	press(m, "esc")
	if m.find != nil || m.mode != modeEdit {
		t.Error("esc must close only the box")
	}
}

func TestThemePicker(t *testing.T) {
	p := writeTemp(t, "a.md", "x\n", 0o644)
	var saved string
	m := testModel(t, p)
	m.opts.SaveTheme = func(n string) error { saved = n; return nil }
	defer m.previewTheme("default")
	press(m, "T")
	if m.themes == nil {
		t.Fatal("no picker")
	}
	press(m, "down")
	if !strings.Contains(screenText(m), "↑/↓ preview · enter keep · esc back") {
		t.Error("no hint")
	}
	press(m, "esc")
	if m.themes != nil || m.msg != "theme default" {
		t.Errorf("esc: message %q", m.msg)
	}
	press(m, "T", "down")
	m.themeKey("enter") // direct call: Update would hide the command in a batch
	if m.themes != nil || len(m.cmds) != 1 {
		t.Fatalf("enter: picker %v, cmds %d", m.themes, len(m.cmds))
	}
	msg, ok := m.cmds[0]().(themeSavedMsg)
	if !ok || msg.name != "catppuccin-mocha" || saved != "catppuccin-mocha" {
		t.Errorf("save: %+v, saved %q", msg, saved)
	}
}

func TestHelpOverlay(t *testing.T) {
	p := writeTemp(t, "a.md", "x\n", 0o644)
	m := testModel(t, p)
	press(m, "?")
	s := screenText(m)
	if m.help == nil || !strings.Contains(s, "Keys") || !strings.Contains(s, "scroll one row") {
		t.Errorf("screen:\n%s", s)
	}
	press(m, "j", "esc")
	if m.help != nil {
		t.Error("esc did not close")
	}
	press(m, "f1", "f1")
	if m.help != nil {
		t.Error("F1 did not toggle")
	}
}

func TestStatusLineHints(t *testing.T) {
	p := writeTemp(t, "a.md", "x\n", 0o644)
	m := testModel(t, p)
	m.msg = ""
	line := ansi.Strip(m.statusLine())
	if !strings.Contains(line, "VIEW") || !strings.Contains(line, viewHints) || !strings.Contains(line, "100%") {
		t.Errorf("status line %q", line)
	}
	press(m, "e")
	m.msg = ""
	if line := ansi.Strip(m.statusLine()); !strings.Contains(line, "EDIT") || !strings.Contains(line, "esc view · ctrl+s save · ctrl+f find · ctrl+g vim") || !strings.Contains(line, "1:1") {
		t.Errorf("status line %q", line)
	}
	typeText(m, "a")
	if line := ansi.Strip(m.statusLine()); !strings.Contains(line, "[+]") {
		t.Errorf("status line %q", line)
	}
}

func TestImageRows(t *testing.T) {
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 16, 32))
	for x := 0; x < 16; x++ {
		for y := 0; y < 32; y++ {
			img.Set(x, y, color.RGBA{R: 200, A: 255})
		}
	}
	f, _ := os.Create(filepath.Join(dir, "p.png"))
	png.Encode(f, img)
	f.Close()
	md := filepath.Join(dir, "a.md")
	os.WriteFile(md, []byte("before\n\n![pic](p.png)\n\nafter ![x](https://e.com/x.png)\n"), 0o644)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	mgr := kitty.NewManager(8, 16)
	m, err := New(Options{Config: config.Defaults(), Path: md, Images: mgr})
	if err != nil {
		t.Fatal(err)
	}
	_, load := m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	if !strings.Contains(screenText(m), "🖼️ pic") {
		t.Errorf("a loading image shows the alt text:\n%s", screenText(m))
	}
	if !strings.Contains(screenText(m), "🖼️ x") {
		t.Error("a remote image shows the alt text")
	}
	if load == nil {
		t.Fatal("no load command")
	}
	_, cmd := m.Update(load())
	if cmd == nil {
		t.Fatal("no place command")
	}
	n := 0
	for _, r := range m.rows {
		if r.Img != nil {
			n++
		}
	}
	if n != 2 || len(m.placed) != 2 {
		t.Fatalf("image rows %d, placed %d", n, len(m.placed))
	}
	if strings.Contains(screenText(m), "🖼️ pic") {
		t.Error("the image line still shows the alt text")
	}
	if !strings.ContainsRune(screenText(m), '\U0010EEEE') {
		t.Error("no placeholder cells")
	}
	if m.Cleanup() == "" {
		t.Error("no delete sequence")
	}
}

func TestImageViewer(t *testing.T) {
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 800, 1600))
	p := filepath.Join(dir, "p.png")
	f, _ := os.Create(p)
	png.Encode(f, img)
	f.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	mgr := kitty.NewManager(8, 16)
	m, err := New(Options{Config: config.Defaults(), Path: p, Images: mgr})
	if err != nil {
		t.Fatal(err)
	}
	_, load := m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	m.Update(load())
	if len(m.rows) != m.textH() || !m.doc.image {
		t.Errorf("rows %d, text height %d", len(m.rows), m.textH())
	}
	press(m, "e")
	if m.mode != modeView {
		t.Error("an image opened in edit mode")
	}
}

func TestBorderDragSavesWidth(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("x\n"), 0o644)
	m := testModel(t, dir)
	w := m.treeW()
	m.Update(tea.MouseClickMsg{X: w - 1, Y: 5, Button: tea.MouseLeft})
	if !m.dragBorder {
		t.Fatal("no drag")
	}
	m.Update(tea.MouseMotionMsg{X: 40, Y: 5, Button: tea.MouseLeft})
	if m.treeW() != 41 {
		t.Errorf("width %d", m.treeW())
	}
	_, cmd := m.Update(tea.MouseReleaseMsg{X: 40, Y: 5, Button: tea.MouseLeft})
	if cmd == nil || m.dragBorder {
		t.Error("no save command after the release")
	}
}

func TestViewClickOnCheckbox(t *testing.T) {
	p := writeTemp(t, "a.md", "- [ ] one\n", 0o644)
	m := testModel(t, p)
	// The checkbox cell: border, margin, then the marker.
	x, y := 1+1+0, 1
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	if got := string(m.doc.buf.Lines[0]); got != "- [x] one" {
		t.Errorf("line %q", got)
	}
}

func TestPlainFile(t *testing.T) {
	p := writeTemp(t, "a.txt", "# not a heading\n**plain**\n", 0o644)
	m := testModel(t, p)
	s := screenText(m)
	if !strings.Contains(s, "# not a heading") || !strings.Contains(s, "**plain**") {
		t.Errorf("screen:\n%s", s)
	}
}

func TestEmptyPaneHasNoEdit(t *testing.T) {
	dir := t.TempDir()
	m := testModel(t, dir)
	m.focus = focusDoc
	press(m, "e", "i")
	if m.mode != modeView {
		t.Fatal("edit mode in a document without a file")
	}
	m.Update(tea.MouseClickMsg{X: 60, Y: 3, Button: tea.MouseLeft})
	m.Update(tea.MouseClickMsg{X: 60, Y: 3, Button: tea.MouseLeft})
	if m.mode != modeView {
		t.Fatal("double-click opened edit mode")
	}
	// A missing file on the command line still opens as a new, editable document.
	m = testModel(t, filepath.Join(dir, "new.md"))
	press(m, "e")
	if m.mode != modeEdit {
		t.Fatal("a new file must be editable")
	}
}

func TestNoSearchPatternMessage(t *testing.T) {
	m := testModel(t, writeTemp(t, "a.md", "x\n", 0o644))
	press(m, "n")
	if m.msg != "no previous regular expression" {
		t.Errorf("message %q", m.msg)
	}
}

func TestDoubleClickOnCheckbox(t *testing.T) {
	p := writeTemp(t, "a.md", "- [ ] one\n- [ ] two\n", 0o644)
	m := testModel(t, p)
	click := func(y int) { m.Update(tea.MouseClickMsg{X: 2, Y: y, Button: tea.MouseLeft}) }
	click(1)
	click(1)
	if got := m.doc.buf.Text(); got != "- [ ] one\n- [ ] two" || m.doc.buf.Dirty() {
		t.Errorf("text %q dirty %v", got, m.doc.buf.Dirty())
	}
	if m.mode != modeEdit || m.doc.buf.Row != 0 {
		t.Errorf("mode %v row %d", m.mode, m.doc.buf.Row)
	}
	if m.doc.buf.UndoDepth() != 0 || m.doc.buf.RedoDepth() != 0 {
		t.Error("the revert must leave no undo or redo step")
	}
	press(m, "esc")
	// With a change before: only the toggle of the double-click goes back.
	click(2)
	time.Sleep(2 * time.Millisecond)
	m.lastClick = clickInfo{}
	click(1)
	click(1)
	if got := m.doc.buf.Text(); got != "- [ ] one\n- [x] two" || !m.doc.buf.Dirty() {
		t.Errorf("text %q dirty %v", got, m.doc.buf.Dirty())
	}
	if m.mode != modeEdit {
		t.Error("no edit mode")
	}
}

func TestDoubleClickOnLink(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.md")
	b := filepath.Join(dir, "b.md")
	os.WriteFile(a, []byte("[go](b.md)\n"), 0o644)
	os.WriteFile(b, []byte("text\n"), 0o644)
	m := testModel(t, a)
	m.Update(tea.MouseClickMsg{X: 2, Y: 1, Button: tea.MouseLeft})
	m.Update(tea.MouseClickMsg{X: 2, Y: 1, Button: tea.MouseLeft})
	if m.doc.path != b || m.mode != modeView {
		t.Errorf("doc %q mode %v", m.doc.path, m.mode)
	}
}

func TestVimMouseDrag(t *testing.T) {
	p := writeTemp(t, "a.md", "abcdef\n", 0o644)
	fv := &fakeVim{normal: true}
	m := testModel(t, p)
	m.opts.NewVim = func(int) keymap.Keymap { return fv }
	press(m, "ctrl+g", "e")
	m.Update(tea.MouseClickMsg{X: 4, Y: 1, Button: tea.MouseLeft})
	if fv.visualOn || m.doc.buf.SelOn {
		t.Fatal("a click must not select")
	}
	m.Update(tea.MouseMotionMsg{X: 7, Y: 1, Button: tea.MouseLeft})
	if !fv.visualOn || fv.visual != [2]int{0, m.doc.buf.SelCol} || m.doc.buf.SelCol == m.doc.buf.Col {
		t.Fatalf("visual %v on %v, anchor %d cursor %d", fv.visual, fv.visualOn, m.doc.buf.SelCol, m.doc.buf.Col)
	}
	m.Update(tea.MouseReleaseMsg{X: 7, Y: 1, Button: tea.MouseLeft})
	m.Update(tea.MouseClickMsg{X: 5, Y: 1, Button: tea.MouseLeft})
	if fv.visualOn {
		t.Error("a click must leave visual mode")
	}
}
