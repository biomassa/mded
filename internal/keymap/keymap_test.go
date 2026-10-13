package keymap

import "testing"

func TestIsTyped(t *testing.T) {
	tests := []struct {
		k    Key
		want bool
	}{
		{Key{"a", "a"}, true},
		{Key{"A", "A"}, true},
		{Key{"space", " "}, true},
		{Key{"shift+a", "A"}, true},
		{Key{"ż", "ż"}, true},
		{Key{"left", ""}, false},
		{Key{"ctrl+a", ""}, false},
		{Key{"ctrl+a", "a"}, false},
		{Key{"alt+x", "x"}, false},
	}
	for _, tc := range tests {
		if got := IsTyped(tc.k); got != tc.want {
			t.Errorf("IsTyped(%+v) = %v, want %v", tc.k, got, tc.want)
		}
	}
}

func TestEffects(t *testing.T) {
	e := []Effect{Status("x", true), SaveAs("f"), EditFile("g"), SetClipboard("c")}
	if !Has(e, KindSaveAs) || Has(e, KindSave) {
		t.Error("Has")
	}
	if e[0].Text != "x" || !e[0].Err || e[1].Path != "f" || e[2].Path != "g" || e[3].Text != "c" {
		t.Errorf("fields: %+v", e)
	}
}
