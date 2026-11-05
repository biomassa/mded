package tui

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// memDoc returns a markdown document of about n bytes: headings, paragraphs, lists, a
// fenced code block every 300 lines, and a table every 500 lines.
func memDoc(n int) string {
	var sb strings.Builder
	for i := 0; sb.Len() < n; i++ {
		switch {
		case i%500 == 0:
			sb.WriteString("| a | b |\n|---|---|\n| x | y |\n")
		case i%300 == 0:
			sb.WriteString("```go\nfunc f() int { return 1 }\n```\n")
		case i%50 == 0:
			fmt.Fprintf(&sb, "## Part %d\n", i)
		case i%7 == 0:
			fmt.Fprintf(&sb, "- item %d with **bold** and `code`\n", i)
		default:
			fmt.Fprintf(&sb, "Line %d of the text, with a [link](https://example.com/%d) in it.\n", i, i)
		}
	}
	return sb.String()
}

// heapNow returns the live heap after a garbage collection.
func heapNow() uint64 {
	var ms runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&ms)
	return ms.HeapAlloc
}

// A large document must not need memory in proportion to its rows: only the rows on the
// screen are built, and an undo step keeps only the lines that it changed. Before this
// rule, a 1 MB document needed more than 1 GB.
func TestMemoryOfLargeDocument(t *testing.T) {
	p := writeTemp(t, "big.md", memDoc(1<<20), 0o644)
	before := heapNow()
	m := testModelSize(t, p, 120, 40)
	n := len(m.doc.buf.Lines)
	m.doc.top, m.doc.sub = n/2, 0 // the middle
	press(m, "j")
	press(m, "G") // the end
	press(m, "g", "e")
	for i := 0; i < 220; i++ { // separate undo steps: a letter, then a move
		press(m, "x", "down")
	}
	press(m, "esc")
	grow := int64(heapNow()) - int64(before)
	t.Logf("%d lines: heap growth %d MB", n, grow>>20)
	if grow > 50<<20 {
		t.Errorf("heap growth %d MB, more than 50 MB", grow>>20)
	}
	if m.doc.buf.UndoDepth() < 200 {
		t.Errorf("undo depth %d, want 200", m.doc.buf.UndoDepth())
	}
	runtime.KeepAlive(m)
}
