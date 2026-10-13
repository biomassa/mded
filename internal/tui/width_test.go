package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// widthDoc has all kinds of content: headings, inline marks, lists, checkboxes, a quote, a
// rule, a table, code, and pictures that cannot show (missing files).
const widthDoc = "# Title\n\nSome **bold** and *italic* with `code` and a [link](x.md).\n\n" +
	"![missing picture](nope.png)\n\nText ![inline](nope2.png) more.\n\n![](a/b.png)\n\n" +
	"- item\n- [ ] task\n- [x] done\n1. [ ] numbered\n\n> quote\n\n---\n\n" +
	"| A | B |\n|---|---|\n| x | y |\n\n```sh\ncurl -LO https://x\n```\n\n## Two\n\n###### Six\n"

// Each drawn row has the same width by the grapheme method and by the wcwidth method. The
// renderer of bubbletea uses wcwidth in terminals without grapheme mode 2027 (kitty), and
// the terminal draws what it measures. If the two methods disagree on a row, the row wraps
// in the terminal, and the screen moves down by one row.
func TestRowsHaveOneWidth(t *testing.T) {
	m := testModelSize(t, writeTemp(t, "a.md", widthDoc, 0o644), 80, 50)
	for _, mode := range []string{"view", "edit"} {
		if mode == "edit" {
			press(m, "e")
		}
		for y, line := range strings.Split(m.View().Content, "\n") {
			plain := ansi.Strip(line)
			if g, wc := ansi.StringWidth(plain), ansi.StringWidthWc(plain); g != wc {
				t.Errorf("%s row %d: grapheme width %d, wcwidth %d: %q", mode, y, g, wc, plain)
			}
			if w := ansi.StringWidthWc(plain); w > 80 {
				t.Errorf("%s row %d: width %d is more than the screen: %q", mode, y, w, plain)
			}
		}
	}
}
