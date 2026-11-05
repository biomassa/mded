package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/biomassa/mded/internal/config"
)

// asyncModel is a model that loads in the background, as the program does.
func asyncModel(t *testing.T, path string) (*Model, tea.Cmd) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := New(Options{Config: config.Defaults(), Path: path,
		SaveTheme: func(string) error { return nil }, SaveSidebarWidth: func(int) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	init := m.Init() // bubbletea calls Init before the first Update
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	return m, init
}

// loadMsg runs the commands of cmd and returns the first docLoadedMsg that they give.
func loadMsg(t *testing.T, cmd tea.Cmd) docLoadedMsg {
	t.Helper()
	var find func(tea.Msg) (docLoadedMsg, bool)
	find = func(msg tea.Msg) (docLoadedMsg, bool) {
		switch msg := msg.(type) {
		case docLoadedMsg:
			return msg, true
		case tea.BatchMsg:
			for _, c := range msg {
				if c == nil {
					continue
				}
				if lm, ok := find(c()); ok {
					return lm, true
				}
			}
		}
		return docLoadedMsg{}, false
	}
	if cmd == nil {
		t.Fatal("no command")
	}
	lm, ok := find(cmd())
	if !ok {
		t.Fatal("no load result")
	}
	return lm
}

// The start file loads in the background. Until the result comes, the status line shows
// "loading NAME…" and the pane shows the empty document.
func TestStartLoadsInBackground(t *testing.T) {
	p := writeTemp(t, "a.md", "# Hello\n\ntext\n", 0o644)
	m, init := asyncModel(t, p)
	if !strings.Contains(screenText(m), "loading a.md…") || strings.Contains(screenText(m), "Hello") {
		t.Fatalf("before the result:\n%s", screenText(m))
	}
	m.Update(loadMsg(t, init))
	if !strings.Contains(screenText(m), "Hello") || strings.Contains(screenText(m), "loading") {
		t.Errorf("after the result:\n%s", screenText(m))
	}
}

// esc cancels a load: its result is dropped when it comes. A result of an older load is
// dropped too.
func TestLoadCancel(t *testing.T) {
	p := writeTemp(t, "a.md", "# Hello\n", 0o644)
	m, init := asyncModel(t, p)
	press(m, "esc")
	if strings.Contains(screenText(m), "loading") {
		t.Error("esc did not end the load")
	}
	m.Update(loadMsg(t, init))
	if strings.Contains(screenText(m), "Hello") || m.doc.path != "" {
		t.Error("the result of a canceled load was used")
	}
	// Two loads: only the result of the second one counts.
	q := writeTemp(t, "b.md", "# Second\n", 0o644)
	var first, second docLoadedMsg
	m.loadThen(p, m.setDoc)
	first = loadMsg(t, tea.Batch(m.cmds...))
	m.cmds = nil
	m.loadThen(q, m.setDoc)
	second = loadMsg(t, tea.Batch(m.cmds...))
	m.cmds = nil
	m.Update(first)
	if m.doc.path == p {
		t.Error("an older load replaced the document")
	}
	m.Update(second)
	if m.doc.path != q {
		t.Errorf("document %q, want %q", m.doc.path, q)
	}
}

// A file above maxEditSize opens in view mode only. A file above maxOpenSize does not
// open; at the start, the program stops with the error.
func TestSizeLimits(t *testing.T) {
	oldOpen, oldEdit := maxOpenSize, maxEditSize
	t.Cleanup(func() { maxOpenSize, maxEditSize = oldOpen, oldEdit })
	maxEditSize, maxOpenSize = 10, 100 // bytes
	big := writeTemp(t, "big.md", strings.Repeat("x", 50)+"\n", 0o644)
	m := testModel(t, big)
	press(m, "e")
	if m.mode == modeEdit {
		t.Fatal("a file above the edit limit opened in edit mode")
	}
	if !strings.Contains(m.msg, "big.md is over 0 MB: view mode only") {
		t.Errorf("message %q", m.msg)
	}
	huge := writeTemp(t, "huge.md", strings.Repeat("x", 200)+"\n", 0o644)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := New(Options{Config: config.Defaults(), Path: huge, SyncLoad: true}); err == nil ||
		!strings.Contains(err.Error(), "huge.md is over 0 MB: too large to open") {
		t.Errorf("start with a too large file: %v", err)
	}
	m.loadThen(huge, m.setDoc)
	if m.doc.path == huge || !strings.Contains(m.msg, "too large to open") {
		t.Errorf("open of a too large file: doc %q, message %q", m.doc.path, m.msg)
	}
}

// The start check finds a binary file and a directory without a full read.
func TestStartCheck(t *testing.T) {
	bin := writeTemp(t, "x.bin", "ab\x00cd", 0o644)
	if err := checkFile(bin); err == nil || !strings.Contains(err.Error(), "binary file: x.bin") {
		t.Errorf("binary: %v", err)
	}
	if err := checkFile(t.TempDir()); err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Errorf("directory: %v", err)
	}
	if err := checkFile(t.TempDir() + "/new.md"); err != nil {
		t.Errorf("a new file: %v", err)
	}
}
