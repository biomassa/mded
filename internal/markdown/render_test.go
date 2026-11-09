package markdown

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/biomassa/mded/internal/theme"
)

// renderRowCells is the reference of RenderRow: one Render call for each cell, as before
// RenderRow joined the runs of cells with the same style.
func renderRowCells(r Row, tw, cx int) string {
	if r.Img != nil {
		return ""
	}
	if r.Pre != "" || r.Cells == nil && !r.Source && r.Fill == 0 {
		return r.Pre
	}
	lead := " "
	switch {
	case r.Indent() == 0:
	case r.Fill > 0:
		lead += theme.HeadingStyle(r.Fill).Render(" ")
	default:
		lead += " "
	}
	var b strings.Builder
	for k := 0; k < len(r.Cells); k++ {
		var st lipgloss.Style
		if r.Cells[k].St != nil {
			st = *r.Cells[k].St
		}
		if k == cx {
			st = st.Reverse(true)
		}
		text := []rune{r.Cells[k].R}
		for k+1 < len(r.Cells) && joins(r.Cells[k+1].R) {
			k++
			text = append(text, r.Cells[k].R)
		}
		b.WriteString(st.Render(string(text)))
	}
	if cx >= len(r.Cells) && cx >= 0 {
		b.WriteString(lipgloss.NewStyle().Reverse(true).Render(" "))
	}
	if r.Fill > 0 {
		n := tw - r.Indent() - lipgloss.Width(b.String())
		if f := theme.HeadingFill(r.Fill); f >= 0 {
			n = min(n, f)
		}
		if n > 0 {
			b.WriteString(theme.HeadingStyle(r.Fill).Render(strings.Repeat(" ", n)))
		}
	}
	return lead + b.String()
}

// screenOf draws s in a buffer, as the terminal shows it.
func screenOf(s string, w int) uv.ScreenBuffer {
	buf := uv.NewScreenBuffer(w, 1)
	uv.NewStyledString(s).Draw(buf, uv.Rect(0, 0, w, 1))
	return buf
}

// sameScreen reports the first cell where a and b differ in text or style.
func sameScreen(t *testing.T, name, a, b string, w int) {
	t.Helper()
	sa, sb := screenOf(a, w), screenOf(b, w)
	for x := 0; x < w; x++ {
		ca, cb := sa.CellAt(x, 0), sb.CellAt(x, 0)
		if (ca == nil) != (cb == nil) || ca != nil && (ca.Content != cb.Content || !ca.Style.Equal(&cb.Style)) {
			t.Fatalf("%s: cell %d differs\nruns:  %q\ncells: %q", name, x, a, b)
		}
	}
}

// TestRenderRowRunsSameScreen checks that a row drawn with one Render call for each run of
// cells looks the same on the screen as a row drawn with one call for each cell.
func TestRenderRowRunsSameScreen(t *testing.T) {
	doc := strings.Join([]string{
		"# Heading **bold** `code`",
		"Text with **bold**, *italic*, ~~strike~~, `code`, and a [link](x.md).",
		"- [ ] a task with `code` and ✔️ an emoji with a variation selector",
		"> a quote with é a combining mark",
		"```go",
		"func f() int { return 1 } // comment",
		"```",
		"| a | b |",
		"|---|---|",
		"| 1 | 2 |",
		"plain\ttab and    spaces at the end   ",
	}, "\n")
	lines := [][]rune{}
	for _, l := range strings.Split(doc, "\n") {
		lines = append(lines, []rune(l))
	}
	hl := stylePtr(lipgloss.NewStyle().Background(lipgloss.Color("#444444")))
	for _, plain := range []bool{false, true} {
		for _, deco := range []bool{false, true} {
			o := LayoutOpts{Width: 60, Plain: plain, AsSource: func(int) bool { return true }}
			if deco { // a highlight on the columns 3 to 6 of each line, as a search match
				o.Decorate = func(line, col int, st *lipgloss.Style) *lipgloss.Style {
					if col >= 3 && col < 7 {
						return hl
					}
					return st
				}
			}
			var c Cache
			for _, r := range c.Layout(lines, o) {
				for _, cx := range []int{-1, 0, 2, 4, 7, len(r.Cells) - 1, len(r.Cells)} {
					got := RenderRow(r, 60, cx)
					want := renderRowCells(r, 60, cx)
					sameScreen(t, string(lines[r.Line]), got, want, 70)
				}
			}
		}
	}
}
