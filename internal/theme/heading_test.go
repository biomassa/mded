package theme

import "testing"

// In each theme, a lower heading level has a fainter bar, and the text on each bar is
// readable. Levels 1 to 4 are bold.
func TestHeadingLevelsDiffer(t *testing.T) {
	for _, name := range Names() {
		for _, dark := range []bool{true, false} {
			Apply(name, dark)
			prev := 22.0
			for lv := 1; lv <= 6; lv++ {
				text, bar := HeadingColors(lv)
				if c := Contrast(text, bar); c < 4.5 {
					t.Errorf("%s level %d: text contrast %.1f", name, lv, c)
				}
				// The contrast of the bar to the background goes down from level to level.
				c := Contrast(bar, BaseBg)
				if c > prev {
					t.Errorf("%s level %d: bar contrast %.2f is above level %d (%.2f)", name, lv, c, lv-1, prev)
				}
				prev = c
				if HeadingStyle(lv).GetBold() != (lv <= 4) {
					t.Errorf("%s level %d: bold %v", name, lv, HeadingStyle(lv).GetBold())
				}
			}
		}
	}
	Apply("default", true)
}
