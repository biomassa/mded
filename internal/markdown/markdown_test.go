package markdown

import (
	"reflect"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func runes(s string) [][]rune {
	var out [][]rune
	for _, l := range strings.Split(s, "\n") {
		out = append(out, []rune(l))
	}
	return out
}

func text(cells []Cell) string {
	var b strings.Builder
	for _, c := range cells {
		b.WriteRune(c.R)
	}
	return b.String()
}

func TestPreviewCells(t *testing.T) {
	cases := []struct {
		src, want string
		level     int
	}{
		{"# Title", " Title", 1},
		{"### A **b** c", " A b c", 3},
		{"- item", "• item", 0},
		{"  * item", "  • item", 0},
		{"- [ ] todo", "☐ todo", 0},
		{"- [x] done", "☑ done", 0},
		{"1. [ ] num", "1. ☐ num", 0},
		{"2. two", "2. two", 0},
		{"> quote", "│ quote", 0},
		{"a `code` **b** ~~c~~ *d* e", "a code b c d e", 0},
		{"snake_case_name", "snake_case_name", 0},
		{"see [site](http://x.y) now", "see site now", 0},
		{"see http://x.y/z now", "see http://x.y/z now", 0},
		{"pic ![cat](a/b.png) end", "pic 📷 cat end", 0},
		{"![](a/b.png)", "📷 b.png", 0},
		{"---", "─", -1},
	}
	for _, c := range cases {
		cells, level := PreviewCells(c.src)
		if got := text(cells); got != c.want || level != c.level {
			t.Errorf("%q: got %q level %d, want %q level %d", c.src, got, level, c.want, c.level)
		}
	}
}

func TestPreviewSrc(t *testing.T) {
	// Link text cells map to the source runes of the text.
	cells, _ := PreviewCells("go [ab](u) x")
	var srcs []int
	for _, c := range cells {
		if c.R == 'a' || c.R == 'b' {
			srcs = append(srcs, c.Src)
		}
	}
	if !reflect.DeepEqual(srcs, []int{4, 5}) {
		t.Errorf("link src = %v", srcs)
	}
	// Checkbox marker cells have the position of the mark inside the brackets.
	cells, _ = PreviewCells("- [ ] t")
	if cells[0].R != '☐' || cells[0].Src != 3 || cells[1].Src != 3 || cells[2].Src != 6 {
		t.Errorf("checkbox cells = %+v", cells[:3])
	}
	// Non-ASCII text keeps rune offsets.
	cells, _ = PreviewCells("- é [l](u)")
	for _, c := range cells {
		if c.R == 'l' && c.Src != 5 {
			t.Errorf("l src = %d", c.Src)
		}
	}
	// Image alt cells.
	cells, _ = PreviewCells("![ab](p)")
	if text(cells) != "📷 ab" || cells[2].Src != 2 || cells[3].Src != 3 {
		t.Errorf("image cells %q %+v", text(cells), cells)
	}
}

func TestImageTakesPriorityOverLink(t *testing.T) {
	cells, _ := PreviewCells("![a](p.png)")
	if strings.Contains(text(cells), "a") && !strings.HasPrefix(text(cells), "📷") {
		t.Errorf("got %q", text(cells))
	}
	refs := FindImages("x ![a b](dir/p.png) y ![](q.jpg) `![c](d)`")
	want := []ImageRef{{"a b", "dir/p.png", 2, 19}, {"", "q.jpg", 22, 32}}
	if !reflect.DeepEqual(refs, want) {
		t.Errorf("refs = %+v", refs)
	}
}

func TestBlocks(t *testing.T) {
	doc := runes("a\n```go\nx\n```\nb\n| h | i |\n|---|---|\n| 1 | 2 |\nc\n```\nopen")
	got := Blocks(doc)
	want := []Block{{1, 3, false}, {5, 7, true}, {9, 10, false}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("blocks = %v", got)
	}
}

func TestLineKinds(t *testing.T) {
	k := LineKinds("- a `c` **b**", false)
	if k[0] != KindMarker || k[4] != KindCode || k[8] != KindBold {
		t.Errorf("kinds = %v", k)
	}
	if k := LineKinds("# h", false); k[0] != KindHeading {
		t.Error("heading")
	}
	if k := LineKinds("x", true); k[0] != KindFence {
		t.Error("fence")
	}
}

func TestItems(t *testing.T) {
	doc := runes("# T [h](#t)\n- [ ] one\n- [x] two [l](http://a) and http://b.c\n```\n- [ ] no [l](u)\n```\nplain ![i](p.png) [m](n.md)")
	items := Items(doc)
	type w struct {
		line, start, end int
		kind             ItemKind
		url              string
		k                int
	}
	want := []w{
		{0, 4, 11, Link, "#t", 0},
		{1, 2, 5, Checkbox, "", 0},
		{2, 2, 5, Checkbox, "", 1},
		{2, 10, 23, Link, "http://a", 0},
		{2, 28, 38, Link, "http://b.c", 0},
		{6, 18, 27, Link, "n.md", 0},
	}
	if len(items) != len(want) {
		t.Fatalf("items = %+v", items)
	}
	for i, it := range items {
		g := w{it.Line, it.Start, it.End, it.Kind, it.URL, it.K}
		if g != want[i] {
			t.Errorf("item %d = %+v, want %+v", i, g, want[i])
		}
	}
	// Cells of an item have Src in its range.
	cells, _ := PreviewCells("- [x] two [l](http://a)")
	in := 0
	for _, c := range cells {
		if c.Src >= 2 && c.Src < 5 {
			in++
		}
	}
	if in != 2 { // the box and the space after it
		t.Errorf("cells in checkbox range = %d", in)
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Hello World":          "hello-world",
		"What's new? (v2.0)":   "whats-new-v20",
		"a - b_c":              "a---b_c",
		"The `code` [link](x)": "the-code-link",
		"Ünï Çode":             "ünï-çode",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
	doc := runes("# A\n```\n# B\n```\n## A\n## A")
	if FindHeading(doc, "a") != 0 || FindHeading(doc, "a-1") != 4 || FindHeading(doc, "a-2") != 5 || FindHeading(doc, "b") != -1 {
		t.Error("FindHeading")
	}
}

func TestToggleCheckbox(t *testing.T) {
	src := "- [ ] a\n```\n- [ ] x\n```\n1. [x] b\ntext"
	got, ok := ToggleCheckbox(src, 1)
	if !ok || got != "- [ ] a\n```\n- [ ] x\n```\n1. [ ] b\ntext" {
		t.Errorf("got %q", got)
	}
	if got, ok := ToggleCheckbox(src, 0); !ok || !strings.HasPrefix(got, "- [x] a") {
		t.Errorf("got %q", got)
	}
	if _, ok := ToggleCheckbox(src, 2); ok {
		t.Error("k out of range")
	}
	if !reflect.DeepEqual(CheckboxLines(src), []int{0, 4}) {
		t.Error("CheckboxLines")
	}
	if l, ok := ToggleCheckboxLine("  * [X] k"); !ok || l != "  * [ ] k" {
		t.Errorf("line = %q", l)
	}
	if _, ok := ToggleCheckboxLine("no"); ok {
		t.Error("not a checkbox")
	}
}

func lineText(rows []Row) []string {
	var out []string
	for _, r := range rows {
		switch {
		case r.Img != nil:
			out = append(out, "IMG")
		case r.Pre != "":
			out = append(out, ansi.Strip(strings.TrimRight(r.Pre, " ")))
		default:
			out = append(out, text(r.Cells))
		}
	}
	return out
}

func TestLayoutAsSource(t *testing.T) {
	c := NewCache()
	c.Reset()
	doc := runes("# Head\n- item\n```go\nx := 1\n```\nend")
	rows := c.Layout(doc, LayoutOpts{Width: 40})
	for _, r := range rows {
		if r.Source {
			t.Fatal("no source line expected")
		}
	}
	if rows[0].Fill != 1 || rows[0].Line != 0 || rows[1].Line != 1 {
		t.Errorf("rows = %+v", rows[:2])
	}
	// The block rows have source lines inside the block.
	for _, r := range rows {
		if r.Line < 0 || r.Line >= len(doc) {
			t.Errorf("bad line %d", r.Line)
		}
	}
	// One line of the block as source shows the whole block as source.
	rows = c.Layout(doc, LayoutOpts{Width: 40, AsSource: func(i int) bool { return i == 3 }})
	got := lineText(rows)
	if !reflect.DeepEqual(got[2:5], []string{"```go", "x := 1", "```"}) {
		t.Errorf("got %q", got)
	}
	for _, r := range rows[2:5] {
		if !r.Source {
			t.Error("block row is not source")
		}
	}
	if rows[0].Source || rows[1].Source {
		t.Error("other lines must stay preview")
	}
	if CursorRow(rows, 3, 2) != 3 || CursorRow(rows, 0, 0) != 0 || CursorRow(rows, 99, 0) != -1 {
		t.Error("CursorRow")
	}
}

func TestLayoutSourceWrapAndDecorate(t *testing.T) {
	c := NewCache()
	n := 0
	rows := c.Layout(runes("aaaa bbbb cccc dddd"), LayoutOpts{
		Width:    10,
		AsSource: func(int) bool { return true },
		Decorate: func(line, col int, st lipgloss.Style) lipgloss.Style { n++; return st },
	})
	// Width 10 less one cell of indent: a row has at most 9 runes.
	if len(rows) != 3 || rows[1].Start != 5 || n != 19 {
		t.Errorf("rows %d start %d n %d", len(rows), rows[1].Start, n)
	}
	if CursorRow(rows, 0, 7) != 1 || CursorRow(rows, 0, 3) != 0 {
		t.Error("CursorRow wrapped")
	}
}

func TestLayoutPlain(t *testing.T) {
	c := NewCache()
	rows := c.Layout(runes("# not heading\n```\n![a](b)"), LayoutOpts{
		Width: 40, Plain: true,
		ImageSize: func(ImageRef) (int, int, bool) { return 4, 2, true },
	})
	if len(rows) != 3 {
		t.Fatalf("rows = %d", len(rows))
	}
	for _, r := range rows {
		if !r.Source || r.Fill != 0 || r.Img != nil {
			t.Errorf("row = %+v", r)
		}
	}
	if rows[0].Cells[0].St.Render("x") != KindStyle(KindText).Render("x") {
		t.Error("plain style")
	}
}

func TestLayoutImages(t *testing.T) {
	c := NewCache()
	size := func(ref ImageRef) (int, int, bool) {
		if ref.Path == "bad.png" {
			return 0, 0, false
		}
		return 4, 3, true
	}
	doc := runes("![a](a.png)\ntext ![b](b.png) more\n  ![c](c.png)  \n![d](bad.png)\n![e](e.png) ![f](f.png)")
	rows := c.Layout(doc, LayoutOpts{Width: 40, ImageSize: size})
	var got []string
	for _, r := range rows {
		if r.Img != nil {
			got = append(got, "IMG"+r.Img.Alt)
			if r.Line < 0 {
				t.Error("line")
			}
		} else {
			got = append(got, ansi.Strip(strings.TrimSpace(r.Pre)))
		}
	}
	want := []string{
		"IMGa", "IMGa", "IMGa", // alone: image rows replace the text
		"text 📷 b more", "IMGb", "IMGb", "IMGb", // inline: text, then rows
		"IMGc", "IMGc", "IMGc", // alone with spaces
		"📷 d", // size not ok: text only
		"📷 e 📷 f", "IMGe", "IMGe", "IMGe", "IMGf", "IMGf", "IMGf",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if rows[1].ImgRow != 1 || rows[1].Line != 0 || rows[4].Line != 1 {
		t.Errorf("img rows: %+v %+v", rows[1], rows[4])
	}
	// A line that shows source gets no image rows.
	rows = c.Layout(doc, LayoutOpts{Width: 40, ImageSize: size, AsSource: func(i int) bool { return i == 0 || i == 1 }})
	if rows[0].Img != nil || !rows[0].Source || rows[1].Img != nil || rows[2].Img == nil || rows[2].Line != 2 {
		t.Errorf("source lines: %+v", rows[:3])
	}
}

func TestRenderRow(t *testing.T) {
	if RenderRow(Row{Img: &ImageRef{}}, 10, 0) != "" {
		t.Error("image row")
	}
	r := Row{Cells: []Cell{{R: 'a'}, {R: 'b'}}, Source: true}
	if got := ansi.Strip(RenderRow(r, 10, 2)); got != "  ab " {
		t.Errorf("got %q", got)
	}
	cells, lv := PreviewCells("# Hi")
	got := ansi.Strip(RenderRow(Row{Cells: cells, Fill: lv}, 10, -1))
	if len([]rune(got)) != 11 || !strings.HasPrefix(got, "  Hi") {
		t.Errorf("bar width %d: %q", len([]rune(got)), got)
	}
}

func TestRender(t *testing.T) {
	c := NewCache()
	out := c.Render("# H\n\nsome text\n\n```go\nx := 1\n```", 30)
	if len(out) < 3 || !strings.Contains(ansi.Strip(out[0]), "H") {
		t.Errorf("out = %q", out)
	}
	if got := c.PreviewRows("---", 5); len(got) != 1 || ansi.Strip(got[0].Pre) != "  ────" {
		t.Errorf("rule = %+v", got)
	}
}

func TestTable(t *testing.T) {
	doc := runes("| Name | Qty | Note |\n|:-----|:---:|-----:|\n| **a** | 1 | [l](http://u) |\n| bbbbbbbb | 22 | x |")
	c := NewCache()
	rows := c.Layout(doc, LayoutOpts{Width: 60})
	var got []string
	for _, r := range rows {
		got = append(got, ansi.Strip(RenderRow(Row{Cells: r.Cells}, 60, -1))[1:]) // without the margin
	}
	if len(got) != 4 {
		t.Fatalf("rows = %q", got)
	}
	if got[0] != "   Name     │ Qty │ Note " || got[1] != "  ──────────┼─────┼──────" {
		t.Errorf("rows = %q", got)
	}
	for i, r := range rows {
		if r.Line != i || r.Source {
			t.Errorf("row %d line %d", i, r.Line)
		}
	}
	// All rows have the same width, and the link cells map to the source.
	w := len([]rune(got[0]))
	for _, g := range got {
		if len([]rune(g)) != w {
			t.Errorf("width differs: %q", got)
		}
	}
	items := Items(doc)
	if len(items) != 1 || items[0].Line != 2 || items[0].Kind != Link || items[0].URL != "http://u" {
		t.Fatalf("items = %+v", items)
	}
	n := 0
	for _, cl := range rows[2].Cells {
		if cl.Src >= items[0].Start && cl.Src < items[0].End {
			n++
			if cl.R != 'l' {
				t.Errorf("cell %q in item", cl.R)
			}
		}
	}
	if n != 1 {
		t.Errorf("cells in item = %d", n)
	}
	// Alignment: the right column puts text at the right edge, the center column centers.
	if !strings.HasSuffix(strings.TrimRight(got[2], " "), "l") || !strings.Contains(got[2], "│  1  │") {
		t.Errorf("align: %q", got[2])
	}
}

func TestTableShrink(t *testing.T) {
	doc := runes("| a | note |\n|---|---|\n| b | long text here that wraps around |")
	rows := NewCache().Layout(doc, LayoutOpts{Width: 20})
	if len(rows) < 4 {
		t.Fatalf("rows = %d", len(rows))
	}
	for i, r := range rows {
		s := ansi.Strip(RenderRow(Row{Cells: r.Cells}, 20, -1))
		if len(r.Cells) > 20 {
			t.Errorf("row %d too wide: %q", i, s)
		}
		if i >= 2 && r.Line != 2 {
			t.Errorf("row %d line %d", i, r.Line)
		}
	}
	// A table with the cursor shows as source.
	rows = NewCache().Layout(doc, LayoutOpts{Width: 20, AsSource: func(i int) bool { return i == 1 }})
	if len(rows) < 3 || !rows[0].Source {
		t.Errorf("source rows = %d", len(rows))
	}
}

// Inline markdown inside bold, italic, strike, and link text renders too, and a link in
// bold text is an item.
func TestNestedInline(t *testing.T) {
	for src, want := range map[string]string{
		"**Search with `/`**, as in":  "Search with /, as in",
		"*see `x` here*":              "see x here",
		"~~old **bold**~~":            "old bold",
		"[**bold** link](u)":          "bold link",
		"**a [link](http://x) here**": "a link here",
	} {
		cells, _ := PreviewCells(src)
		if got := text(cells); got != want {
			t.Errorf("%q: got %q, want %q", src, got, want)
		}
	}
	cells, _ := PreviewCells("**Search with `/`**")
	for _, c := range cells {
		if c.R == '/' && c.Src != 15 {
			t.Errorf("/ src = %d", c.Src)
		}
	}
	items := Items([][]rune{[]rune("**a [link](http://x) here**")})
	if len(items) != 1 || items[0].URL != "http://x" {
		t.Errorf("items %+v", items)
	}
}
