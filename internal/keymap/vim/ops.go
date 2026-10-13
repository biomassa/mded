package vim

import (
	"strconv"
	"strings"

	"github.com/biomassa/mded/internal/buffer"
	"github.com/biomassa/mded/internal/keymap"
)

// maxMacroKeys limits the keys that macros run in one Handle call. It stops endless macros.
const maxMacroKeys = 100000

// runCmd runs a parsed command. Each command is one undo step. An insert session keeps the
// step open until esc. resume is true if the command ran before and waited for a search.
func (v *Vim) runCmd(c cmd, resume bool) {
	b := v.b
	noGroup := false
	switch c.name {
	case "u", "ctrl+r", "@", ".", "q":
		noGroup = true
	}
	if !noGroup && !resume {
		cnt := 0
		if c.hasCount() {
			cnt = c.count()
		}
		v.cur, v.change = &dotCmd{reg: c.reg, count: cnt, body: c.body}, false
	}
	if noGroup {
		v.exec(c)
		return
	}
	b.BeginChange()
	v.exec(c)
	if v.ins == nil {
		b.EndChange()
	}
}

// exec runs a command.
func (v *Vim) exec(c cmd) {
	b := v.b
	n, has := c.count(), c.hasCount()
	switch {
	case c.obj && c.op == "":
		v.selectObject(c, n)
		return
	case c.op != "" && c.name == "" && !c.dbl && v.visual():
		v.visualOp(c)
		return
	case v.visual() && (c.name == "p" || c.name == "P"):
		v.visualPut(c)
		return
	case v.visual() && c.name == "r":
		v.visualReplace(c)
		return
	case c.op != "":
		v.operator(c)
		return
	}
	switch c.name {
	case "u", "ctrl+r":
		for i := 0; i < n; i++ {
			var ok bool
			if c.name == "u" {
				ok = b.Undo()
			} else {
				ok = b.Redo()
			}
			if !ok {
				break
			}
		}
		v.mode = modeNormal
	case ".":
		v.repeatDot(c)
	case "q":
		v.startRecord(c.arg)
	case "@":
		v.playMacro(c.arg, n)
	case "m":
		if c.arg >= 'a' && c.arg <= 'z' || c.arg >= 'A' && c.arg <= 'Z' {
			v.marks[c.arg] = pos{b.Row, b.Col}
		}
	case "v":
		v.toggleVisual(modeVisual)
	case "V":
		v.toggleVisual(modeVLine)
	case "o", "O":
		if v.visual() {
			b.SelRow, b.Row = b.Row, b.SelRow
			b.SelCol, b.Col = b.Col, b.SelCol
			return
		}
		v.openLine(c.name == "O", n)
	case ":":
		init := ""
		if v.visual() {
			v.exitVisual()
			init = "'<,'>"
		}
		v.openPrompt(':', init, nil)
	case "ctrl+s":
		v.fx = append(v.fx, keymap.Save())
	case "ctrl+p":
		v.fx = append(v.fx, keymap.OpenImagePicker())
	case "x", "X", "s", "S", "D", "C", "Y":
		v.shortcut(c)
	case "J", "gJ":
		v.join(c, n)
	case "~":
		l := b.Lines[b.Row]
		if len(l) == 0 {
			return
		}
		end := min(b.Col+n, len(l))
		b.ChangeCase(b.Row, b.Col, b.Row, end, buffer.CaseToggle)
		b.Col = end
		v.change = true
	case "r":
		v.replaceChars(c, n)
	case "p", "P":
		v.putCmd(c, n)
	case "i", "a", "I", "A", "R":
		l := b.Lines[b.Row]
		switch c.name {
		case "a":
			b.Col = min(b.Col+1, len(l))
			if len(l) == 0 {
				b.Col = 0
			}
		case "A":
			b.Col = len(l)
		case "I":
			b.Col = firstNonBlankInsert(l)
		}
		v.change = true
		v.startInsert(rune(c.name[0]), n)
	default:
		v.exec2(c, n, has)
	}
}

// exec2 runs motions. A motion moves the cursor, and it extends the selection in visual mode.
func (v *Vim) exec2(c cmd, n int, has bool) {
	r := v.motion(&c, n, has, false)
	if r.prompt {
		v.openPrompt(rune(c.name[0]), "", nil)
		return
	}
	v.goTo(r)
}

// firstNonBlankInsert returns the column of I: the first char that is not blank, or the line
// end if the line is blank.
func firstNonBlankInsert(l []rune) int {
	for i, r := range l {
		if r != ' ' && r != '\t' {
			return i
		}
	}
	return len(l)
}

// shortcut runs x X s S D C Y as the operator that each one stands for.
func (v *Vim) shortcut(c cmd) {
	b := v.b
	o := cmd{reg: c.reg, n1: c.n1, has1: c.has1, body: c.body}
	switch c.name {
	case "x":
		o.op, o.name = "d", "l"
	case "X":
		o.op, o.name = "d", "h"
	case "s":
		o.op, o.name = "c", "l"
		if len(b.Lines[b.Row]) == 0 { // nothing to remove
			v.change = true
			v.startInsert('c', 1)
			return
		}
	case "S":
		o.op, o.dbl = "c", true
	case "D":
		o.op, o.name = "d", "$"
	case "C":
		o.op, o.name = "c", "$"
	case "Y":
		o.op, o.dbl = "y", true
	}
	v.operator(o)
}

// join joins lines with J or gJ.
func (v *Vim) join(c cmd, n int) {
	b := v.b
	if v.visual() {
		r1, r2 := min(b.SelRow, b.Row), max(b.SelRow, b.Row)
		rep := v.visInfo(true)
		v.exitVisual()
		b.Row = r1
		if !b.JoinLines(max(2, r2-r1+1), c.name == "J") {
			v.fail()
			return
		}
		v.change = true
		v.setVisRep(rep)
		return
	}
	if !b.JoinLines(max(2, n), c.name == "J") {
		v.fail()
		return
	}
	v.change = true
}

// replaceChars runs r: it replaces n chars with one char.
func (v *Vim) replaceChars(c cmd, n int) {
	b := v.b
	l := b.Lines[b.Row]
	if b.Col+n > len(l) {
		v.fail()
		return
	}
	v.change = true
	if c.arg == '\n' {
		b.DeleteRange(b.Row, b.Col, b.Row, b.Col+n)
		b.SplitLine()
		return
	}
	nl := append([]rune(nil), l...)
	for i := 0; i < n; i++ {
		nl[b.Col+i] = c.arg
	}
	b.SetLine(b.Row, nl)
	b.Col += n - 1
}

// openLine runs o and O: it opens a line and starts an insert session.
func (v *Vim) openLine(above bool, n int) {
	b := v.b
	prefix := b.OpenPrefix(b.Row, above) // a list item continues the list
	if above {
		b.InsertLines(b.Row, "") // the marks of the cursor line go down with it
	} else {
		b.End()
		b.SplitLine()
	}
	b.Col = 0
	b.Insert([]rune(prefix))
	v.change = true
	k := 'o'
	if above {
		k = 'O'
	}
	v.startInsert(k, n)
	if prefix != "" { // a count repeats the prefix with the typed text
		v.ins.keys = append(v.ins.keys, keymap.Key{Name: prefix, Text: prefix})
	}
}

// startInsert opens an insert session. The undo step of the command stays open.
func (v *Vim) startInsert(kind rune, count int) {
	v.mode = modeInsert
	if kind == 'R' {
		v.mode = modeReplace
	}
	v.ins = &session{kind: kind, count: max(1, count)}
	if kind == 'c' {
		v.ins.count = 1
	}
}

// ---- operators ----

// operator runs an operator with a motion, a text object, or a doubled key.
func (v *Vim) operator(c cmd) {
	b := v.b
	n, has := c.count(), c.hasCount()
	var rg region
	switch {
	case c.dbl:
		r2 := min(b.Row+n-1, len(b.Lines)-1)
		if n > 1 && b.Row == len(b.Lines)-1 {
			v.fail()
			return
		}
		rg = region{r1: b.Row, r2: r2, linewise: true}
	case c.obj && c.name[1] == 's':
		rg = v.sentenceRegion(v.sentenceObj(c.name[0] == 'i', n, false, pos{}), c.op)
	case c.obj:
		var ok bool
		if rg, ok = v.object(c.name, n); !ok {
			v.fail()
			return
		}
	default:
		var m mres
		if c.op == "c" && (c.name == "w" || c.name == "W") && v.onWord(c.name == "W") {
			// The change of a word does not take the blanks after it.
			m = v.changeWord(c.name == "W", n)
		} else {
			m = v.motion(&c, n, has, true)
		}
		if m.prompt {
			cc := c
			v.openPrompt(rune(c.name[0]), "", &cc)
			return
		}
		if m.fail {
			v.fail()
			return
		}
		if m.jump {
			v.marks['\''] = pos{b.Row, b.Col}
		}
		rg = v.motionRegion(m)
		if strings.ContainsAny(c.name, "%{}nN`/?") {
			c.big = true // the delete goes to register 1 even if it is small
		}
	}
	v.apply(c, rg, 1)
}

// onWord reports whether the cursor is on a char that is not blank.
func (v *Vim) onWord(big bool) bool {
	b := v.b
	l := b.Lines[b.Row]
	return b.Col < len(l) && cls(l[b.Col], big) != 0
}

// changeWord finds the range of cw: up to the end of the n-th word, from the cursor.
func (v *Vim) changeWord(big bool, n int) mres {
	b := v.b
	l := b.Lines[b.Row]
	r := mres{row: b.Row, incl: true}
	c := cls(l[b.Col], big)
	e := b.Col
	for e+1 < len(l) && cls(l[e+1], big) == c {
		e++
	}
	p := pos{b.Row, e}
	for i := 1; i < n; i++ {
		np, ok := endWord(b, p, big)
		if !ok {
			break
		}
		p = np
	}
	r.row, r.col = p.row, p.col
	return r
}

// motionRegion turns a motion into the region that an operator works on.
func (v *Vim) motionRegion(m mres) region {
	b := v.b
	cur := pos{b.Row, b.Col}
	tgt := pos{m.row, m.col}
	if m.linewise {
		return region{r1: min(cur.row, tgt.row), r2: max(cur.row, tgt.row), linewise: true}
	}
	s, e := cur, tgt
	if e.row < s.row || (e.row == s.row && e.col < s.col) {
		s, e = e, s
	}
	if m.incl {
		e.col = min(e.col+1, len(b.Lines[e.row]))
		return region{r1: s.row, c1: s.col, r2: e.row, c2: e.col}
	}
	if e.col == 0 && e.row > s.row {
		// An exclusive motion that ends in column 0 stops at the end of the line before.
		e.row--
		e.col = len(b.Lines[e.row])
		blanks := strings.TrimSpace(string(b.Lines[s.row][:min(s.col, len(b.Lines[s.row]))])) == ""
		if blanks {
			return region{r1: s.row, r2: e.row, linewise: true}
		}
	}
	return region{r1: s.row, c1: s.col, r2: e.row, c2: e.col}
}

// apply runs operator c.op on a region. shift is the number of indents for > and <.
func (v *Vim) apply(c cmd, rg region, shift int) {
	b := v.b
	text := func() string {
		if rg.linewise {
			return b.YankLines(rg.r1, rg.r2)
		}
		return b.TextRange(rg.r1, rg.c1, rg.r2, rg.c2)
	}
	empty := !rg.linewise && rg.r1 == rg.r2 && rg.c1 == rg.c2
	switch c.op {
	case "y":
		if empty {
			return
		}
		v.writeReg(c.reg, regval{text(), rg.linewise}, false, false)
		if rg.linewise {
			b.Row = rg.r1
		} else {
			b.Row, b.Col = rg.r1, rg.c1
		}
	case "d", "c":
		if empty && c.op == "d" {
			return
		}
		v.change = true
		t := text()
		small := !rg.linewise && rg.r1 == rg.r2 && !c.big
		switch {
		case c.op == "c" && rg.linewise:
			b.SetLine(rg.r1, []rune{})
			if rg.r2 > rg.r1 {
				b.DeleteLines(rg.r1+1, rg.r2)
			}
			b.Row, b.Col, b.Want = rg.r1, 0, -1
		case rg.linewise:
			b.DeleteLines(rg.r1, rg.r2)
			b.Col = firstNonBlank(b.Lines[b.Row])
		default:
			b.DeleteRange(rg.r1, rg.c1, rg.r2, rg.c2)
		}
		if !empty || rg.linewise {
			v.writeReg(c.reg, regval{t, rg.linewise}, true, small)
		}
		if c.op == "c" {
			v.startInsert('c', 1)
		}
	case ">", "<":
		v.change = true
		if c.op == ">" {
			b.Indent(rg.r1, rg.r2, v.tabWidth*shift)
		} else {
			b.Outdent(rg.r1, rg.r2, v.tabWidth*shift)
		}
		b.Row = rg.r1
		b.Col, b.Want = firstNonBlank(b.Lines[b.Row]), -1
	case "gu", "gU", "g~":
		v.change = true
		mode := map[string]buffer.CaseMode{"gu": buffer.CaseLower, "gU": buffer.CaseUpper,
			"g~": buffer.CaseToggle}[c.op]
		if rg.linewise {
			b.ChangeCaseLines(rg.r1, rg.r2, mode)
			b.Row = rg.r1
		} else {
			b.ChangeCase(rg.r1, rg.c1, rg.r2, rg.c2, mode)
			b.Row, b.Col = rg.r1, rg.c1
		}
	}
	b.Want = -1
}

// ---- visual mode ----

// toggleVisual starts visual mode m, or it leaves visual mode, or it switches between the two.
func (v *Vim) toggleVisual(m mode) {
	b := v.b
	if v.mode == m {
		v.exitVisual()
		return
	}
	if !v.visual() {
		b.SelRow, b.SelCol, b.SelOn = b.Row, b.Col, true
	}
	v.mode = m
	b.SelLinewise, b.SelInclusive = m == modeVLine, m == modeVisual
}

// StartVisual starts charwise visual mode with the anchor at row, col. The cursor stays where
// it is. The UI calls it when the mouse drags. It does nothing in insert and replace mode.
// A second call moves the anchor, so the UI can call it for each motion of the mouse.
func (v *Vim) StartVisual(b *buffer.Buffer, row, col int) {
	if v.mode == modeInsert || v.mode == modeReplace {
		return
	}
	v.b = b
	v.pending = nil
	row = max(0, min(row, len(b.Lines)-1))
	col = max(0, min(col, len(b.Lines[row])-1))
	b.SelRow, b.SelCol, b.SelOn = row, col, true
	b.SelLinewise, b.SelInclusive = false, true
	v.mode = modeVisual
	v.fixCursor()
}

// LeaveVisual leaves visual mode, if it is on. The UI calls it when the mouse clicks.
func (v *Vim) LeaveVisual(b *buffer.Buffer) {
	if !v.visual() {
		return
	}
	v.b = b
	v.exitVisual()
}

// exitVisual leaves visual mode and sets the marks < and >.
func (v *Vim) exitVisual() {
	b := v.b
	a, c := pos{b.SelRow, b.SelCol}, pos{b.Row, b.Col}
	if c.row < a.row || (c.row == a.row && c.col < a.col) {
		a, c = c, a
	}
	v.marks['<'], v.marks['>'] = a, c
	b.ClearSel()
	v.mode = modeNormal
}

// selectObject selects a text object in visual mode.
func (v *Vim) selectObject(c cmd, n int) {
	b := v.b
	if !v.visual() {
		v.fail()
		return
	}
	if c.name[1] == 's' {
		v.selectSentence(c.name[0] == 'i', n)
		return
	}
	rg, ok := v.object(c.name, n)
	if !ok {
		v.fail()
		return
	}
	if rg.linewise {
		b.SelRow, b.Row = rg.r1, rg.r2
		b.SelCol = 0
		v.mode = modeVLine
		b.SelLinewise, b.SelInclusive = true, false
		return
	}
	if rg.r1 == rg.r2 && rg.c1 == rg.c2 {
		return
	}
	end := pos{rg.r2, rg.c2 - 1}
	if rg.c2 == 0 {
		end = pos{rg.r2 - 1, len(b.Lines[rg.r2-1])}
	}
	b.SelRow, b.SelCol = rg.r1, rg.c1
	b.Row, b.Col, b.Want = end.row, end.col, -1
	v.mode = modeVisual
	b.SelLinewise, b.SelInclusive = false, true
}

// visualOp runs an operator on the selection.
func (v *Vim) visualOp(c cmd) {
	b := v.b
	linewise := b.SelLinewise || c.vline
	a, e := v.selEnds()
	rep := v.visInfo(linewise)
	r1, c1, r2, c2, ok := b.SelRange()
	v.exitVisual()
	if !ok && !linewise {
		return
	}
	rg := region{r1: r1, c1: c1, r2: r2, c2: c2}
	if linewise {
		rg = region{r1: a.row, r2: e.row, linewise: true}
	}
	b.Row, b.Col = a.row, a.col
	v.apply(c, rg, c.count())
	if v.change {
		v.setVisRep(rep)
	}
}

// ---- registers ----

// writeReg stores text that a yank or a delete made. del is true for a delete, small for a
// delete that fits in one line. Every write also goes to the system clipboard.
func (v *Vim) writeReg(name rune, r regval, del, small bool) {
	if name == '_' {
		return
	}
	clip := r.text
	if r.linewise {
		clip += "\n"
	}
	v.fx = append(v.fx, keymap.SetClipboard(clip))
	switch {
	case name >= 'a' && name <= 'z':
		v.regs[name] = r
		delete(v.macros, name)
		v.unnamed = r
	case name >= 'A' && name <= 'Z':
		low := name - 'A' + 'a'
		if old, ok := v.regs[low]; ok {
			r = appendReg(old, r)
		}
		v.regs[low] = r
		delete(v.macros, low)
		v.unnamed = r
	case name == '+' || name == '*':
		v.unnamed = r
	case !del:
		v.regs['0'] = r
		v.unnamed = r
	case small:
		v.regs['-'] = r
		v.unnamed = r
	default:
		for i := '9'; i > '1'; i-- {
			if old, ok := v.regs[i-1]; ok {
				v.regs[i] = old
			}
		}
		v.regs['1'] = r
		v.unnamed = r
	}
}

// appendReg adds r to the end of old. The result is linewise if one of them is.
func appendReg(old, r regval) regval {
	if old.linewise || r.linewise {
		return regval{old.text + "\n" + r.text, true}
	}
	return regval{old.text + r.text, false}
}

// readReg returns the content of a register.
func (v *Vim) readReg(name rune) (regval, bool) {
	var r regval
	switch {
	case name == 0 || name == '"':
		r = v.unnamed
	case name >= 'a' && name <= 'z' || name >= '0' && name <= '9' || name == '-':
		r = v.regs[name]
	case name >= 'A' && name <= 'Z':
		r = v.regs[name-'A'+'a']
	}
	return r, r.text != "" || r.linewise
}

// putCmd runs p and P.
func (v *Vim) putCmd(c cmd, n int) {
	if c.reg == '+' || c.reg == '*' {
		v.putReq = &putReq{before: c.name == "P", count: n}
		v.fx = append(v.fx, keymap.ReadClipboard())
		return
	}
	r, ok := v.readReg(c.reg)
	if !ok {
		v.fail()
		return
	}
	v.change = true
	v.put(r, c.name == "P", n)
}

// put puts a register in the text n times.
func (v *Vim) put(r regval, before bool, n int) {
	b := v.b
	b.Want = -1
	if r.linewise {
		text := strings.TrimSuffix(strings.Repeat(r.text+"\n", n), "\n")
		at := b.Row + 1
		if before {
			at = b.Row
		}
		b.InsertLines(at, text)
		b.Col = firstNonBlank(b.Lines[b.Row])
		return
	}
	if r.text == "" {
		return
	}
	text := strings.Repeat(r.text, n)
	col := b.Col
	if l := b.Lines[b.Row]; !before && len(l) > 0 {
		col = min(col+1, len(l))
	}
	row := b.Row
	b.Col = col
	b.InsertText(text)
	if strings.Contains(text, "\n") {
		b.Row, b.Col = row, col
		return
	}
	b.Col--
}

// ---- undo, dot, macros ----

// repeatDot runs the last change again. A count replaces the count of the change.
func (v *Vim) repeatDot(c cmd) {
	d := v.dot
	if d == nil {
		return
	}
	if c.hasCount() {
		d.count = c.count()
	}
	var keys []keymap.Key
	if d.reg != 0 {
		keys = append(keys, key(`"`), key(string(d.reg)))
	}
	if d.count > 0 {
		for _, r := range strconv.Itoa(d.count) {
			keys = append(keys, key(string(r)))
		}
	}
	keys = append(keys, d.body...)
	if d.vis != nil {
		v.startVisRep(d.vis)
	}
	v.inDot++
	for _, k := range keys {
		v.feed(k)
	}
	v.inDot--
	if d.vis != nil && v.visual() {
		v.exitVisual()
	}
}

// key makes a typed key.
func key(s string) keymap.Key { return keymap.Key{Name: s, Text: s} }

// startRecord starts to record a macro in register r. An upper case name appends.
func (v *Vim) startRecord(r rune) {
	if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
		v.recReg, v.rec = r, nil
	}
}

// stopRecord ends the macro that records and stores its keys.
func (v *Vim) stopRecord() {
	r := v.recReg
	v.recReg = 0
	low := r
	if r >= 'A' && r <= 'Z' {
		low = r - 'A' + 'a'
		v.macros[low] = append(append([]keymap.Key(nil), v.macros[low]...), v.rec...)
	} else {
		v.macros[low] = v.rec
	}
	var sb strings.Builder
	for _, k := range v.macros[low] {
		switch s := sym(k); {
		case s == "esc":
			sb.WriteByte(0x1b)
		case s == "enter":
			sb.WriteByte('\r')
		case keymap.IsTyped(k):
			sb.WriteString(s)
		}
	}
	v.regs[low] = regval{text: sb.String()}
	v.rec = nil
}

// playMacro runs the macro in register r n times. A failed command stops it.
func (v *Vim) playMacro(r rune, n int) {
	if r == '@' {
		r = v.lastMacro
	}
	if r == 0 {
		v.fail()
		return
	}
	if r >= 'A' && r <= 'Z' {
		r = r - 'A' + 'a'
	}
	keys, ok := v.macros[r]
	if !ok {
		reg, has := v.regs[r]
		if !has {
			v.fail()
			return
		}
		for _, c := range reg.text {
			switch c {
			case 0x1b:
				keys = append(keys, keymap.Key{Name: "esc"})
			case '\r':
				keys = append(keys, keymap.Key{Name: "enter"})
			default:
				keys = append(keys, key(string(c)))
			}
		}
	}
	v.lastMacro = r
	v.failed = false
	for i := 0; i < n && !v.failed; i++ {
		for _, k := range keys {
			if v.failed || v.macroKeys >= maxMacroKeys {
				v.failed = true
				return
			}
			v.macroKeys++
			v.feed(k)
		}
	}
}

// selectSentence selects is or as in visual mode. A selection of more than one char gets
// longer, as in vim. Otherwise the selection is the object.
func (v *Vim) selectSentence(inner bool, n int) {
	b := v.b
	anchor := pos{b.SelRow, b.SelCol}
	r := v.sentenceObj(inner, n, true, anchor)
	if !r.extend {
		b.SelRow, b.SelCol = r.start.row, r.start.col
	}
	b.Row, b.Col, b.Want = r.end.row, r.end.col, -1
	v.mode = modeVisual
	b.SelLinewise, b.SelInclusive = false, true
}
