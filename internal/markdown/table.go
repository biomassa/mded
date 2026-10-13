package markdown

import (
	"strconv"
	"strings"

	"github.com/biomassa/mded/internal/theme"
)

// tableCell is the text of one table cell and the rune offset of the text in its line.
type tableCell struct {
	text string
	off  int
}

// splitTableRow splits a table row at its "|" marks. An escaped "\|" does not split. Each
// cell text has no space at the start and the end.
func splitTableRow(l []rune) []tableCell {
	var segs [][2]int
	a := 0
	for i := 0; i <= len(l); i++ {
		if i == len(l) || l[i] == '|' && (i == 0 || l[i-1] != '\\') {
			segs = append(segs, [2]int{a, i})
			a = i + 1
		}
	}
	blank := func(s [2]int) bool { return strings.TrimSpace(string(l[s[0]:s[1]])) == "" }
	if len(segs) > 0 && blank(segs[0]) && strings.HasPrefix(strings.TrimSpace(string(l)), "|") {
		segs = segs[1:]
	}
	if len(segs) > 0 && blank(segs[len(segs)-1]) {
		segs = segs[:len(segs)-1]
	}
	out := make([]tableCell, 0, len(segs))
	for _, s := range segs {
		a, b := s[0], s[1]
		for a < b && l[a] == ' ' || a < b && l[a] == '\t' {
			a++
		}
		for b > a && (l[b-1] == ' ' || l[b-1] == '\t') {
			b--
		}
		out = append(out, tableCell{string(l[a:b]), a})
	}
	return out
}

// tableAlign is the alignment of a column: 0 left, 1 center, 2 right.
func tableAlign(sep string) int {
	sep = strings.TrimSpace(sep)
	switch {
	case strings.HasPrefix(sep, ":") && strings.HasSuffix(sep, ":") && len(sep) > 1:
		return 1
	case strings.HasSuffix(sep, ":"):
		return 2
	}
	return 0
}

// tableRows draws a table as rows with cells: natural column widths, and the widest columns
// shrink and wrap if the table is wider than tw. The cells have the source position of
// their text in Src. A border or a padding cell has Src -1. The Line of a row is relative
// to the first line of the table. The cache keeps the rows.
func (c *Cache) tableRows(lines [][]rune, b Block, tw int) []Row {
	var sb strings.Builder
	for j := b.From; j <= b.To; j++ {
		sb.WriteString(string(lines[j]))
		sb.WriteByte('\n')
	}
	key := strconv.Itoa(tw) + "\x00" + sb.String()
	if c.tables == nil || len(c.tables) > 2000 {
		c.tables = map[string][]Row{}
	}
	rows, ok := c.tables[key]
	if !ok {
		rows = buildTable(lines[b.From:b.To+1], tw)
		c.tables[key] = rows
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		r.Line += b.From
		out[i] = r
	}
	return out
}

// buildTable makes the rows of a table. lines[0] is the header, lines[1] the separator.
func buildTable(lines [][]rune, tw int) []Row {
	text := KindStyle(KindText)
	hdr := splitTableRow(lines[0])
	n := len(hdr)
	aligns := make([]int, n)
	for i, sc := range splitTableRow(lines[1]) {
		if i < n {
			aligns[i] = tableAlign(sc.text)
		}
	}
	// cells[r][i] are the cells of column i in line r. Line 1 (the separator) has none.
	cells := make([][][]Cell, len(lines))
	widths := make([]int, n)
	for r, l := range lines {
		if r == 1 {
			continue
		}
		cells[r] = make([][]Cell, n)
		for i, tc := range splitTableRow(l) {
			if i >= n {
				break
			}
			cells[r][i] = inlineCells(tc.text, tc.off, text, nil)
			widths[i] = max(widths[i], len(cells[r][i]))
		}
	}
	// Shrink the widest columns until the table fits: each column has 2 spaces of padding
	// and a border between columns.
	avail := tw - 3*n
	sum := func() (s int) {
		for _, w := range widths {
			s += w
		}
		return
	}
	for sum() > max(avail, n) {
		wi := 0
		for i, w := range widths {
			if w > widths[wi] {
				wi = i
			}
		}
		if widths[wi] <= 1 {
			break
		}
		widths[wi]--
	}
	for i := range widths {
		widths[i] = max(1, widths[i])
	}
	line := theme.St(theme.Text)
	bar := func(r rune) Cell { return Cell{r, line, -1} }
	pad := func(k int) (out []Cell) {
		for ; k > 0; k-- {
			out = append(out, Cell{' ', text, -1})
		}
		return
	}
	var rows []Row
	for r := range lines {
		if r == 1 { // the header separator
			row := Row{Line: 1, Cells: []Cell{bar(' ')}}
			for i, w := range widths {
				if i > 0 {
					row.Cells = append(row.Cells, bar('┼'))
				}
				for k := 0; k < w+2; k++ {
					row.Cells = append(row.Cells, bar('─'))
				}
			}
			row.Pre = RenderRow(row, tw, -1)
			rows = append(rows, row)
			continue
		}
		// Wrap each cell of the row, then make as many screen rows as the tallest cell has.
		wrapped := make([][][]Cell, n)
		height := 1
		for i := range widths {
			wrapped[i] = wrapCells(cells[r][i], widths[i])
			height = max(height, len(wrapped[i]))
		}
		for k := 0; k < height; k++ {
			row := Row{Line: r, Cells: []Cell{bar(' ')}}
			for i, w := range widths {
				if i > 0 {
					row.Cells = append(row.Cells, bar('│'))
				}
				var seg []Cell
				if k < len(wrapped[i]) {
					seg = wrapped[i][k]
				}
				left, right := 0, w-len(seg)
				switch aligns[i] {
				case 1:
					left = right / 2
					right -= left
				case 2:
					left, right = right, 0
				}
				row.Cells = append(row.Cells, pad(1+left)...)
				row.Cells = append(row.Cells, seg...)
				row.Cells = append(row.Cells, pad(right+1)...)
			}
			row.Pre = RenderRow(row, tw, -1)
			rows = append(rows, row)
		}
	}
	return rows
}

// wrapCells splits the cells of one table cell into segments of at most w cells. A space
// at a break goes away.
func wrapCells(cs []Cell, w int) [][]Cell {
	rs := make([]rune, len(cs))
	for i, c := range cs {
		rs[i] = c.R
	}
	starts := WrapLine(rs, w)
	out := make([][]Cell, len(starts))
	for k, s := range starts {
		end := len(cs)
		if k+1 < len(starts) {
			end = starts[k+1]
		}
		seg := cs[s:end]
		if k+1 < len(starts) {
			for len(seg) > 1 && seg[len(seg)-1].R == ' ' {
				seg = seg[:len(seg)-1]
			}
		}
		out[k] = seg
	}
	return out
}
