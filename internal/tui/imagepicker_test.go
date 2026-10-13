package tui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/biomassa/mded/internal/config"
	"github.com/biomassa/mded/internal/keymap"
	"github.com/biomassa/mded/internal/keymap/vim"
	"github.com/biomassa/mded/internal/kitty"
)

// pickerFixture makes a directory with a document, images, directories, and hidden entries.
func pickerFixture(t *testing.T) (dir, md string) {
	t.Helper()
	dir = t.TempDir()
	for _, d := range []string{"Zeta", "alpha", ".hid", "sub dir"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"b.jpg", "A.GIF", "My Pic.PNG", ".h.png", "c.webp", "n.txt", "d.jpeg"} {
		os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644)
	}
	md = filepath.Join(dir, "doc.md")
	os.WriteFile(md, []byte("one\ntwo\nthree"), 0o644)
	return dir, md
}

func pickerNames(m *Model) []string {
	var out []string
	for _, e := range m.img.rows() {
		out = append(out, e.name)
	}
	return out
}

func equal(a, b []string) bool { return strings.Join(a, "|") == strings.Join(b, "|") }

func TestPickerListing(t *testing.T) {
	_, md := pickerFixture(t)
	m := testModel(t, md)
	press(m, "e", "ctrl+p")
	if m.img == nil {
		t.Fatal("picker not open")
	}
	want := []string{"../", "alpha", "sub dir", "Zeta", "A.GIF", "b.jpg", "c.webp", "d.jpeg", "My Pic.PNG"}
	if got := pickerNames(m); !equal(got, want) {
		t.Errorf("rows %v, want %v", got, want)
	}
	s := screenText(m)
	for _, w := range []string{"Insert image", "↑/↓ move · → open · ← up · type to filter · enter pick · esc cancel"} {
		if !strings.Contains(s, w) {
			t.Errorf("screen lacks %q", w)
		}
	}
	// Filter: case-insensitive substring; the parent row stays first.
	typeText(m, "PIC")
	if got := pickerNames(m); !equal(got, []string{"../", "My Pic.PNG"}) {
		t.Errorf("filtered %v", got)
	}
	press(m, "backspace")
	if string(m.img.filter) != "PI" {
		t.Errorf("filter %q", string(m.img.filter))
	}
	// Entering a directory resets the filter.
	press(m, "backspace", "backspace")
	typeText(m, "zet")
	press(m, "enter")
	if filepath.Base(m.img.dir) != "Zeta" || len(m.img.filter) != 0 {
		t.Errorf("dir %q filter %q", m.img.dir, string(m.img.filter))
	}
	press(m, "left")
	if filepath.Base(m.img.dir) == "Zeta" || m.img.selected().name != "Zeta" {
		t.Errorf("left: dir %q cursor %q", m.img.dir, m.img.selected().name)
	}
	press(m, "esc")
	if m.img != nil {
		t.Error("esc did not close")
	}
}

func TestPickerHidden(t *testing.T) {
	_, md := pickerFixture(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := config.Defaults()
	cfg.ShowHidden = true
	m, err := New(Options{Config: cfg, Path: md})
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 24})
	press(m, "e", "ctrl+p")
	got := pickerNames(m)
	if got[1] != ".hid" || got[len(got)-1] != "My Pic.PNG" || len(got) != 11 {
		t.Errorf("hidden shown: %v", got)
	}
}

func TestPickerAltFlowAndInsert(t *testing.T) {
	_, md := pickerFixture(t)
	m := testModel(t, md)
	press(m, "e", "down") // cursor on line 2
	press(m, "ctrl+p")
	typeText(m, "my")
	cur := m.img.cur
	press(m, "enter")
	if !m.img.altOn || string(m.img.alt) != "My Pic" {
		t.Fatalf("alt prompt %v %q", m.img.altOn, string(m.img.alt))
	}
	if !strings.Contains(screenText(m), "alt text: My Pic") {
		t.Error("status line lacks the prompt")
	}
	// esc goes back: same directory, same cursor.
	press(m, "esc")
	if m.img == nil || m.img.altOn || m.img.cur != cur || string(m.img.filter) != "my" {
		t.Fatalf("after esc: %+v", m.img)
	}
	press(m, "enter", "backspace", "backspace", "!")
	press(m, "enter")
	if m.img != nil {
		t.Fatal("picker still open")
	}
	b := m.doc.buf
	if got := b.Text(); got != "one\ntwo\n![My P!](My%20Pic.PNG)\nthree" {
		t.Errorf("text %q", got)
	}
	if b.Row != 1 || m.mode != modeEdit || !b.Dirty() {
		t.Errorf("row %d mode %v dirty %v", b.Row, m.mode, b.Dirty())
	}
	press(m, "ctrl+z")
	if b.Text() != "one\ntwo\nthree" {
		t.Errorf("undo: %q", b.Text())
	}
	press(m, "ctrl+z")
	if b.Text() != "one\ntwo\nthree" {
		t.Errorf("second undo changed text: %q", b.Text())
	}
}

func TestPickerNestedRelativePath(t *testing.T) {
	dir, md := pickerFixture(t)
	os.WriteFile(filepath.Join(dir, "sub dir", "ph(o) 100%.png"), []byte("x"), 0o644)
	m := testModel(t, md)
	press(m, "e", "ctrl+p", "down", "down", "enter", "down", "enter")
	if string(m.img.alt) != "ph(o) 100%" {
		t.Fatalf("alt %q", string(m.img.alt))
	}
	press(m, "enter")
	want := "![ph(o) 100%](sub%20dir/ph%28o%29%20100%25.png)"
	if got := m.doc.buf.Lines[1]; string(got) != want {
		t.Errorf("line %q, want %q", string(got), want)
	}
	if p, ok := m.imagePath("sub%20dir/ph%28o%29%20100%25.png"); !ok || p != filepath.Join(dir, "sub dir", "ph(o) 100%.png") {
		t.Errorf("round trip: %q", p)
	}
}

func TestPickerAbsoluteWithoutPath(t *testing.T) {
	m := testModel(t, "")
	m.doc.path = ""
	file := filepath.Join(t.TempDir(), "a b.png")
	if got, want := m.imageRef(file), strings.ReplaceAll(filepath.ToSlash(file), " ", "%20"); got != want {
		t.Errorf("ref %q want %q", got, want)
	}
}

func TestPickerVimUndo(t *testing.T) {
	_, md := pickerFixture(t)
	for _, insert := range []bool{false, true} {
		m := newVimModel(t, md)
		press(m, "e")
		if insert {
			press(m, "A")
		}
		press(m, "ctrl+p")
		if m.img == nil {
			t.Fatal("picker not open")
		}
		for i := 0; i < 8; i++ { // down to the last file
			press(m, "down")
		}
		press(m, "enter", "enter")
		b := m.doc.buf
		if string(b.Lines[1]) != "![My Pic](My%20Pic.PNG)" || b.Row != 0 {
			t.Errorf("insert=%v lines %q row %d", insert, b.Text(), b.Row)
		}
		if got := m.km.ModeLabel(); (got == "INSERT") != insert {
			t.Errorf("mode %s", got)
		}
		if insert {
			press(m, "esc")
		}
		press(m, "u")
		if b.Text() != "one\ntwo\nthree" {
			t.Errorf("insert=%v after u: %q", insert, b.Text())
		}
		press(m, "u")
		if b.Text() != "one\ntwo\nthree" {
			t.Errorf("insert=%v second u: %q", insert, b.Text())
		}
	}
}

func newVimModel(t *testing.T, path string) *Model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := New(Options{
		Config: config.Defaults(), Path: path, Vim: true,
		NewVim: func(tw int) keymap.Keymap { return vim.New(tw) },
	})
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 24})
	return m
}

func TestPickerMouse(t *testing.T) {
	_, md := pickerFixture(t)
	m := testModel(t, md)
	press(m, "e", "ctrl+p")
	r := m.pickerRect()
	if r.w != 140*8/10 || r.h != 24*8/10 {
		t.Errorf("rect %+v", r)
	}
	// Click outside does nothing.
	m.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
	if m.img == nil || m.img.cur != 0 {
		t.Error("outside click changed the picker")
	}
	row3 := func() (int, int) { return r.x + 3, r.y + 2 + 3 }
	x, y := row3()
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	if m.img.cur != 3 {
		t.Errorf("click cursor %d", m.img.cur)
	}
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft}) // double-click on "Zeta"
	if filepath.Base(m.img.dir) != "Zeta" {
		t.Errorf("double-click dir %q", m.img.dir)
	}
	m.Update(tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelDown})
	if m.img.off != 0 { // a short list does not scroll
		t.Errorf("off %d", m.img.off)
	}
}

func TestPickerPreviewInfo(t *testing.T) {
	dir, md := pickerFixture(t)
	f, _ := os.Create(filepath.Join(dir, "real.png"))
	png.Encode(f, image.NewRGBA(image.Rect(0, 0, 640, 480)))
	f.Close()
	m := testModel(t, md)
	press(m, "e", "ctrl+p")
	typeText(m, "real")
	s := screenText(m)
	if !strings.Contains(s, "640 × 480") || !strings.Contains(s, " B") && !strings.Contains(s, "KB") {
		t.Errorf("no info:\n%s", s)
	}
	if got := formatSize(12595); got != "12.3 KB" {
		t.Errorf("size %q", got)
	}
}

func TestPickerPreviewImage(t *testing.T) {
	dir, md := pickerFixture(t)
	f, _ := os.Create(filepath.Join(dir, "real.png"))
	png.Encode(f, image.NewRGBA(image.Rect(0, 0, 80, 64)))
	f.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := New(Options{Config: config.Defaults(), Path: md, Images: kitty.NewManager(8, 16)})
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 24})
	if msg := m.opts.Images.Load(filepath.Join(dir, "real.png"))(); msg.(kitty.LoadedMsg).Err != nil {
		t.Fatal(msg)
	}
	press(m, "e", "ctrl+p")
	typeText(m, "real")
	if m.img.prev.id == 0 {
		t.Fatal("no preview image")
	}
	if cols := m.img.prev.cols; cols != 10 { // 80 px / 8 px cells: not scaled up
		t.Errorf("cols %d", cols)
	}
	if !strings.ContainsRune(screenText(m), '\U0010EEEE') {
		t.Error("no placeholder cells")
	}
}

func TestImagePathPercentDecoding(t *testing.T) {
	m := testModel(t, "")
	m.doc.abs = "/d/e/doc.md"
	for ref, want := range map[string]string{
		"my%20pic.png": "/d/e/my pic.png",
		"a/b%2Fc.png":  "/d/e/a/b/c.png",
		"100%.png":     "/d/e/100%.png", // bad escape: the raw path
		"/x/y%20z.png": "/x/y z.png",
		"plain.png":    "/d/e/plain.png",
	} {
		if got, ok := m.imagePath(ref); !ok || got != want {
			t.Errorf("imagePath(%q) = %q, %v; want %q", ref, got, ok, want)
		}
	}
}

// TestPickerVimMarks checks that the line of the image inserter moves the vim marks, and that
// the vim hooks leave the buffer with the vim keymap.
func TestPickerVimMarks(t *testing.T) {
	_, md := pickerFixture(t)
	m := newVimModel(t, md)
	press(m, "e", "j", "m", "a", "k", "ctrl+p")
	if m.img == nil {
		t.Fatal("picker not open")
	}
	for i := 0; i < 8; i++ {
		press(m, "down")
	}
	press(m, "enter", "enter")
	b := m.doc.buf
	if b.OnShift == nil {
		t.Error("the hook is not in the buffer during the vim session")
	}
	press(m, "G", "'", "a")
	if b.Row != 2 || string(b.Lines[2]) != "two" {
		t.Errorf("mark a: row %d in %q", b.Row, b.Text())
	}
	press(m, "ctrl+g") // conventional flavor
	if b.OnShift != nil || b.OnSnap != nil || b.OnRestore != nil {
		t.Error("the vim hooks stay in the buffer after the flavor switch")
	}
	press(m, "ctrl+g") // vim again
	press(m, "x")
	if b.OnShift == nil {
		t.Error("the hook is not back after the switch to vim")
	}
	press(m, "esc") // view mode
	if b.OnShift != nil {
		t.Error("the hook stays in the buffer in view mode")
	}
}

// TestVimHookLeavesOldDoc checks that a new document in edit mode takes the vim hooks from
// the buffer of the old document.
func TestVimHookLeavesOldDoc(t *testing.T) {
	_, md := pickerFixture(t)
	m := newVimModel(t, md)
	press(m, "e", "x")
	old := m.doc.buf
	if old.OnShift == nil {
		t.Fatal("no hook in edit mode")
	}
	m.setDoc(blankDoc())
	if old.OnShift != nil || old.OnSnap != nil || old.OnRestore != nil {
		t.Error("the old buffer keeps the vim hooks")
	}
}
