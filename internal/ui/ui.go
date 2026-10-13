// Package ui has the small drawing helpers that the panes share: boxes, padding, and hints.
package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/biomassa/mded/internal/theme"
)

// WrapHint wraps a hint to lines at most w cells wide, each with a one-space indent.
// It breaks lines only between items (at " · "), so that an item such as "1–5 picks"
// stays on one line. An item that is wider than a line wraps at spaces.
func WrapHint(text, hex string, w int) []string {
	const sep = " · "
	var lines []string
	cur := ""
	for _, item := range strings.Split(text, sep) {
		switch {
		case cur == "":
			cur = item
		case lipgloss.Width(cur+sep+item) <= w-1:
			cur += sep + item
		default:
			lines = append(lines, cur)
			cur = item
		}
	}
	lines = append(lines, cur)
	var out []string
	for _, l := range lines {
		for _, part := range strings.Split(lipgloss.NewStyle().Width(max(1, w-1)).Render(l), "\n") {
			out = append(out, " "+theme.St(hex).Render(part))
		}
	}
	return out
}

// Trunc cuts s to w cells and adds an ellipsis.
func Trunc(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// Pad makes a styled string exactly w cells wide. It truncates or fills with the fill style.
func Pad(s string, w int, fill lipgloss.Style) string {
	if lipgloss.Width(s) > w {
		s = ansi.Truncate(s, w, "…")
	}
	if n := w - lipgloss.Width(s); n > 0 {
		s += fill.Render(strings.Repeat(" ", n))
	}
	return s
}

// PadLines returns exactly h lines, each w cells wide.
func PadLines(lines []string, w, h int) []string {
	out := make([]string, h)
	for i := range out {
		if i < len(lines) {
			out[i] = Pad(lines[i], w, lipgloss.NewStyle())
		} else {
			out[i] = strings.Repeat(" ", w)
		}
	}
	return out
}

// Box draws a rounded border with the title embedded in the top edge.
func Box(title string, lines []string, w int, borderHex string) string {
	b := theme.St(borderHex)
	inner := w - 2
	var sb strings.Builder
	if title == "" {
		sb.WriteString(b.Render("╭" + strings.Repeat("─", inner) + "╮"))
	} else {
		title = Trunc(title, max(0, inner-4))
		fill := max(0, inner-3-lipgloss.Width(title))
		sb.WriteString(b.Render("╭─ ") + title + b.Render(" "+strings.Repeat("─", fill)+"╮"))
	}
	for _, l := range lines {
		sb.WriteString("\n" + b.Render("│") + l + b.Render("│"))
	}
	sb.WriteString("\n" + b.Render("╰"+strings.Repeat("─", inner)+"╯"))
	return sb.String()
}
