package markdown

import (
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
	Decorate func(line, col int, st lipgloss.Style) lipgloss.Style
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
	starts := WrapLine(l, o.Width)
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
			st := KindStyle(kind)
			if level > 0 && kind == KindHeading { // headings are bars in their level color
				st = theme.HeadingStyle(level)
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

// Layout builds all rows of a document. The UI does the scrolling. A line that AsSource
// accepts shows as source. Another line shows as preview rows from the cache, and a fenced
// block or a table shows as rendered rows. An image on a line that does not show source
// gets image rows: in place of the line if it is alone on the line, else below it.
func (c *Cache) Layout(lines [][]rune, o LayoutOpts) []Row {
	o.Width = max(1, o.Width)
	asSource := func(i int) bool { return o.AsSource != nil && o.AsSource(i) }
	var rows []Row
	if o.Plain {
		for i, l := range lines {
			rows = append(rows, o.sourceRows(l, i, false)...)
		}
		return rows
	}
	blockAt := map[int]Block{} // first line → block
	inBlock := make([]bool, len(lines))
	blockSrc := map[int]bool{} // first line → the block shows source
	for _, b := range Blocks(lines) {
		blockAt[b.From] = b
		for i := b.From; i <= b.To; i++ {
			inBlock[i] = true
			if asSource(i) {
				blockSrc[b.From] = true
			}
		}
	}
	cur := Block{From: -1, To: -1} // the block of the line in the loop
	fence := false                 // for the source styles: the line is in a fenced block
	for i, l := range lines {
		if b, ok := blockAt[i]; ok {
			cur = b
		}
		src := blockSrc[cur.From] && i >= cur.From && i <= cur.To || !inBlock[i] && asSource(i)
		switch {
		case src:
			rows = append(rows, o.sourceRows(l, i, fence)...)
		case inBlock[i]:
			if b, first := blockAt[i]; first { // the block rows come with its first line
				for k, r := range c.BlockRows(lines, b, o.Width) {
					if !b.Table { // a code block row: its source line, for a click
						r.Line = min(b.From+k, b.To)
					}
					rows = append(rows, r)
				}
			}
		default:
			img, alone := o.imageRows(l, i)
			if alone {
				rows = append(rows, img...)
				break
			}
			for _, r := range c.PreviewRows(string(l), o.Width) {
				r.Line = i
				rows = append(rows, r)
			}
			rows = append(rows, img...)
		}
		if isFence(l) {
			fence = !fence
		}
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
