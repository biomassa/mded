package theme

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestNamesAndApply(t *testing.T) {
	names := Names()
	if names[0] != "default" || len(names) != 20 {
		t.Fatalf("names = %v", names)
	}
	Apply("nord", true)
	if Current() != "nord" || Bg() != "#2e3440" || !Dark() {
		t.Errorf("nord: %s %s %v", Current(), Bg(), Dark())
	}
	Apply("gruvbox-light", true)
	if Dark() {
		t.Error("gruvbox-light must be light")
	}
	Apply("nope", false)
	if Current() != "default" || Bg() != "" || Dark() || BaseBg != "#FFFFFF" {
		t.Errorf("default light: %s %q %v %s", Current(), Bg(), Dark(), BaseBg)
	}
	Apply("default", true)
	if !Dark() || Text != "#E6E6E6" {
		t.Error("default dark")
	}
	for _, n := range names {
		Apply(n, true)
		if _, bar := HeadingColors(1); Contrast(Text, BaseBg) < 3 || bar == "" {
			t.Errorf("%s: low contrast", n)
		}
		_ = ReaderStyle()
	}
	Apply("default", true)
}

func TestHelpers(t *testing.T) {
	if Mix("#000000", "#FFFFFF", 0.5) != "#808080" {
		t.Error("Mix")
	}
	if Contrast("#000000", "#FFFFFF") < 20.9 {
		t.Error("Contrast")
	}
	if Contrast(Readable("#333333", "#222222", 4.5), "#222222") < 4.5 {
		t.Error("Readable")
	}
	if Label("default") != "default (terminal colors)" || Label("nord") != "nord" {
		t.Error("Label")
	}
	if got := ansi.Strip(HeadingBar(1, "Hi", 10)); got != " Hi      " || strings.Count(got, "") != 10 {
		t.Errorf("bar = %q", got)
	}
}
