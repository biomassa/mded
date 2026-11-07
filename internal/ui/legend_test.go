package ui

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestLegendKeepsText(t *testing.T) {
	for _, s := range []string{"e edit · / search · ? help", "↑↓ move · tab document", "single"} {
		if got := ansi.Strip(Legend(s)); got != s {
			t.Errorf("Legend(%q) shows %q", s, got)
		}
	}
}
