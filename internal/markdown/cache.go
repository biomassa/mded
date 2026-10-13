package markdown

import (
	"fmt"
	"regexp"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/biomassa/mded/internal/theme"
)

var mdLink = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// Cache keeps rendered markdown. Glamour is slow, and View runs on every update.
// Use a pointer, so that all copies of the Model share it.
type Cache struct {
	out       map[string][]string
	renderers map[int]*glamour.TermRenderer
	preview   map[string][]Row // formatted lines for the width previewW: see PreviewRows
	previewW  int
	tables    map[string][]Row // table rows by width and text, with Line relative to the table
}

// NewCache returns an empty cache.
func NewCache() *Cache { return &Cache{} }

// Reset clears the cache. Call it when the theme changes.
func (c *Cache) Reset() {
	c.out = map[string][]string{}
	c.renderers = map[int]*glamour.TermRenderer{}
	c.preview = map[string][]Row{}
	c.tables = map[string][]Row{}
}

// Render returns markdown as lines that are at most w cells wide.
// If glamour fails, it returns plain wrapped text.
func (c *Cache) Render(src string, w int) []string {
	// A drag of the pane border makes many widths: the cache starts again when it gets large.
	if c.out == nil || len(c.out) > 4000 || len(c.renderers) > 12 {
		c.Reset()
	}
	key := fmt.Sprintf("%d\x00%s", w, src)
	if lines, ok := c.out[key]; ok {
		return lines
	}
	r := c.renderers[w]
	if r == nil {
		r, _ = glamour.NewTermRenderer(glamour.WithStyles(theme.ReaderStyle()), glamour.WithWordWrap(max(10, w-2)))
		c.renderers[w] = r
	}
	// Headings are full-width bars in the color of their level. Glamour renders the text
	// between them.
	var lines []string
	var chunk []string
	flush := func() {
		text := strings.Trim(strings.Join(chunk, "\n"), "\n")
		chunk = nil
		if strings.TrimSpace(text) == "" {
			return
		}
		var part []string
		if r != nil {
			if out, err := r.Render(text); err == nil {
				part = strings.Split(strings.Trim(out, "\n"), "\n")
			}
		}
		if part == nil {
			part = strings.Split(lipgloss.NewStyle().Width(w).Render(text), "\n")
		}
		// Glamour adds lines with only spaces around a part. Drop them, so that one empty
		// line separates a part from a heading.
		for len(part) > 0 && strings.TrimSpace(ansi.Strip(part[0])) == "" {
			part = part[1:]
		}
		for len(part) > 0 && strings.TrimSpace(ansi.Strip(part[len(part)-1])) == "" {
			part = part[:len(part)-1]
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, part...)
	}
	inFence := false
	for _, l := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			inFence = !inFence
		}
		level, text := HeadingLevel(l)
		if inFence || level == 0 {
			chunk = append(chunk, l)
			continue
		}
		flush()
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, theme.HeadingBar(level, plain(text), w))
	}
	flush()
	c.out[key] = lines
	return lines
}

// BlockRows returns the rows of a fenced code block or a table as the reader shows it.
// The cache keeps the rendered text.
func (c *Cache) BlockRows(lines [][]rune, b Block, tw int) []Row {
	if b.Table {
		return c.tableRows(lines, b, tw)
	}
	text := make([]string, 0, b.To-b.From+1)
	for j := b.From; j <= b.To; j++ {
		text = append(text, string(lines[j]))
	}
	rendered := c.Render(strings.Join(text, "\n"), tw+2)
	rows := make([]Row, len(rendered))
	for k, l := range rendered {
		rows[k] = Row{Pre: l}
	}
	return rows
}

// PreviewRows returns the formatted rows of a source line for width tw. Each row is drawn
// one time and kept in the cache with its cells (for a click), so that a key press
// draws only the lines that change: the cursor line and the line that it leaves.
// The cache is cleared when the theme changes, and when it gets large.
func (c *Cache) PreviewRows(src string, tw int) []Row {
	if c.preview == nil || len(c.preview) > 20000 || c.previewW != tw {
		c.preview, c.previewW = map[string][]Row{}, tw // one width at a time
	}
	if rows, ok := c.preview[src]; ok {
		return rows
	}
	var rows []Row
	cells, level := PreviewCells(src)
	if level == -1 { // a rule
		rows = []Row{{Pre: " " + theme.St(theme.Dim).Render(strings.Repeat("─", tw))}}
	} else {
		rs := make([]rune, len(cells))
		for k, cl := range cells {
			rs[k] = cl.R
		}
		starts := WrapLine(rs, tw)
		for k, s := range starts {
			end := len(cells)
			if k+1 < len(starts) {
				end = starts[k+1]
			}
			row := Row{Cells: cells[s:end]}
			if level > 0 {
				row.Fill = level
			}
			row.Pre = RenderRow(row, tw, -1)
			rows = append(rows, row)
		}
	}
	c.preview[src] = rows
	return rows
}

// RenderRow draws a row of width tw. cx is the cursor column in the row, or -1. An image
// row gives "": the UI draws it.
func RenderRow(r Row, tw, cx int) string {
	if r.Img != nil {
		return ""
	}
	if r.Pre != "" || r.Cells == nil && !r.Source && r.Fill == 0 {
		return r.Pre
	}
	var b strings.Builder
	for k, cl := range r.Cells {
		st := cl.St
		if k == cx {
			st = st.Reverse(true)
		}
		b.WriteString(st.Render(string(cl.R)))
	}
	if cx >= len(r.Cells) && cx >= 0 { // the cursor after the last rune
		b.WriteString(lipgloss.NewStyle().Reverse(true).Render(" "))
	}
	if r.Fill > 0 { // fill the bar to the full width
		if n := tw - lipgloss.Width(b.String()); n > 0 {
			b.WriteString(theme.HeadingStyle(r.Fill).Render(strings.Repeat(" ", n)))
		}
	}
	return " " + b.String()
}
