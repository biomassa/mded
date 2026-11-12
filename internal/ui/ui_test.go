package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestBoxAndPad(t *testing.T) {
	out := Box("Title", PadLines([]string{"a"}, 10, 2), 12, "#888888")
	lines := strings.Split(out, "\n")
	if len(lines) != 4 {
		t.Fatalf("lines = %d", len(lines))
	}
	for _, l := range lines {
		if w := lipgloss.Width(l); w != 12 {
			t.Errorf("width %d: %q", w, ansi.Strip(l))
		}
	}
	if !strings.Contains(ansi.Strip(lines[0]), "Title") {
		t.Error("no title")
	}
}

func TestWrapHint(t *testing.T) {
	for _, l := range WrapHint("aaa bbb · ccc ddd · eee", "#888888", 14) {
		if lipgloss.Width(l) > 14 {
			t.Errorf("too wide: %q", ansi.Strip(l))
		}
	}
	if Trunc("hello world", 5) != "hell…" {
		t.Errorf("trunc = %q", Trunc("hello world", 5))
	}
}

func TestPaneLinesFoot(t *testing.T) {
	lines := PaneLinesFoot("T", "name ↑", PadLines([]string{"a"}, 18, 1), 20, "#888888", "#444444", true)
	bottom := ansi.Strip(lines[len(lines)-1])
	if bottom != strings.Repeat("─", 8)+" name ↑ ───┴" {
		t.Fatalf("bottom %q", bottom)
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w != 20 {
			t.Fatalf("line %d width %d: %q", i, w, ansi.Strip(l))
		}
	}
	// Too narrow for the label: a plain line.
	lines = PaneLinesFoot("T", "name ↑", PadLines([]string{"a"}, 8, 1), 10, "#888888", "#444444", true)
	if bottom := ansi.Strip(lines[len(lines)-1]); bottom != strings.Repeat("─", 9)+"┴" {
		t.Fatalf("narrow bottom %q", bottom)
	}
}
