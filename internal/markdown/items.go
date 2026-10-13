package markdown

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// ImageRef is an image reference `![alt](path)` in a source line. Start and End are rune
// offsets in the line.
type ImageRef struct {
	Alt, Path  string
	Start, End int
}

// FindImages returns the image references of a line, in order. A reference in a code span
// is not an image.
func FindImages(line string) []ImageRef {
	if !strings.Contains(line, "![") {
		return nil
	}
	var out []ImageRef
	for _, t := range scanInline(line) {
		if !strings.HasPrefix(t.text, "![") {
			continue
		}
		mid := strings.Index(t.text, "](")
		out = append(out, ImageRef{Alt: t.text[2:mid], Path: t.text[mid+2 : len(t.text)-1], Start: t.a, End: t.b})
	}
	return out
}

// ItemKind is the type of an Item.
type ItemKind int

// The kinds of item that the tab key selects in view mode.
const (
	Checkbox ItemKind = iota
	Link
)

// Item is a checkbox or a link in the document. Start and End are the rune range in the
// source line: "[ ]" of a checkbox, the whole `[text](url)` of a link, or a bare URL.
// A preview Cell is part of the item if its Src is in [Start, End).
type Item struct {
	Line, Start, End int
	Kind             ItemKind
	URL              string // links: the target
	K                int    // checkboxes: the number for ToggleCheckbox
}

// Items returns the checkboxes and the links of the document in reading order. Lines in
// fenced code are not items, because they show as rendered code.
func Items(lines [][]rune) []Item {
	inBlock := make([]bool, len(lines))
	tableAt := map[int]bool{} // table lines with cells, not the separator
	for _, b := range Blocks(lines) {
		for i := b.From; i <= b.To; i++ {
			if b.Table && i != b.From+1 {
				tableAt[i] = true
				continue
			}
			inBlock[i] = true
		}
	}
	var out []Item
	k := 0
	for i, l := range lines {
		if inBlock[i] || len(l) == 0 {
			continue
		}
		var spans []span
		if tableAt[i] {
			for _, c := range splitTableRow(l) {
				inlineCells(c.text, c.off, KindStyle(KindText), &spans)
			}
		} else {
			_, _, spans = previewLine(string(l), true)
		}
		sort.SliceStable(spans, func(a, b int) bool { return spans[a].start < spans[b].start })
		for _, s := range spans {
			it := Item{Line: i, Start: s.start, End: s.end, Kind: s.kind, URL: s.url}
			if s.kind == Checkbox {
				it.K = k
				k++
			}
			out = append(out, it)
		}
	}
	return out
}

// checkboxLine matches a markdown task list item: "- [ ] text", "* [x] text", "1. [ ] text".
var checkboxLine = regexp.MustCompile(`^(\s*(?:[-*+]|\d+[.)])\s+\[)([ xX])(\]\s)`)

// CheckboxLines returns the numbers of the source lines that are checkbox items. Lines
// in fenced code are not items, because the reader shows them as code.
func CheckboxLines(src string) []int {
	var out []int
	inFence := false
	for i, l := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			inFence = !inFence
			continue
		}
		if !inFence && checkboxLine.MatchString(l) {
			out = append(out, i)
		}
	}
	return out
}

// ToggleCheckboxLine changes the checkbox of one line between [ ] and [x]. It reports
// false if the line is not a checkbox item.
func ToggleCheckboxLine(line string) (string, bool) {
	sub := checkboxLine.FindStringSubmatch(line)
	if sub == nil {
		return line, false
	}
	mark := "x"
	if sub[2] != " " {
		mark = " "
	}
	return sub[1] + mark + sub[3] + line[len(sub[0]):], true
}

// ToggleCheckbox changes checkbox k of the markdown src between [ ] and [x].
func ToggleCheckbox(src string, k int) (string, bool) {
	lines := strings.Split(src, "\n")
	idx := CheckboxLines(src)
	if k < 0 || k >= len(idx) {
		return src, false
	}
	var ok bool
	lines[idx[k]], ok = ToggleCheckboxLine(lines[idx[k]])
	return strings.Join(lines, "\n"), ok
}

// Slug returns the GitHub-style anchor of heading text: lower case, with the markup and
// the punctuation removed (not "-" and "_"), and the spaces changed to "-".
func Slug(heading string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(plain(heading)) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// FindHeading returns the line of the heading whose anchor is slug, or -1. Headings in
// fenced code do not count. As on GitHub, the second heading with the same anchor has the
// suffix "-1", the third "-2", and so on.
func FindHeading(lines [][]rune, slug string) int {
	slug = strings.ToLower(slug)
	seen := map[string]int{}
	inFence := false
	for i, l := range lines {
		if isFence(l) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		lv, text := HeadingLevel(string(l))
		if lv == 0 {
			continue
		}
		s := Slug(text)
		n := seen[s]
		seen[s]++
		if n > 0 {
			s += "-" + strconv.Itoa(n)
		}
		if s == slug {
			return i
		}
	}
	return -1
}
