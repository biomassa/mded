package theme

import (
	"math"
	"testing"
)

// In each palette theme, the six heading levels have different hues, and the text on each
// bar is readable.
func TestHeadingLevelsDiffer(t *testing.T) {
	for _, name := range Names() {
		if name == "default" {
			continue
		}
		Apply(name, true)
		var bars [6]string
		for lv := 1; lv <= 6; lv++ {
			text, bar := HeadingColors(lv)
			bars[lv-1] = bar
			if c := Contrast(text, bar); c < 4.5 {
				t.Errorf("%s level %d: text contrast %.1f", name, lv, c)
			}
		}
		for i := 0; i < 6; i++ {
			for j := i + 1; j < 6; j++ {
				hi, _, _ := toHSL(headingHexes[i])
				hj, _, _ := toHSL(headingHexes[j])
				d := math.Abs(hi - hj)
				if d = math.Min(d, 360-d); d < minHueGap-1 {
					t.Errorf("%s: levels %d and %d have hues %.0f and %.0f", name, i+1, j+1, hi, hj)
				}
			}
		}
		t.Logf("%-24s %v", name, bars)
	}
	Apply("default", true)
}

func TestHSLRoundTrip(t *testing.T) {
	for _, hex := range []string{"#89B4FA", "#F38BA8", "#000000", "#FFFFFF", "#50FA7B", "#7A637F"} {
		if got := fromHSL(toHSL(hex)); got != hex {
			t.Errorf("%s → %s", hex, got)
		}
	}
}
