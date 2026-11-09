package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/biomassa/mded/internal/config"
)

// benchLines is the size of the benchmark documents.
const benchLines = 10000

// benchCode returns a C-like source of n lines.
func benchCode(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		switch i % 8 {
		case 0:
			fmt.Fprintf(&b, "/* function %d: it computes a value from the arguments */\n", i)
		case 1:
			fmt.Fprintf(&b, "static int func_%d(const char *name, size_t len, struct ctx *c)\n", i)
		case 2:
			b.WriteString("{\n")
		case 3:
			fmt.Fprintf(&b, "\tif (len > %d && name[0] != '\\0') return c->table[len %% 17] + %d;\n", i, i)
		case 4:
			b.WriteString("\tfor (size_t k = 0; k < len; k++) c->sum += (unsigned char)name[k];\n")
		case 5:
			b.WriteString("\treturn (int)(c->sum & 0x7fffffff);\n")
		case 6:
			b.WriteString("}\n")
		default:
			b.WriteString("\n")
		}
	}
	return b.String()
}

// benchMarkdown returns a markdown document of about n lines with headings, paragraphs,
// lists, links, code blocks, and tables.
func benchMarkdown(n int) string {
	var b strings.Builder
	for i := 0; strings.Count(b.String(), "\n") < n; i++ {
		fmt.Fprintf(&b, "## Section %d\n\n", i)
		b.WriteString("This paragraph has **bold text**, *italic text*, `inline code`, and a [link](https://example.com/page). ")
		b.WriteString("It is long enough to wrap on a narrow screen, so that the layout must wrap it.\n\n")
		b.WriteString("- [ ] a task that is open\n- [x] a task that is done\n- an item with a [link](other.md)\n\n")
		b.WriteString("```go\nfunc f(x int) int {\n\treturn x * 2 // double\n}\n```\n\n")
		b.WriteString("| Name | Value | Note |\n|---|---|---|\n| alpha | 1 | first |\n| beta | 22 | second |\n\n")
		b.WriteString("> A quote with *style*.\n\n")
	}
	return b.String()
}

// benchModel opens a document of benchLines lines in a model of 160x45 cells.
func benchModel(b *testing.B, name, text string) *Model {
	b.Helper()
	dir := b.TempDir()
	b.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		b.Fatal(err)
	}
	m, err := New(Options{
		Config:           config.Defaults(),
		Path:             path,
		SaveTheme:        func(string) error { return nil },
		SaveSidebarWidth: func(int) error { return nil },
		OpenURL:          func(string) error { return nil },
		SyncLoad:         true,
	})
	if err != nil {
		b.Fatal(err)
	}
	m.noHold = true // each event builds its frame: the benchmark measures the frame
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 45})
	m.View()
	return m
}

// benchWheel measures one wheel event and the View after it, as bubbletea does for each
// message. It scrolls to the end of the document and back.
func benchWheel(b *testing.B, name, text string) {
	m := benchModel(b, name, text)
	x, y := m.docRect().x+10, m.docRect().y+5
	down := true
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		btn := tea.MouseWheelDown
		if !down {
			btn = tea.MouseWheelUp
		}
		top := m.doc.top
		m.Update(tea.MouseWheelMsg{X: x, Y: y, Button: btn})
		m.View()
		if m.doc.top == top && m.doc.off == 0 { // an end of the document: go back
			down = !down
		}
	}
}

func BenchmarkWheelCode(b *testing.B)     { benchWheel(b, "big.h", benchCode(benchLines)) }
func BenchmarkWheelMarkdown(b *testing.B) { benchWheel(b, "big.md", benchMarkdown(benchLines)) }

// BenchmarkViewCode measures View with no change, as after a message that changes nothing.
func BenchmarkViewCode(b *testing.B) {
	m := benchModel(b, "big.h", benchCode(benchLines))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.View()
	}
}
