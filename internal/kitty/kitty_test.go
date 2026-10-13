package kitty

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestDiacriticTable(t *testing.T) {
	if len(rowColDiacritics) != 297 {
		t.Fatalf("len = %d, want 297", len(rowColDiacritics))
	}
	for i, want := range []rune{0x0305, 0x030D, 0x030E} {
		if rowColDiacritics[i] != want {
			t.Errorf("entry %d = %U, want %U", i, rowColDiacritics[i], want)
		}
	}
}

func TestFitCells(t *testing.T) {
	cases := []struct {
		name               string
		w, h, maxC, maxR   int
		wantCols, wantRows int
	}{
		{"natural", 80, 160, 100, 100, 10, 10},
		{"round up", 81, 17, 100, 100, 11, 2},
		{"fit width", 800, 400, 50, 100, 50, 13},
		{"fit height", 400, 1600, 100, 10, 5, 10},
		{"no upscale", 8, 16, 100, 100, 1, 1},
		{"tiny", 1, 1, 100, 100, 1, 1},
		{"table clamp", 80000, 160, 1000, 1000, 297, 1},
	}
	for _, c := range cases {
		cols, rows := fitCells(c.w, c.h, 8, 16, c.maxC, c.maxR)
		if cols != c.wantCols || rows != c.wantRows {
			t.Errorf("%s: got %dx%d, want %dx%d", c.name, cols, rows, c.wantCols, c.wantRows)
		}
	}
}

func TestChunking(t *testing.T) {
	b64 := strings.Repeat("A", 4096*2+10)
	seq := transmitSeq(7, 3, 2, b64)
	parts := strings.Split(strings.TrimSuffix(seq, "\x1b\\"), "\x1b\\")
	if len(parts) != 3 {
		t.Fatalf("chunks = %d, want 3", len(parts))
	}
	wantHead := []string{
		"\x1b_Ga=T,q=2,f=100,t=d,i=7,U=1,c=3,r=2,m=1;",
		"\x1b_Gm=1;",
		"\x1b_Gm=0;",
	}
	wantLen := []int{4096, 4096, 10}
	for i, p := range parts {
		if !strings.HasPrefix(p, wantHead[i]) {
			t.Errorf("chunk %d head = %q", i, p[:min(len(p), 50)])
		}
		if n := len(p) - len(wantHead[i]); n != wantLen[i] {
			t.Errorf("chunk %d payload = %d, want %d", i, n, wantLen[i])
		}
	}
	one := transmitSeq(1, 1, 1, "AAAA")
	if !strings.Contains(one, "m=0;AAAA") || strings.Count(one, "\x1b_G") != 1 {
		t.Errorf("single chunk = %q", one)
	}
}

func TestRowString(t *testing.T) {
	s := RowString(0x010203, 1, 5)
	if got := strings.Count(s, string(placeholder)); got != 5 {
		t.Errorf("cells = %d, want 5", got)
	}
	if w := lipgloss.Width(s); w != 5 {
		t.Errorf("width = %d, want 5", w)
	}
	if RowString(1, MaxCells, 3) != "" {
		t.Error("row outside table must be empty")
	}
}

func TestSupported(t *testing.T) {
	env := func(kv ...string) func(string) string {
		m := map[string]string{}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return func(k string) string { return m[k] }
	}
	cases := []struct {
		mode string
		env  func(string) string
		want bool
	}{
		{"auto", env("KITTY_WINDOW_ID", "1"), true},
		{"auto", env("TERM", "xterm-kitty"), true},
		{"auto", env("TERM_PROGRAM", "ghostty"), true},
		{"auto", env("TERM", "xterm-ghostty"), true},
		{"auto", env("TERM", "xterm-256color"), false},
		{"auto", env("TERM", "xterm-kitty", "TMUX", "/tmp/x"), false},
		{"on", env(), true},
		{"on", env("TMUX", "/tmp/x"), false},
		{"off", env("TERM", "xterm-kitty"), false},
	}
	for i, c := range cases {
		if got := SupportedEnv(c.mode, c.env); got != c.want {
			t.Errorf("case %d (%s): got %v, want %v", i, c.mode, got, c.want)
		}
	}
}

func TestLoadPlaceDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.png")
	img := image.NewRGBA(image.Rect(0, 0, 80, 32))
	for x := 0; x < 80; x++ {
		img.Set(x, 0, color.White)
	}
	f, _ := os.Create(path)
	png.Encode(f, img)
	f.Close()

	m := NewManager(8, 16)
	if _, _, ok := m.Size(path, 10, 10); ok {
		t.Fatal("size before load must not be ok")
	}
	msg := m.Load(path)()
	if lm := msg.(LoadedMsg); lm.Err != nil {
		t.Fatal(lm.Err)
	}
	cols, rows, ok := m.Size(path, 5, 10)
	if !ok || cols != 5 || rows != 1 {
		t.Fatalf("size = %d %d %v", cols, rows, ok)
	}
	id, cmd := m.Place(path, cols, rows)
	if id == 0 || cmd == nil || cmd() == nil {
		t.Fatal("first place must transmit")
	}
	if id2, cmd2 := m.Place(path, cols, rows); id2 != id || cmd2 != nil {
		t.Fatal("second place must reuse the id")
	}
	m.Forget(path)
	if !strings.Contains(m.DeleteAll(), "a=d,d=I,i=1,q=2") {
		t.Fatal("DeleteAll must list the id")
	}
}
