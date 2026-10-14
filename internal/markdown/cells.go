// Package markdown is the rendering engine for view mode and edit mode. It turns source
// lines into screen rows: preview cells, source cells, rendered blocks, and image rows.
package markdown

import (
	"path"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/biomassa/mded/internal/theme"
)

// Cell is one rune on the screen with its style. Src is the rune of the source line that
// it comes from, for a click and for item highlights. A marker that the preview adds
// (• or ☐) has the position of the source marker.
type Cell struct {
	R   rune
	St  lipgloss.Style
	Src int
}

// Row is one screen row. A rendered block or a cached row has its text in Pre. Other rows
// have Cells. An image row has Img and no text: the UI draws it.
type Row struct {
	Cells  []Cell
	Pre    string
	Line   int  // the source line of the row
	Fill   int  // a heading bar of this level: the row gets its style to the full width
	Source bool // the row shows source, so it can have the cursor
	Start  int  // source rows: the first rune of the line on this row
	// Img is the image of an image row. ImgRow is the number of the row inside the image.
	Img    *ImageRef
	ImgRow int
}

var (
	// inlineToken matches the inline markdown that the preview renders, in order of
	// priority: backslash escape, code, autolink, bold, strike, image, link, bare URL,
	// italic. An image starts with "!", so it wins over the link token at the same place.
	inlineToken = regexp.MustCompile("\\\\[!-/:-@\\[-`{-~]|`[^`]+`|<[a-zA-Z][a-zA-Z0-9+.-]{1,31}:[^\\s<>]*>|\\*\\*[^*]+\\*\\*|__[^_]+__|~~[^~]+~~|!\\[[^\\]]*\\]\\([^)\\s]+\\)|\\[[^\\]]+\\]\\([^)\\s]+\\)|https?://[^\\s<>()\\[\\]]+|\\*[^*\\s][^*]*\\*|_[^_\\s][^_]*_")
	checkItem   = regexp.MustCompile(`^(\s*)([-*+]|\d+[.)])\s+\[([ xX])\]\s+(.*)$`)
	bulletItem  = regexp.MustCompile(`^(\s*)[-*+]\s+(.*)$`)
	numberItem  = regexp.MustCompile(`^(\s*)(\d+[.)])\s+(.*)$`)
	quoteLine   = regexp.MustCompile(`^\s*>\s?(.*)$`)
	ruleLine    = regexp.MustCompile(`^\s*([-*_])(\s*[-*_]){2,}\s*$`)
	// tableDelim matches the delimiter row of a table: cells of dashes with optional
	// colons, between pipes. The outer pipes are optional.
	tableDelim = regexp.MustCompile(`^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?\s*$`)
)

// token is one inline element of a text. a and b are rune offsets.
type token struct {
	a, b int
	text string
}

// scanInline finds the inline elements of s that the preview renders.
func scanInline(s string) []token {
	locs := inlineToken.FindAllStringIndex(s, -1)
	if locs == nil {
		return nil
	}
	rs := []rune(s)
	var out []token
	for _, loc := range locs {
		a := utf8.RuneCountInString(s[:loc[0]])
		b := a + utf8.RuneCountInString(s[loc[0]:loc[1]])
		tok := s[loc[0]:loc[1]]
		// "_" and "*" inside a word (snake_case, 2*3*4) are not italic marks.
		if (tok[0] == '_' || (tok[0] == '*' && !strings.HasPrefix(tok, "**"))) && a > 0 && isWordRune(rs[a-1]) {
			continue
		}
		out = append(out, token{a, b, tok})
	}
	return out
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// span is an item of a line that the tab key can select: a checkbox or a link.
type span struct {
	kind       ItemKind
	start, end int // rune range in the source line
	url        string
}

// inlineCells renders the inline markdown of s. offset is the rune position of s in its
// source line. A link or a bare URL goes into spans, if it is not nil.
func inlineCells(s string, offset int, base lipgloss.Style, spans *[]span) []Cell {
	rs := []rune(s)
	var out []Cell
	part := func(a, b int, st lipgloss.Style) {
		for i := a; i < b; i++ {
			out = append(out, Cell{rs[i], st, offset + i})
		}
	}
	// inner renders the text of a span with its own inline markdown, for example code in
	// bold text. sp gets the links in it. The text of a link gives no items (sp is nil),
	// because the link itself is the item.
	inner := func(a, b int, st lipgloss.Style, sp *[]span) {
		out = append(out, inlineCells(string(rs[a:b]), offset+a, st, sp)...)
	}
	add := func(kind ItemKind, a, b int, url string) {
		if spans != nil {
			*spans = append(*spans, span{kind, offset + a, offset + b, url})
		}
	}
	at := 0
	for _, t := range scanInline(s) {
		a, b, tok := t.a, t.b, t.text
		part(at, a, base)
		switch {
		case tok[0] == '\\': // an escaped mark shows as text
			part(a+1, b, base)
		case tok[0] == '`':
			part(a+1, b-1, base.Foreground(theme.C(theme.Code)))
		case tok[0] == '<': // an autolink: <https://example.com>
			url := tok[1 : len(tok)-1]
			part(a+1, b-1, base.Foreground(theme.C(theme.Link)).Underline(true).Hyperlink(url))
			add(Link, a, b, url)
		case strings.HasPrefix(tok, "**") || strings.HasPrefix(tok, "__"):
			inner(a+2, b-2, base.Bold(true), spans)
		case strings.HasPrefix(tok, "~~"):
			inner(a+2, b-2, base.Strikethrough(true), spans)
		case strings.HasPrefix(tok, "!["):
			// An image reference shows as "📷 alt" in the link style, with no hyperlink. The
			// camera is two cells wide by every width method. A glyph with a variation
			// selector (such as 🖼 U+FE0F) is not: the renderer counts it one cell, the
			// terminal draws two, and the row wraps.
			st := base.Foreground(theme.C(theme.Link)).Underline(true)
			mid := strings.Index(tok, "](")
			out = append(out, Cell{'📷', st, offset + a}, Cell{' ', st, offset + a})
			if alt := tok[2:mid]; alt != "" {
				part(a+2, a+2+utf8.RuneCountInString(alt), st)
			} else { // no alt text: show the file name
				for _, r := range path.Base(tok[mid+2 : len(tok)-1]) {
					out = append(out, Cell{r, st, offset + a})
				}
			}
		case tok[0] == '[':
			close := strings.Index(tok, "](")
			text := utf8.RuneCountInString(tok[1:close])
			url := tok[close+2 : len(tok)-1]
			add(Link, a, b, url)
			inner(a+1, a+1+text, base.Foreground(theme.C(theme.Link)).Underline(true).Hyperlink(url), nil)
		case strings.HasPrefix(tok, "http"):
			part(a, b, base.Foreground(theme.C(theme.Link)).Underline(true).Hyperlink(tok))
			add(Link, a, b, tok)
		default: // *italic* or _italic_
			inner(a+1, b-1, base.Italic(true), spans)
		}
		at = b
	}
	part(at, len(rs), base)
	return out
}

// PreviewCells renders one source line that is not in a block, as the reader shows it.
// It reports a heading level, so that the row can be a full-width bar. Level -1 is a rule.
func PreviewCells(src string) (cells []Cell, level int) {
	cells, level, _ = previewLine(src, false)
	return cells, level
}

// previewLine is PreviewCells. If withSpans is true, it also returns the checkbox and the
// links of the line.
func previewLine(src string, withSpans bool) (cells []Cell, level int, spans []span) {
	var sp *[]span
	if withSpans {
		sp = &spans
	}
	text := KindStyle(KindText)
	marker := KindStyle(KindMarker)
	lead := func(indent string) []Cell {
		var out []Cell
		for i, r := range indent {
			out = append(out, Cell{r, text, i})
		}
		return out
	}
	runeAt := func(byteIdx int) int { return utf8.RuneCountInString(src[:byteIdx]) }
	if lv, h := HeadingLevel(src); lv > 0 {
		st := theme.HeadingStyle(lv)
		off := runeAt(strings.Index(src, h))
		return append([]Cell{{' ', st, 0}}, inlineCells(h, off, st, sp)...), lv, spans
	}
	if ruleLine.MatchString(src) {
		return []Cell{{'─', theme.St(theme.Dim), 0}}, -1, nil // -1: a rule to the full width
	}
	if sm := checkItem.FindStringSubmatchIndex(src); sm != nil {
		box := '☐'
		if src[sm[6]:sm[7]] != " " {
			box = '☑'
		}
		cells = lead(src[sm[2]:sm[3]])
		if lit := src[sm[4]:sm[5]]; lit[0] >= '0' && lit[0] <= '9' { // a numbered item keeps its number
			for i, r := range lit {
				cells = append(cells, Cell{r, marker, runeAt(sm[4]) + i})
			}
			cells = append(cells, Cell{' ', text, runeAt(sm[5])})
		}
		mark := runeAt(sm[6])
		cells = append(cells, Cell{box, marker, mark}, Cell{' ', text, mark})
		if withSpans {
			spans = append(spans, span{kind: Checkbox, start: mark - 1, end: mark + 2})
		}
		return append(cells, inlineCells(src[sm[8]:sm[9]], runeAt(sm[8]), text, sp)...), 0, spans
	}
	if sm := bulletItem.FindStringSubmatchIndex(src); sm != nil {
		cells = lead(src[sm[2]:sm[3]])
		cells = append(cells, Cell{'•', marker, runeAt(sm[3])}, Cell{' ', text, runeAt(sm[3])})
		return append(cells, inlineCells(src[sm[4]:sm[5]], runeAt(sm[4]), text, sp)...), 0, spans
	}
	if sm := numberItem.FindStringSubmatchIndex(src); sm != nil {
		cells = lead(src[sm[2]:sm[3]])
		for i, r := range src[sm[4]:sm[5]] {
			cells = append(cells, Cell{r, marker, runeAt(sm[4]) + i})
		}
		cells = append(cells, Cell{' ', text, runeAt(sm[5])})
		return append(cells, inlineCells(src[sm[6]:sm[7]], runeAt(sm[6]), text, sp)...), 0, spans
	}
	if sm := quoteLine.FindStringSubmatchIndex(src); sm != nil {
		q := KindStyle(KindQuote)
		cells = []Cell{{'│', theme.St(theme.Muted), 0}, {' ', q, 0}}
		return append(cells, inlineCells(src[sm[2]:sm[3]], runeAt(sm[2]), q, sp)...), 0, spans
	}
	return inlineCells(src, 0, text, sp), 0, spans
}
