package theme

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// minHeadingStep is the smallest perceived difference (CIE76 ΔE) between the bars of two
// adjacent heading levels, and between the level 4 bar and the background. A difference of
// about 2.3 is just visible. 5 is clear at a glance.
const minHeadingStep = 4.9

// In each theme, dark and light, the heading levels are clearly different: the bars of
// levels 1 to 4 have a clear step between them and to the background, the bar length goes
// down from level to level, and all heading text is readable.
func TestHeadingHierarchy(t *testing.T) {
	for _, name := range Names() {
		for _, dark := range []bool{true, false} {
			Apply(name, dark)
			var bars []string
			for lv := 1; lv <= 6; lv++ {
				text, bar := HeadingColors(lv)
				if c := Contrast(text, bar); c < 4.5 {
					t.Errorf("%s dark=%v level %d: text contrast %.1f", name, dark, lv, c)
				}
				if lv <= headingBarLevels {
					bars = append(bars, bar)
				}
				if bold := HeadingStyle(lv).GetBold(); bold != (lv <= 5) {
					t.Errorf("%s level %d: bold %v", name, lv, bold)
				}
			}
			bars = append(bars, BaseBg)
			for i := 0; i+1 < len(bars); i++ {
				if d := deltaE(bars[i], bars[i+1]); d < minHeadingStep {
					t.Errorf("%s dark=%v: level %d and %s differ by ΔE %.1f", name, dark, i+1, next(i), d)
				}
			}
			// Levels 5 and 6 differ by the weight of the text.
			if HeadingStyle(5).GetBold() == HeadingStyle(6).GetBold() {
				t.Errorf("%s: levels 5 and 6 look the same", name)
			}
		}
	}
	Apply("default", true)
}

// next names the item after bar i in the test above.
func next(i int) string {
	if i+1 == headingBarLevels {
		return "the background"
	}
	return "level " + string(rune('1'+i+1))
}

// The bar length shows the level: the full width for levels 1 and 2, a short bar for
// levels 3 and 4, and no bar for levels 5 and 6.
func TestHeadingBarLength(t *testing.T) {
	Apply("default", true)
	const w = 40
	widths := make([]int, 7)
	for lv := 1; lv <= 6; lv++ {
		widths[lv] = ansi.StringWidth(HeadingBar(lv, "Title", w))
	}
	if widths[1] != w-1 || widths[2] != w-1 {
		t.Errorf("levels 1 and 2: widths %d %d, want %d", widths[1], widths[2], w-1)
	}
	if widths[3] != len(" Title")+2 || widths[4] != widths[3] {
		t.Errorf("levels 3 and 4: widths %d %d", widths[3], widths[4])
	}
	if widths[5] != len(" Title") || strings.Contains(HeadingBar(5, "Title", w), "48;") {
		t.Errorf("level 5 must have no bar: %q", HeadingBar(5, "Title", w))
	}
}

func TestDeltaE(t *testing.T) {
	if d := deltaE("#000000", "#FFFFFF"); d < 99 || d > 101 {
		t.Errorf("black to white: %.1f", d)
	}
	if d := deltaE("#336699", "#336699"); d != 0 {
		t.Errorf("same color: %.1f", d)
	}
}
