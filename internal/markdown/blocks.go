package markdown

import (
	"strings"
	"unicode"
)

// Block is a fenced code block or a table: source lines From and To, both included.
// Table is true for a table.
type Block struct {
	From, To int
	Table    bool
}

// hasPrefixTrim reports whether l, without its leading space, starts with p.
func hasPrefixTrim(l []rune, p string) bool {
	i := 0
	for i < len(l) && unicode.IsSpace(l[i]) {
		i++
	}
	n := 0
	for _, r := range p {
		if i+n >= len(l) || l[i+n] != r {
			return false
		}
		n++
	}
	return true
}

// isFence reports whether the line starts or ends a fenced code block.
func isFence(l []rune) bool { return hasPrefixTrim(l, "```") }

// Blocks finds the fenced code blocks and the tables of the document. An open fence
// goes to the last line.
func Blocks(lines [][]rune) []Block {
	var out []Block
	for i := 0; i < len(lines); i++ {
		if isFence(lines[i]) {
			j := i + 1
			for j < len(lines) && !isFence(lines[j]) {
				j++
			}
			out = append(out, Block{From: i, To: min(j, len(lines)-1)})
			i = j
			continue
		}
		if isTableStart(lines, i) {
			j := i + 1
			for j+1 < len(lines) && hasPipe(lines[j+1]) && !isFence(lines[j+1]) {
				j++
			}
			out = append(out, Block{From: i, To: j, Table: true})
			i = j
		}
	}
	return out
}

// hasPipe reports whether l has a "|" that is not escaped.
func hasPipe(l []rune) bool {
	for i, r := range l {
		if r == '|' && (i == 0 || l[i-1] != '\\') {
			return true
		}
	}
	return false
}

// isTableStart reports whether line i is the header of a table, as in GitHub Flavored
// Markdown: a line with a "|", then a delimiter row with the same number of cells. A
// heading, a list item, or a quote is not a header.
func isTableStart(lines [][]rune, i int) bool {
	if !hasPipe(lines[i]) || i+1 >= len(lines) || !tableDelim.MatchString(string(lines[i+1])) {
		return false
	}
	src := string(lines[i])
	if lv, _ := HeadingLevel(src); lv > 0 || bulletItem.MatchString(src) || numberItem.MatchString(src) || quoteLine.MatchString(src) {
		return false
	}
	return len(splitTableRow(lines[i])) == len(splitTableRow(lines[i+1]))
}

// plain strips markdown links and bold markers for single-line display.
func plain(s string) string {
	s = mdLink.ReplaceAllString(s, "$1")
	s = strings.ReplaceAll(s, "**", "")
	return strings.ReplaceAll(s, "\n", " ")
}
