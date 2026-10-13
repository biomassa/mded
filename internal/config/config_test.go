package config

import (
	"os"
	"path/filepath"
	"testing"
)

func setup(t *testing.T) string {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	return filepath.Join(dir, "mded", "config.toml")
}

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMissing(t *testing.T) {
	setup(t)
	c, err := Load()
	if err != nil || c != Defaults() {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestLoadPartialAndInvalid(t *testing.T) {
	p := setup(t)
	write(t, p, "theme = \"nord\"\ndefault_mode = \"emacs\"\nimages = \"maybe\"\nimage_max_rows = 0\ntab_width = 2\nsidebar_width = -4\nshow_hidden = true\n")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := Defaults()
	want.Theme, want.TabWidth, want.ShowHidden = "nord", 2, true
	if c != want {
		t.Fatalf("got %+v, want %+v", c, want)
	}
}

func TestLoadBadFile(t *testing.T) {
	p := setup(t)
	write(t, p, "theme = [")
	if c, err := Load(); err == nil || c != Defaults() {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestSaveKeepsOtherKeys(t *testing.T) {
	p := setup(t)
	write(t, p, "tab_width = 8\nimages = \"off\"\n")
	if err := SaveTheme("dracula"); err != nil {
		t.Fatal(err)
	}
	if err := SaveSidebarWidth(42); err != nil {
		t.Fatal(err)
	}
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := Defaults()
	want.Theme, want.TabWidth, want.Images, want.SidebarWidth = "dracula", 8, "off", 42
	if c != want {
		t.Fatalf("got %+v, want %+v", c, want)
	}
	st, _ := os.Stat(p)
	if st.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v", st.Mode())
	}
}

func TestSaveCreatesFile(t *testing.T) {
	setup(t)
	if err := SaveTheme("nord"); err != nil {
		t.Fatal(err)
	}
	if c, _ := Load(); c.Theme != "nord" {
		t.Fatalf("theme = %q", c.Theme)
	}
}
