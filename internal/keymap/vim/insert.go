package vim

import (
	"strings"

	"github.com/biomassa/mded/internal/keymap"
)

// insertKey handles a key in insert mode and in replace mode.
func (v *Vim) insertKey(k keymap.Key) {
	if k.Name == "ctrl+p" { // the image picker: the session stays open and does not record the key
		v.fx = append(v.fx, keymap.OpenImagePicker())
		return
	}
	if v.cur != nil {
		v.cur.body = append(v.cur.body, k)
	}
	if k.Name == "esc" {
		v.finishInsert()
		return
	}
	v.ins.keys = append(v.ins.keys, k)
	v.insertEdit(k)
}

// insertEdit applies one key of an insert session.
func (v *Vim) insertEdit(k keymap.Key) {
	b := v.b
	replace := v.mode == modeReplace
	switch {
	case k.Name == "enter":
		v.ins.repl = nil
		if replace {
			b.SplitLine()
		} else {
			b.Newline()
		}
	case k.Name == "backspace" || (replace && k.Name == "alt+backspace"):
		if replace {
			v.replaceBack()
		} else {
			b.Backspace()
		}
	case k.Name == "alt+backspace":
		b.DeleteWordBack()
	case k.Name == "delete":
		if !replace {
			b.DeleteForward()
		}
	case k.Name == "tab":
		v.typeText(strings.Repeat(" ", v.tabWidth))
	case k.Name == "left":
		v.ins.repl = nil
		b.Col, b.Want = max(0, b.Col-1), -1
	case k.Name == "right":
		v.ins.repl = nil
		b.Col, b.Want = min(len(b.Lines[b.Row]), b.Col+1), -1
	case k.Name == "home":
		v.ins.repl = nil
		b.Col, b.Want = 0, -1
	case k.Name == "end":
		v.ins.repl = nil
		b.Col, b.Want = len(b.Lines[b.Row]), -1
	case k.Name == "up" || k.Name == "down":
		v.ins.repl = nil
		d := 1
		if k.Name == "up" {
			d = -1
		}
		t := b.Row + d
		if t < 0 || t >= len(b.Lines) {
			return
		}
		w := b.Want
		if w < 0 {
			w = b.Col
		}
		b.Row, b.Col, b.Want = t, min(w, len(b.Lines[t])), w
	case keymap.IsTyped(k):
		v.typeText(k.Text)
	}
}

// typeText puts typed text at the cursor. Replace mode overwrites chars.
func (v *Vim) typeText(text string) {
	b := v.b
	if v.mode != modeReplace {
		b.Insert([]rune(text))
		return
	}
	for _, r := range text {
		l := b.Lines[b.Row]
		if b.Col < len(l) {
			v.ins.repl = append(v.ins.repl, replEntry{orig: l[b.Col], had: true})
			nl := append([]rune(nil), l...)
			nl[b.Col] = r
			b.SetLine(b.Row, nl)
			b.Col++
			continue
		}
		v.ins.repl = append(v.ins.repl, replEntry{})
		b.Insert([]rune{r})
	}
	b.Want = -1
}

// replaceBack runs backspace in replace mode: it restores the char that the last typed char
// replaced. Without such a char, it only moves the cursor left.
func (v *Vim) replaceBack() {
	b := v.b
	s := v.ins
	if len(s.repl) == 0 {
		b.Col = max(0, b.Col-1)
		return
	}
	e := s.repl[len(s.repl)-1]
	s.repl = s.repl[:len(s.repl)-1]
	b.Col--
	l := b.Lines[b.Row]
	nl := append([]rune(nil), l...)
	if e.had {
		nl[b.Col] = e.orig
	} else {
		nl = append(nl[:b.Col], nl[b.Col+1:]...)
	}
	b.SetLine(b.Row, nl)
}

// finishInsert ends an insert session. It repeats the typed text for a count, moves the cursor
// left, and closes the undo step.
func (v *Vim) finishInsert() {
	b := v.b
	s := v.ins
	for i := 1; i < s.count; i++ {
		if s.kind == 'o' || s.kind == 'O' {
			b.End()
			b.SplitLine()
		}
		for _, k := range s.keys {
			v.insertEdit(k)
		}
	}
	v.ins = nil
	v.mode = modeNormal
	if b.Col > 0 {
		b.Col--
	}
	b.Want = -1
	b.EndChange()
}
