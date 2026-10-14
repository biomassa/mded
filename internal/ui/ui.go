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

// Pane draws a pane with only a top line and a bottom line, w cells wide. The top line has
// the title. The side columns stay empty, so that the text keeps the position that it has in
// a Box. If sep is true, the right column is a vertical line in sepHex that separates the
// pane from the pane at its right, and the top and the bottom line join it with ┬ and ┴.
// lineHex is the color of the top line: the accent for the pane with the focus.
func Pane(title string, lines []string, w int, lineHex, sepHex string, sep bool) string {
	top, sp := theme.St(lineHex), theme.St(sepHex)
	inner := w - 2
	end, side, bottomEnd := top.Render("─"), " ", sp.Render("─")
	if sep {
		end, side, bottomEnd = sp.Render("┬"), sp.Render("│"), sp.Render("┴")
	}
	var sb strings.Builder
	if title == "" {
		sb.WriteString(top.Render(strings.Repeat("─", inner+1)) + end)
	} else {
		title = Trunc(title, max(0, inner-4))
		fill := max(0, inner-3-lipgloss.Width(title))
		sb.WriteString(top.Render("── ") + title + top.Render(" "+strings.Repeat("─", fill)) + end)
	}
	for _, l := range lines {
		sb.WriteString("\n " + l + side)
	}
	sb.WriteString("\n" + sp.Render(strings.Repeat("─", inner+1)) + bottomEnd)
	return sb.String()
}
