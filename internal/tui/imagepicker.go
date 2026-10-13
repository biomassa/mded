package tui

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	_ "golang.org/x/image/webp" // Register the WebP decoder for image.DecodeConfig.

	"github.com/biomassa/mded/internal/keymap"
	"github.com/biomassa/mded/internal/kitty"
	"github.com/biomassa/mded/internal/theme"
	"github.com/biomassa/mded/internal/ui"
)

// The image picker (ctrl+p in edit mode) is a popup that lists the directories and the image
// files of a directory, with a preview of the image under the cursor. A pick opens the alt
// text prompt in the status line. enter in that prompt inserts the image line below the
// cursor line. esc in that prompt goes back to the picker.

const (
	pickerTitle = "Insert image"
	pickerHint  = "↑/↓ move · → open · ← up · type to filter · enter pick · esc cancel"
	altPrompt   = "alt text: "
	parentName  = "../"
)

// pickEntry is one row of the picker list: a directory or an image file.
type pickEntry struct {
	name string
	dir  bool
}

// imgInfo is what the preview shows without graphics: the file size and the pixel size.
type imgInfo struct {
	size int64
	w, h int
	err  error
}

// imagePicker is the open picker.
type imagePicker struct {
	dir     string
	entries []pickEntry // the entries of dir, without the parent row, sorted
	filter  []rune
	cur     int // index in the visible rows. Row 0 is the parent row.
	off     int // first visible row
	info    map[string]imgInfo

	altOn  bool   // the alt text prompt is open
	alt    []rune // the text of the prompt
	altCur int
	chosen string // the file of the prompt

	prev     placedImage // the preview image that the terminal shows
	prevPath string
}

// openImagePicker opens the picker in the directory of the document, or in the working
// directory for a document without a path.
func (m *Model) openImagePicker() {
	dir := ""
	if m.doc.abs != "" {
		dir = filepath.Dir(m.doc.abs)
	} else if wd, err := os.Getwd(); err == nil {
		dir = wd
	} else {
		dir = "."
	}
	p := &imagePicker{info: map[string]imgInfo{}}
	if err := p.setDir(dir, m.tree.ShowHidden()); err != nil {
		m.status("cannot read "+dir+": "+errText(err), true)
		return
	}
	m.img = p
}

// readEntries lists the directories and the image files of dir. Directories come first,
// and each group is sorted by name without regard to case.
func readEntries(dir string, hidden bool) ([]pickEntry, error) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []pickEntry
	for _, de := range des {
		name := de.Name()
		if !hidden && strings.HasPrefix(name, ".") {
			continue
		}
		isDir := de.IsDir()
		if de.Type()&os.ModeSymlink != 0 {
			if st, err := os.Stat(filepath.Join(dir, name)); err == nil {
				isDir = st.IsDir()
			}
		}
		if isDir || isImage(name) {
			out = append(out, pickEntry{name, isDir})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].dir != out[j].dir {
			return out[i].dir
		}
		return strings.ToLower(out[i].name) < strings.ToLower(out[j].name)
	})
	return out, nil
}

// setDir reads dir and makes it the current directory. The filter and the cursor reset.
// On an error nothing changes.
func (p *imagePicker) setDir(dir string, hidden bool) error {
	es, err := readEntries(dir, hidden)
	if err != nil {
		return err
	}
	p.dir, p.entries = dir, es
	p.filter, p.cur, p.off = nil, 0, 0
	return nil
}

// rows returns the visible entries: the parent row, then the entries that match the filter.
func (p *imagePicker) rows() []pickEntry {
	out := []pickEntry{{parentName, true}}
	f := strings.ToLower(string(p.filter))
	for _, e := range p.entries {
		if f == "" || strings.Contains(strings.ToLower(e.name), f) {
			out = append(out, e)
		}
	}
	return out
}

// selected returns the row under the cursor.
func (p *imagePicker) selected() pickEntry {
	rs := p.rows()
	return rs[max(0, min(p.cur, len(rs)-1))]
}

// selectedFile returns the path of the image file under the cursor, or "".
func (p *imagePicker) selectedFile() string {
	if e := p.selected(); !e.dir {
		return filepath.Join(p.dir, e.name)
	}
	return ""
}

// pickerRect is the popup: 80% of the screen width and height, in the middle.
func (m *Model) pickerRect() rect {
	w := min(m.width, max(30, m.width*8/10))
	h := min(m.height, max(8, m.height*8/10))
	return rect{(m.width - w) / 2, (m.height - h) / 2, w, h}
}

// pickerGeom holds the sizes inside the popup, in cells.
type pickerGeom struct {
	inner    int // the width between the borders
	listW    int // the width of the list column
	prevW    int // the width of the preview column
	listRows int // the rows of the list and of the preview
}

// pickerGeom splits the popup. The list takes 40% of the popup width, the preview 60%. One
// column of the preview is the separator line.
func (m *Model) pickerGeom() pickerGeom {
	r := m.pickerRect()
	inner := max(1, r.w-2)
	listW := max(1, r.w*40/100-1)
	return pickerGeom{inner, listW, max(1, inner-listW), max(1, r.h-2-3)} // path line, hint line, spare
}

// keepVisible scrolls the list so that the cursor row shows.
func (p *imagePicker) keepVisible(rows int) {
	n := len(p.rows())
	p.cur = max(0, min(p.cur, n-1))
	if p.cur < p.off {
		p.off = p.cur
	}
	if p.cur >= p.off+rows {
		p.off = p.cur - rows + 1
	}
	p.off = max(0, min(p.off, max(0, n-rows)))
}

// imageKey handles a key while the picker or its alt text prompt is open.
func (m *Model) imageKey(msg tea.KeyPressMsg) {
	p := m.img
	if p.altOn {
		m.altKey(msg)
		return
	}
	k := msg.String()
	rows := m.pickerGeom().listRows
	switch k {
	case "esc":
		m.closePicker()
		return
	case "up":
		p.cur--
	case "down":
		p.cur++
	case "right":
		m.pickerEnter(true)
	case "left":
		m.pickerUp()
	case "enter":
		m.pickerEnter(false)
	case "backspace":
		if len(p.filter) > 0 {
			p.filter = p.filter[:len(p.filter)-1]
			p.afterFilter()
		}
	default:
		if keymap.IsTyped(keymap.Key{Name: k, Text: msg.Text}) {
			p.filter = append(p.filter, []rune(msg.Text)...)
			p.afterFilter()
		}
	}
	if m.img != nil {
		m.img.keepVisible(rows)
	}
}

// afterFilter puts the cursor on the first match, or on the parent row if nothing matches
// or the filter is empty.
func (p *imagePicker) afterFilter() {
	p.cur, p.off = 0, 0
	if len(p.filter) > 0 && len(p.rows()) > 1 {
		p.cur = 1
	}
}

// closePicker closes the picker and its images.
func (m *Model) closePicker() { m.img = nil }

// pickerUp goes to the parent directory. The cursor goes to the directory that was left.
func (m *Model) pickerUp() {
	p := m.img
	parent := filepath.Dir(p.dir)
	if parent == p.dir {
		return
	}
	from := filepath.Base(p.dir)
	if err := p.setDir(parent, m.tree.ShowHidden()); err != nil {
		m.status("cannot read "+parent+": "+errText(err), true)
		return
	}
	for i, e := range p.rows() {
		if e.dir && e.name == from {
			p.cur = i
		}
	}
}

// pickerEnter opens a directory. For a file, it picks the file, unless dirOnly is set (the
// right arrow does nothing on a file).
func (m *Model) pickerEnter(dirOnly bool) {
	p := m.img
	e := p.selected()
	switch {
	case e.name == parentName:
		m.pickerUp()
	case e.dir:
		dir := filepath.Join(p.dir, e.name)
		if err := p.setDir(dir, m.tree.ShowHidden()); err != nil {
			m.status("cannot read "+dir+": "+errText(err), true)
		}
	case !dirOnly:
		p.chosen = filepath.Join(p.dir, e.name)
		base := filepath.Base(p.chosen)
		p.alt = []rune(strings.TrimSuffix(base, filepath.Ext(base)))
		p.altCur = len(p.alt)
		p.altOn = true
	}
}

// altKey handles a key of the alt text prompt.
func (m *Model) altKey(msg tea.KeyPressMsg) {
	p := m.img
	k := msg.String()
	switch k {
	case "esc":
		p.altOn = false
	case "enter":
		path := p.chosen
		m.img = nil
		m.insertImage(string(p.alt), path)
	case "left":
		p.altCur = max(0, p.altCur-1)
	case "right":
		p.altCur = min(len(p.alt), p.altCur+1)
	case "home":
		p.altCur = 0
	case "end":
		p.altCur = len(p.alt)
	case "backspace":
		if p.altCur > 0 {
			p.alt = append(p.alt[:p.altCur-1], p.alt[p.altCur:]...)
			p.altCur--
		}
	case "delete":
		if p.altCur < len(p.alt) {
			p.alt = append(p.alt[:p.altCur], p.alt[p.altCur+1:]...)
		}
	default:
		if keymap.IsTyped(keymap.Key{Name: k, Text: msg.Text}) {
			p.altInsert(msg.Text)
		}
	}
}

// altInsert puts text at the cursor of the alt text prompt.
func (p *imagePicker) altInsert(text string) {
	rs := []rune(oneLine(text))
	p.alt = append(p.alt[:p.altCur:p.altCur], append(rs, p.alt[p.altCur:]...)...)
	p.altCur += len(rs)
}

// pickerPaste handles pasted text: the alt text, or the filter.
func (m *Model) pickerPaste(text string) {
	p := m.img
	if p.altOn {
		p.altInsert(text)
		return
	}
	p.filter = append(p.filter, []rune(oneLine(text))...)
	p.afterFilter()
	p.keepVisible(m.pickerGeom().listRows)
}

// altLine returns the status line text of the alt text prompt and the cursor position.
func (p *imagePicker) altLine() (string, int) {
	return altPrompt + string(p.alt), len([]rune(altPrompt)) + p.altCur
}

// escapePath makes the path of an image line: slashes, and a percent escape for the
// characters that end or change the meaning of the reference.
func escapePath(p string) string {
	p = filepath.ToSlash(p)
	r := strings.NewReplacer("%", "%25", " ", "%20", "(", "%28", ")", "%29")
	return r.Replace(p)
}

// imageRef returns the reference for a file: relative to the directory of the document,
// absolute if the document has no path.
func (m *Model) imageRef(file string) string {
	if m.doc.abs != "" {
		if rel, err := filepath.Rel(filepath.Dir(m.doc.abs), file); err == nil {
			return escapePath(rel)
		}
	}
	return escapePath(file)
}

// insertImage puts the line ![alt](path) below the cursor line. The cursor and the mode
// stay. PushUndo makes one step in the conventional flavor. Inside a vim insert session it
// does nothing, and the line joins the step of the session. In vim normal mode it makes one
// step, as in the conventional flavor.
func (m *Model) insertImage(alt, file string) {
	b := m.doc.buf
	b.Clamp()
	row, col, want := b.Row, b.Col, b.Want
	b.PushUndo(false)
	b.InsertLines(row+1, "!["+alt+"]("+m.imageRef(file)+")")
	b.Row, b.Col, b.Want = row, col, want
	b.Refresh()
	m.apply([]keymap.Effect{keymap.Changed()})
}

// ---- preview ----

// formatSize writes a file size with a unit.
func formatSize(n int64) string {
	const k = 1024
	switch {
	case n < k:
		return fmt.Sprintf("%d B", n)
	case n < k*k:
		return fmt.Sprintf("%.1f KB", float64(n)/k)
	case n < k*k*k:
		return fmt.Sprintf("%.1f MB", float64(n)/(k*k))
	}
	return fmt.Sprintf("%.1f GB", float64(n)/(k*k*k))
}

// readInfo reads the file size and the pixel size of an image. It decodes the header only.
func readInfo(path string) imgInfo {
	st, err := os.Stat(path)
	if err != nil {
		return imgInfo{err: err}
	}
	in := imgInfo{size: st.Size()}
	f, err := os.Open(path)
	if err != nil {
		in.err = err
		return in
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		in.err = err
		return in
	}
	in.w, in.h = cfg.Width, cfg.Height
	return in
}

// pickerSync runs at the end of each Update. It reads the info of the file under the cursor
// and starts the preview image.
func (m *Model) pickerSync() {
	p := m.img
	if p == nil || p.altOn {
		return
	}
	path := p.selectedFile()
	p.prev, p.prevPath = placedImage{}, ""
	if path == "" {
		return
	}
	if _, ok := p.info[path]; !ok {
		p.info[path] = readInfo(path)
	}
	img := m.opts.Images
	if img == nil {
		return
	}
	g := m.pickerGeom()
	cols, rows, ok := img.Size(path, max(1, g.prevW-3), g.listRows)
	if !ok {
		if !m.imgTried[path] {
			m.imgTried[path] = true
			m.cmd(img.Load(path))
		}
		return
	}
	id, cmd := img.Place(path, cols, rows)
	m.cmd(m.afterFirstFrame(cmd))
	if id != 0 {
		p.prev, p.prevPath = placedImage{id, cols}, path
	}
}

// previewLines draws the preview column: the image rows, or the file size and the pixel
// size.
func (m *Model) previewLines(g pickerGeom) []string {
	p := m.img
	var out []string
	path := p.selectedFile()
	switch {
	case path == "":
	case p.prev.id != 0 && p.prevPath == path:
		_, rows, _ := m.opts.Images.Size(path, max(1, g.prevW-3), g.listRows)
		for r := 0; r < rows; r++ {
			out = append(out, "  "+kitty.RowString(p.prev.id, r, p.prev.cols))
		}
	default:
		in := p.info[path]
		dim := theme.St(theme.Text)
		out = append(out, " "+dim.Render(formatSize(in.size)))
		if in.err != nil {
			out = append(out, " "+theme.St(theme.Error).Render("cannot read the image"))
		} else {
			out = append(out, " "+dim.Render(fmt.Sprintf("%d × %d", in.w, in.h)))
		}
	}
	return out
}

// ---- drawing ----

// pickerBox draws the popup.
func (m *Model) pickerBox() string {
	p := m.img
	r := m.pickerRect()
	g := m.pickerGeom()
	rows := p.rows()
	p.off = max(0, min(p.off, max(0, len(rows)-g.listRows)))
	dim := theme.St(theme.Dim)

	// The first line: the directory, and the filter if there is one.
	path := ui.Trunc(p.dir, max(1, g.inner-2))
	if rs := []rune(p.dir); len(rs) > g.inner-2 && g.inner > 4 {
		path = "…" + string(rs[len(rs)-(g.inner-3):])
	}
	first := " " + dim.Render(path)
	if len(p.filter) > 0 {
		first = " " + dim.Render(path) + "  " + theme.St(theme.Fg(theme.Accent)).Render("filter: "+string(p.filter))
	}

	prev := m.previewLines(g)
	sep := dim.Render("│")
	lines := []string{first}
	for i := 0; i < g.listRows; i++ {
		left := strings.Repeat(" ", g.listW)
		if idx := p.off + i; idx < len(rows) {
			left = m.pickerRow(rows[idx], idx == p.cur, g.listW)
		}
		right := ""
		if i < len(prev) {
			right = prev[i]
		}
		right = ui.Pad(right, g.prevW-1, lipgloss.NewStyle())
		lines = append(lines, left+sep+right)
	}
	lines = append(lines, "", " "+dim.Render(ui.Trunc(pickerHint, g.inner-1)))
	title := theme.St(theme.Fg(theme.Accent)).Bold(true).Render(pickerTitle)
	return ui.Box(title, ui.PadLines(lines, g.inner, r.h-2), r.w, theme.Fg(theme.Accent))
}

// pickerRow draws one row of the list. A directory has a slash at its end.
func (m *Model) pickerRow(e pickEntry, selected bool, w int) string {
	name := e.name
	if e.dir && name != parentName {
		name += "/"
	}
	label := ui.Trunc(name, max(1, w-2))
	if selected {
		return lipgloss.NewStyle().Background(theme.C(theme.SelBg)).Foreground(theme.C(theme.Fg(theme.Accent))).Bold(true).
			Render(ui.Pad(" "+label, w, lipgloss.NewStyle()))
	}
	hex := theme.Text
	if e.dir {
		hex = theme.Accent
	}
	return ui.Pad(" "+theme.St(theme.Fg(hex)).Render(label), w, lipgloss.NewStyle())
}

// ---- mouse ----

// pickerListIndex returns the index of the row under the cell, or -1.
func (m *Model) pickerListIndex(x, y int) int {
	r := m.pickerRect()
	g := m.pickerGeom()
	row := y - r.y - 2 // the border and the path line
	col := x - r.x - 1
	if row < 0 || row >= g.listRows || col < 0 || col >= g.listW {
		return -1
	}
	if i := m.img.off + row; i < len(m.img.rows()) {
		return i
	}
	return -1
}

// imageClick selects the row under the mouse. A double-click acts as enter. A click outside
// the popup, or while the alt text prompt is open, does nothing.
func (m *Model) imageClick(x, y int, dbl bool) {
	p := m.img
	if p.altOn || !m.pickerRect().has(x, y) {
		return
	}
	i := m.pickerListIndex(x, y)
	if i < 0 {
		return
	}
	p.cur = i
	if dbl {
		m.pickerEnter(false)
	}
	if m.img != nil {
		m.img.keepVisible(m.pickerGeom().listRows)
	}
}

// imageWheel scrolls the list.
func (m *Model) imageWheel(x, y, d int) {
	p := m.img
	if p.altOn || !m.pickerRect().has(x, y) {
		return
	}
	rows := m.pickerGeom().listRows
	p.off = max(0, min(p.off+d, max(0, len(p.rows())-rows)))
}
