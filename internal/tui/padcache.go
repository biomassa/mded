package tui

import (
	"github.com/biomassa/mded/internal/ui"

	"charm.land/lipgloss/v2"
)

// padCache keeps the padded lines of the last two frames. A scroll shows most lines of the
// frame before, and a map lookup is much faster than a width measure. The cache holds at
// most the lines of two frames.
type padCache struct {
	w         int
	cur, prev map[string]string
}

// pad returns s padded or cut to w cells, as ui.Pad with a plain fill.
func (c *padCache) pad(s string, w int) string {
	if w != c.w || c.cur == nil {
		c.w, c.cur, c.prev = w, map[string]string{}, nil
	}
	if p, ok := c.cur[s]; ok {
		return p
	}
	p, ok := c.prev[s]
	if !ok {
		p = ui.Pad(s, w, lipgloss.NewStyle())
	}
	c.cur[s] = p
	return p
}

// frame starts a new frame: the lines of the frame before stay for one more frame.
func (c *padCache) frame() {
	c.prev, c.cur = c.cur, make(map[string]string, len(c.cur))
}
