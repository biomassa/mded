package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"path/filepath"
	"strings"

	"github.com/biomassa/mded/internal/markdown"
)

// placedImage is an image row that the terminal shows: the image id and the width in cells.
type placedImage struct {
	id   uint32
	cols int
}

// imagePath returns the file of an image reference. A relative path starts at the directory
// of the document. A remote URL gives ok=false. The tilde is not special.
func (m *Model) imagePath(ref string) (string, bool) {
	l := strings.ToLower(ref)
	if ref == "" || strings.HasPrefix(l, "http:") || strings.HasPrefix(l, "https:") || strings.HasPrefix(l, "data:") {
		return "", false
	}
	ref = unescape(ref)
	if filepath.IsAbs(ref) {
		return filepath.Clean(ref), true
	}
	dir := "."
	if m.doc.abs != "" {
		dir = filepath.Dir(m.doc.abs)
	}
	return filepath.Join(dir, ref), true
}

// maxImageRows is the largest height of an image: the setting, or the pane in the viewer.
func (m *Model) maxImageRows() int {
	if m.doc.image {
		return m.textH()
	}
	return m.cfg.ImageMaxRows
}

// imageSize is the LayoutOpts.ImageSize function. It starts the load of an image that is
// not decoded yet, one time per file.
func (m *Model) imageSize(ref markdown.ImageRef) (cols, rows int, ok bool) {
	img := m.opts.Images
	if img == nil {
		return 0, 0, false
	}
	path, ok := m.imagePath(ref.Path)
	if !ok {
		return 0, 0, false
	}
	return m.sizeOf(path)
}

// sizeOf returns the size of an image file in cells. The image loads in the background.
func (m *Model) sizeOf(path string) (cols, rows int, ok bool) {
	img := m.opts.Images
	cols, rows, ok = img.Size(path, m.textW(), m.maxImageRows())
	if !ok && !m.imgTried[path] {
		m.imgTried[path] = true
		m.cmd(img.Load(path))
	}
	return cols, rows, ok
}

// imageDocRows makes the rows of the image viewer: one image that fits the pane. Without
// graphics support, or while the image loads, it is one line of text.
func (m *Model) imageDocRows(tw, th int) []markdown.Row {
	d := m.doc
	name := "🖼\uFE0F " + d.title()
	if m.opts.Images != nil {
		if cols, rows, ok := m.sizeOf(d.abs); ok && cols > 0 {
			ref := &markdown.ImageRef{Path: d.abs}
			out := make([]markdown.Row, rows)
			for k := range out {
				out[k] = markdown.Row{Img: ref, ImgRow: k}
			}
			return out
		}
	}
	return []markdown.Row{{Pre: " " + name}}
}

// placeImages finds the image rows in view and sends their images to the terminal. Place
// gives the transmit command one time per image and size.
func (m *Model) placeImages(tw, th int) {
	m.placed = nil
	img := m.opts.Images
	if img == nil {
		return
	}
	d := m.doc
	for i := d.off; i < d.off+th && i < len(m.rows); i++ {
		r := m.rows[i]
		if r.Img == nil {
			continue
		}
		path, ok := m.imagePath(r.Img.Path)
		if !ok {
			continue
		}
		cols, rows, ok := img.Size(path, tw, m.maxImageRows())
		if !ok {
			continue
		}
		id, cmd := img.Place(path, cols, rows)
		m.cmd(m.afterFirstFrame(cmd))
		if id == 0 {
			continue
		}
		if m.placed == nil {
			m.placed = map[int]placedImage{}
		}
		m.placed[i] = placedImage{id, cols}
	}
}

// frameDelay is the time from the first View call until the first frame is surely on the
// screen. The renderer draws frames on a 60 Hz ticker.
const frameDelay = 100 * time.Millisecond

// afterFirstFrame makes an image transmit command wait until the first frame is on the
// screen. bubbletea writes a raw sequence at once, but it goes to the alternate screen only
// with the first frame. kitty keeps the images of the main screen and of the alternate
// screen apart, so an image that comes before the first frame does not show.
func (m *Model) afterFirstFrame(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	shown := m.shown
	return func() tea.Msg {
		for shown.Load() == 0 {
			time.Sleep(10 * time.Millisecond)
		}
		if d := time.Until(time.Unix(0, shown.Load()).Add(frameDelay)); d > 0 {
			time.Sleep(d)
		}
		return cmd()
	}
}
