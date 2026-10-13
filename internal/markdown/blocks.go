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
	isRow := func(i int) bool { return hasPrefixTrim(lines[i], "|") }
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
		if isRow(i) && i+1 < len(lines) && tableSep.MatchString(string(lines[i+1])) {
			j := i + 1
			for j+1 < len(lines) && isRow(j+1) {
				j++
			}
			out = append(out, Block{From: i, To: j, Table: true})
			i = j
		}
	}
	return out
}

// plain strips markdown links and bold markers for single-line display.
func plain(s string) string {
	s = mdLink.ReplaceAllString(s, "$1")
	s = strings.ReplaceAll(s, "**", "")
	return strings.ReplaceAll(s, "\n", " ")
}
