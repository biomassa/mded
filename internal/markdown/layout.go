package markdown

import (
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/biomassa/mded/internal/theme"
)

// LayoutOpts tells Layout how to build the rows.
type LayoutOpts struct {
	Width int  // text width
	Plain bool // non-markdown file: every line as source, no markdown styles, no blocks, no images
	// AsSource tells if the UI wants line to show its source: the cursor line, the active
	// block, the selection, and the lines with a search match. A block with one such line
	// shows all its lines as source.
	AsSource func(line int) bool
	// Decorate changes the style of a source cell: search highlight and selection. It can be nil.
	// It returns st itself for a cell that does not change, so that the cells share it.
	Decorate func(line, col int, st *lipgloss.Style) *lipgloss.Style
	// ImageSize gives the size of an image in cells. If it is nil or not ok, the image
	// has no rows.
	ImageSize func(ref ImageRef) (cols, rows int, ok bool)
}

// sourceRows makes the rows of source line i with the live styles of the source view.
func (o LayoutOpts) sourceRows(l []rune, i int, inFence bool) []Row {
	var kinds []StyleKind
	level := 0
	if !o.Plain {
		src := string(l)
		kinds = LineKinds(src, inFence && !isFence(l))
		if !inFence {
			level, _ = HeadingLevel(src)
		}
	}
	var rows []Row
	kindSt := map[StyleKind]*lipgloss.Style{}
	starts := WrapLine(l, BodyWidth(o.Width)) // every source row has the indent
	for k, s := range starts {
		end := len(l)
		if k+1 < len(starts) {
			end = starts[k+1]
		}
		row := Row{Line: i, Source: true, Start: s}
		for j := s; j < end; j++ {
			kind := KindText
			if kinds != nil {
				kind = kinds[j]
			}
			st := kindSt[kind]
			if st == nil { // one style of each kind for the line, shared by its cells
				if level > 0 && kind == KindHeading { // headings are bars in their level color
					st = stylePtr(theme.HeadingStyle(level))
				} else {
					st = stylePtr(KindStyle(kind))
				}
				kindSt[kind] = st
			}
			if o.Decorate != nil {
				st = o.Decorate(i, j, st)
			}
			row.Cells = append(row.Cells, Cell{l[j], st, j})
		}
		if level > 0 {
			row.Fill = level
		}
		rows = append(rows, row)
	}
	return rows
}

// hasImageMark is a quick test for "![" in a line, so that most lines skip the regexp.
func hasImageMark(l []rune) bool {
	for i := 0; i+1 < len(l); i++ {
		if l[i] == '!' && l[i+1] == '[' {
			return true
		}
	}
	return false
}

// imageRows makes the image rows of the references of a line that ImageSize accepts. alone
// is true if the whole line is one reference that has rows.
func (o LayoutOpts) imageRows(l []rune, i int) (rows []Row, alone bool) {
	if o.ImageSize == nil || !hasImageMark(l) {
		return nil, false
	}
	src := string(l)
	refs := FindImages(src)
	for _, ref := range refs {
		cols, n, ok := o.ImageSize(ref)
		if !ok || cols < 1 || n < 1 {
			continue
		}
		ref := ref
		for k := 0; k < n; k++ {
			rows = append(rows, Row{Line: i, Img: &ref, ImgRow: k})
		}
	}
	if len(refs) == 1 && len(rows) > 0 {
		r := []rune(src)
		alone = strings.TrimSpace(string(r[:refs[0].Start]))+strings.TrimSpace(string(r[refs[0].End:])) == ""
	}
	return rows, alone
}

// Unit returns the first and the last line of the layout unit of line i: the block (a
// fenced code block or a table) that has line i, or line i alone. blocks must be in the
// order of their lines, as Blocks returns them. The rows of a unit come only as a whole.
func Unit(blocks []Block, i int) (from, to int) {
	if b, ok := blockOf(blocks, i); ok {
		return b.From, b.To
	}
	return i, i
}

// blockOf returns the block that has line i.
func blockOf(blocks []Block, i int) (Block, bool) {
	k := sort.Search(len(blocks), func(k int) bool { return blocks[k].From > i }) - 1
	if k >= 0 && i <= blocks[k].To {
		return blocks[k], true
	}
	return Block{}, false
}

// UnitRows builds the rows of the unit that starts at line from (see Unit). A line that
// AsSource accepts shows as source. Another line shows as preview rows from the cache, and a
// block shows as rendered rows. A block with one line that AsSource accepts shows all its
// lines as source. An image on a line that does not show source gets image rows: in place
// of the line if it is alone on the line, else below it.
func (c *Cache) UnitRows(lines [][]rune, o LayoutOpts, blocks []Block, from int) []Row {
	o.Width = max(1, o.Width)
	asSource := func(i int) bool { return o.AsSource != nil && o.AsSource(i) }
	if from < 0 || from >= len(lines) {
		return nil
	}
	if o.Plain {
		return o.sourceRows(lines[from], from, false)
	}
	if b, ok := blockOf(blocks, from); ok && b.From == from {
		src := false
		for i := b.From; i <= b.To && !src; i++ {
			src = asSource(i)
		}
		var rows []Row
		if src {
			for i := b.From; i <= b.To; i++ {
				// In a fenced block, the lines after the opening fence have the code style.
				rows = append(rows, o.sourceRows(lines[i], i, !b.Table && i > b.From)...)
			}
			return rows
		}
		first, last := codeLines(lines, b)
		for k, r := range c.BlockRows(lines, b, o.Width) {
			if !b.Table { // a code block row: its source line, for a click and a copy
				r.Line = max(first, min(first+k, last))
			}
			rows = append(rows, r)
		}
		return rows
	}
	l := lines[from]
	if asSource(from) {
		return o.sourceRows(l, from, false)
	}
	img, alone := o.imageRows(l, from)
	if alone {
		return img
	}
	var rows []Row
	for _, r := range c.PreviewRows(string(l), o.Width) {
		r.Line = from
		rows = append(rows, r)
	}
	return append(rows, img...)
}

// Layout builds all rows of a document: the rows of all units, in order. The UI builds only
// the rows on the screen (see UnitRows). Layout is for tests and small documents.
func (c *Cache) Layout(lines [][]rune, o LayoutOpts) []Row {
	var blocks []Block
	if !o.Plain {
		blocks = Blocks(lines)
	}
	var rows []Row
	for i := 0; i < len(lines); {
		_, to := Unit(blocks, i)
		rows = append(rows, c.UnitRows(lines, o, blocks, i)...)
		i = to + 1
	}
	return rows
}

// CursorRow returns the index of the row that has the cursor at col of source line. It
// uses Start of the source rows. It gives -1 if the line has no row.
func CursorRow(rows []Row, line, col int) int {
	first, best := -1, -1
	for i, r := range rows {
		if r.Line != line || r.Img != nil {
			continue
		}
		if first < 0 {
			first = i
		}
		if r.Source && col >= r.Start {
			best = i
		}
	}
	if best < 0 {
		return first
	}
	return best
}

// codeLines returns the first and the last code line of a fenced block, without the fence
// lines. glamour does not show the fences, so row k of the block shows code line first+k
// (a long line that wraps makes this an approximation). A block with no code gives its
// first line.
func codeLines(lines [][]rune, b Block) (first, last int) {
	first, last = b.From+1, b.To
	if last > b.From && strings.HasPrefix(strings.TrimSpace(string(lines[last])), "```") {
		last--
	}
	if last < first {
		return b.From, b.From
	}
	return first, last
}
