// Package kitty shows images in the terminal with the kitty graphics protocol.
// It uses Unicode placeholders, so the images scroll and clip like normal text.
package kitty

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"  // Register the GIF decoder.
	_ "image/jpeg" // Register the JPEG decoder.
	"image/png"
	"os"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // Register the WebP decoder.
	"golang.org/x/sys/unix"
)

const (
	// placeholder is the Unicode placeholder character of the protocol.
	placeholder = '\U0010EEEE'
	// chunkSize is the largest base64 payload of one escape sequence.
	chunkSize = 4096
	// maxID is the largest 24-bit image id.
	maxID = 0xFFFFFF
	// Cell pixel size when the terminal does not report it.
	defaultCellW = 8
	defaultCellH = 16
)

// MaxCells is the largest number of rows or columns of one image.
// It is the size of the diacritics table.
const MaxCells = len(rowColDiacritics)

// Supported tells if images are on for the mode "auto", "on" or "off".
// It reads the process environment.
func Supported(mode string) bool {
	return SupportedEnv(mode, os.Getenv)
}

// SupportedEnv is Supported with an injected environment lookup.
// Inside tmux the result is always false (v0.1).
func SupportedEnv(mode string, getenv func(string) string) bool {
	if mode == "off" || getenv("TMUX") != "" {
		return false
	}
	if mode == "on" {
		return true
	}
	if mode != "auto" {
		return false
	}
	term := getenv("TERM")
	return getenv("KITTY_WINDOW_ID") != "" || term == "xterm-kitty" ||
		term == "xterm-ghostty" || getenv("TERM_PROGRAM") == "ghostty"
}

// CellSize returns the pixel size of one terminal cell.
// It falls back to 8x16 when the terminal does not report it.
func CellSize() (w, h int) {
	for _, f := range []*os.File{os.Stdout, os.Stdin, os.Stderr} {
		ws, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
		if err != nil || ws.Col == 0 || ws.Row == 0 || ws.Xpixel == 0 || ws.Ypixel == 0 {
			continue
		}
		return int(ws.Xpixel) / int(ws.Col), int(ws.Ypixel) / int(ws.Row)
	}
	return defaultCellW, defaultCellH
}

// LoadedMsg tells that Load finished for a path.
type LoadedMsg struct {
	Path string
	Err  error
}

// entry is one decoded image.
type entry struct {
	img   image.Image
	mtime time.Time
}

// placeKey identifies one transmitted image.
type placeKey struct {
	path       string
	mtime      time.Time
	cols, rows int
}

// Manager decodes, caches and transmits images.
// It is safe for use from many goroutines.
type Manager struct {
	mu           sync.Mutex
	cellW, cellH int
	images       map[string]entry
	loading      map[string]bool
	placed       map[placeKey]uint32
	sent         []uint32 // Every id given out, for DeleteAll.
	nextID       uint32
}

// NewManager makes a Manager for the given cell pixel size.
// A size of zero or less becomes the default 8x16.
func NewManager(cellW, cellH int) *Manager {
	if cellW <= 0 {
		cellW = defaultCellW
	}
	if cellH <= 0 {
		cellH = defaultCellH
	}
	return &Manager{
		cellW:   cellW,
		cellH:   cellH,
		images:  map[string]entry{},
		loading: map[string]bool{},
		placed:  map[placeKey]uint32{},
	}
}

// Size returns the size in cells of a decoded image, fitted to maxCols and maxRows.
// It keeps the aspect ratio and never scales up.
// It returns ok=false when the image is not decoded yet.
func (m *Manager) Size(path string, maxCols, maxRows int) (cols, rows int, ok bool) {
	m.mu.Lock()
	e, found := m.images[path]
	m.mu.Unlock()
	if !found {
		return 0, 0, false
	}
	b := e.img.Bounds()
	cols, rows = fitCells(b.Dx(), b.Dy(), m.cellW, m.cellH, maxCols, maxRows)
	return cols, rows, true
}

// fitCells computes the cell size of a pixel size.
func fitCells(pxW, pxH, cellW, cellH, maxCols, maxRows int) (cols, rows int) {
	maxCols = max(1, min(maxCols, MaxCells))
	maxRows = max(1, min(maxRows, MaxCells))
	nc := float64(pxW) / float64(cellW)
	nr := float64(pxH) / float64(cellH)
	f := 1.0
	if nc > 0 {
		f = min(f, float64(maxCols)/nc)
	}
	if nr > 0 {
		f = min(f, float64(maxRows)/nr)
	}
	if f < 1 {
		cols = int(ceil(nc * f))
		rows = int(ceil(nr * f))
	} else {
		cols = int(ceil(nc))
		rows = int(ceil(nr))
	}
	return max(1, min(cols, maxCols)), max(1, min(rows, maxRows))
}

// ceil rounds up, with a small tolerance for float error.
func ceil(v float64) float64 {
	i := float64(int(v))
	if v-i < 1e-9 {
		return i
	}
	return i + 1
}

// Load decodes the image file off the UI goroutine.
// It caches the image with its mtime. A changed mtime reloads the file.
// It returns nil when a load of the same path is already running.
func (m *Manager) Load(path string) tea.Cmd {
	m.mu.Lock()
	if m.loading[path] {
		m.mu.Unlock()
		return nil
	}
	m.loading[path] = true
	m.mu.Unlock()
	return func() tea.Msg {
		err := m.load(path)
		m.mu.Lock()
		delete(m.loading, path)
		m.mu.Unlock()
		return LoadedMsg{Path: path, Err: err}
	}
}

// load reads and decodes one file into the cache.
func (m *Manager) load(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	m.mu.Lock()
	e, found := m.images[path]
	m.mu.Unlock()
	if found && e.mtime.Equal(st.ModTime()) {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.images[path] = entry{img: img, mtime: st.ModTime()}
	m.mu.Unlock()
	return nil
}

// Forget drops the cache entries of a path, for example when the file changed.
// Images already sent to the terminal stay until DeleteAll.
func (m *Manager) Forget(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.images, path)
	for k := range m.placed {
		if k.path == path {
			delete(m.placed, k)
		}
	}
}

// Place returns the image id for a path at a cell size.
// When the image is not sent at this size yet, it also returns a command that
// scales, encodes and transmits it. The command is nil otherwise.
// It returns id 0 when the image is not decoded yet.
func (m *Manager) Place(path string, cols, rows int) (id uint32, cmd tea.Cmd) {
	cols = max(1, min(cols, MaxCells))
	rows = max(1, min(rows, MaxCells))
	m.mu.Lock()
	defer m.mu.Unlock()
	e, found := m.images[path]
	if !found {
		return 0, nil
	}
	key := placeKey{path, e.mtime, cols, rows}
	if id, ok := m.placed[key]; ok {
		return id, nil
	}
	m.nextID++
	if m.nextID > maxID {
		m.nextID = 1
	}
	id = m.nextID
	m.placed[key] = id
	m.sent = append(m.sent, id)
	w, h := cols*m.cellW, rows*m.cellH
	return id, func() tea.Msg {
		data, err := scalePNG(e.img, w, h)
		if err != nil {
			return nil
		}
		return tea.Raw(transmitSeq(id, cols, rows, base64.StdEncoding.EncodeToString(data)))()
	}
}

// scalePNG scales an image to w x h pixels and encodes it as PNG.
func scalePNG(src image.Image, w, h int) ([]byte, error) {
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	if b := src.Bounds(); b.Dx() == w && b.Dy() == h {
		draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	} else {
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// transmitSeq builds the chunked transmit sequence for base64 data.
// Only the first chunk carries the keys.
func transmitSeq(id uint32, cols, rows int, b64 string) string {
	var sb strings.Builder
	for first := true; ; first = false {
		chunk := b64
		if len(chunk) > chunkSize {
			chunk = chunk[:chunkSize]
		}
		b64 = b64[len(chunk):]
		more := 0
		if len(b64) > 0 {
			more = 1
		}
		if first {
			fmt.Fprintf(&sb, "\x1b_Ga=T,q=2,f=100,t=d,i=%d,U=1,c=%d,r=%d,m=%d;%s\x1b\\",
				id, cols, rows, more, chunk)
		} else {
			fmt.Fprintf(&sb, "\x1b_Gm=%d;%s\x1b\\", more, chunk)
		}
		if more == 0 {
			return sb.String()
		}
	}
}

// RowString returns one image row as placeholder cells.
// The foreground color carries the image id.
// It returns an empty string when row is outside the diacritics table.
func (m *Manager) RowString(id uint32, row, cols int) string {
	return RowString(id, row, cols)
}

// RowString is the Manager method without a Manager.
func RowString(id uint32, row, cols int) string {
	if row < 0 || row >= MaxCells || cols <= 0 {
		return ""
	}
	cols = min(cols, MaxCells)
	var sb strings.Builder
	for c := 0; c < cols; c++ {
		sb.WriteRune(placeholder)
		sb.WriteRune(rowColDiacritics[row])
		sb.WriteRune(rowColDiacritics[c])
	}
	color := lipgloss.Color(fmt.Sprintf("#%06X", id&maxID))
	return lipgloss.NewStyle().Foreground(color).Render(sb.String())
}

// DeleteAll returns the sequences that delete every transmitted image.
// The UI writes it on exit.
func (m *Manager) DeleteAll() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := map[uint32]bool{}
	var sb strings.Builder
	for _, id := range m.sent {
		if !seen[id] {
			seen[id] = true
			fmt.Fprintf(&sb, "\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", id)
		}
	}
	return sb.String()
}
