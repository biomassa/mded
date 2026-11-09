package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// TestWheelEventsShareFrame checks that wheel events less than one frame apart change the
// scroll position, give the last frame again, and get one new frame from the frameMsg.
func TestWheelEventsShareFrame(t *testing.T) {
	var b strings.Builder
	for i := range 500 {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	p := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	m := testModel(t, p)
	first := m.View().Content
	x, y := m.docRect().x+5, m.docRect().y+3
	wheel := tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelDown}

	m.built = time.Now().Add(-frameTime) // the last frame is one frame old
	m.Update(wheel)                      // so the event gets a new frame at once
	second := m.View().Content
	if second == first || m.topLine() != 3 {
		t.Fatalf("first event: top line %d, frame changed %v", m.topLine(), second != first)
	}
	m.built = time.Now() // the next events come less than one frame after the frame: held
	_, cmd := m.Update(wheel)
	m.Update(wheel)
	if cmd == nil {
		t.Fatal("a held event must ask for a frame")
	}
	if got := m.View().Content; got != second {
		t.Fatal("a held event must give the last frame")
	}
	m.Update(frameMsg{})
	if got := m.View().Content; got == second || m.topLine() != 9 {
		t.Fatalf("after the frame: top line %d, want 9", m.topLine())
	}
}
