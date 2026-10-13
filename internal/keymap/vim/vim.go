// Package vim is the vim edit keymap. It is a synchronous state machine over a
// buffer.Buffer: Handle takes one key and returns effects. Macros and the dot command replay
// recorded keys through the same path.
//
// The engine keeps its mode, the keys of a command that is not complete, the registers, the
// marks, the macros, the last change for the dot command, and the text of the : and / prompts.
// The buffer keeps the document state.
//
// Marks follow the lines: the buffer reports each edit that adds or removes lines, and the
// engine moves its marks (see shiftMarks). Undo and redo do not move marks. A mark that is out
// of the text is limited to it.
package vim

import (
	"strings"

	"github.com/biomassa/mded/internal/buffer"
	"github.com/biomassa/mded/internal/keymap"
)

// hints is the hint text of all vim modes.
const hints = "esc view · :w save · ctrl+g conventional"

// wantEOL is the wanted column after $. Up and down then stay at the line end.
const wantEOL = 1 << 30

// mode is the mode of the engine.
type mode int

const (
	modeNormal mode = iota
	modeInsert
	modeReplace
	modeVisual
	modeVLine
)

// regval is the content of a register.
type regval struct {
	text     string // lines are joined with "\n", with no final "\n"
	linewise bool
}

// pos is a position in the text.
type pos struct{ row, col int }

// dotCmd is a change that the dot command can repeat.
type dotCmd struct {
	reg   rune
	count int // 0 for no count
	body  []keymap.Key
	vis   *visRep // set for a change that ran on a visual selection
}

// session is an insert or replace session.
type session struct {
	kind  rune // i a I A o O R, or c for a change command
	count int  // the text goes in this many times
	keys  []keymap.Key
	repl  []replEntry // Replace mode: the chars that the typed text replaced
}

// replEntry is one char that Replace mode wrote.
type replEntry struct {
	orig rune
	had  bool // false if the char was appended at the line end
}

// putReq is a put that waits for the system clipboard.
type putReq struct {
	before bool
	count  int
	vis    bool // the put replaces the visual selection
}

// Vim is the vim keymap.
type Vim struct {
	tabWidth int
	mode     mode

	b       *buffer.Buffer // the buffer of the last Handle call
	w, rows int
	fx      []keymap.Effect

	pending []keymap.Key // keys of a command that is not complete

	regs    map[rune]regval
	unnamed regval
	marks   map[rune]pos

	macros    map[rune][]keymap.Key
	rec       []keymap.Key
	recReg    rune // 0 if no macro records
	lastMacro rune
	macroKeys int // keys that macros ran in this Handle call
	failed    bool

	lastFind struct {
		ok  bool
		cmd rune // f F t T
		ch  rune
	}

	dot    *dotCmd
	cur    *dotCmd // the command that runs now
	change bool    // cur changes text
	inDot  int

	ins *session

	prompt   *prompt
	conf     *confirm
	deferred *cmd // an operator that waits for a / or ? search
	target   pos
	haveTgt  bool
	putReq   *putReq

	searchHist []string
	exHist     []string
}

var _ keymap.Keymap = (*Vim)(nil)

// New returns a vim keymap. tab inserts tabWidth spaces and > shifts by tabWidth.
func New(tabWidth int) *Vim {
	return &Vim{
		tabWidth: max(1, tabWidth),
		regs:     map[rune]regval{},
		marks:    map[rune]pos{},
		macros:   map[rune][]keymap.Key{},
	}
}

// Mode returns the mode name: normal, insert, replace, visual, or vline.
func (v *Vim) Mode() string {
	switch v.mode {
	case modeInsert:
		return "insert"
	case modeReplace:
		return "replace"
	case modeVisual:
		return "visual"
	case modeVLine:
		return "vline"
	}
	return "normal"
}

// Normal reports whether the engine is in normal mode with no pending key and no prompt.
// The UI handles its own keys (ctrl+w w, alt+h, alt+l) in this state.
func (v *Vim) Normal() bool {
	return v.mode == modeNormal && len(v.pending) == 0 && v.prompt == nil && v.conf == nil &&
		v.putReq == nil
}

// ModeLabel returns the label for the status line.
func (v *Vim) ModeLabel() string {
	switch v.mode {
	case modeInsert:
		return "INSERT"
	case modeReplace:
		return "REPLACE"
	case modeVisual:
		return "VISUAL"
	case modeVLine:
		return "V-LINE"
	}
	return "NORMAL"
}

// Hints returns the key hints. All vim modes show the same text.
func (v *Vim) Hints() string { return hints }

// visual reports whether a visual mode is on.
func (v *Vim) visual() bool { return v.mode == modeVisual || v.mode == modeVLine }

// status adds a status message to the effects.
func (v *Vim) status(text string, isErr bool) {
	v.fx = append(v.fx, keymap.Status(text, isErr))
}

// fail marks the command as failed. A running macro stops at the next key.
func (v *Vim) fail() { v.failed = true }

// Handle processes one key.
func (v *Vim) Handle(b *buffer.Buffer, k keymap.Key, width, pageRows int) []keymap.Effect {
	v.b, v.w, v.rows = b, max(1, width), max(1, pageRows)
	b.OnShift = v.shiftMarks
	rev := b.Rev
	v.fx = nil
	v.macroKeys = 0
	b.Clamp()
	v.top(k)
	return v.finish(rev)
}

// finish keeps the search matches current and adds the Changed effect.
func (v *Vim) finish(rev int) []keymap.Effect {
	v.b.OnShift = nil
	if v.b.Rev != rev {
		v.b.Refresh()
		v.fx = append(v.fx, keymap.Changed())
	}
	fx := v.fx
	v.fx = nil
	return fx
}

// top handles macro recording, then runs the key.
func (v *Vim) top(k keymap.Key) {
	if v.recReg != 0 && v.Normal() && sym(k) == "q" {
		v.stopRecord()
		return
	}
	if v.recReg != 0 {
		v.rec = append(v.rec, k)
	}
	v.feed(k)
}

// feed runs one key in the state that the engine is in.
func (v *Vim) feed(k keymap.Key) {
	if k.Name == "ctrl+g" {
		v.abort()
		v.fx = append(v.fx, keymap.SwitchFlavor())
		return
	}
	switch {
	case v.conf != nil:
		v.confirmKey(k)
	case v.prompt != nil:
		v.promptKey(k)
	case v.mode == modeInsert || v.mode == modeReplace:
		v.insertKey(k)
	default:
		v.normalKey(k)
	}
	v.fixCursor()
	v.settle()
}

// abort ends an insert session, a prompt, a pending command, and visual mode.
func (v *Vim) abort() {
	if v.b == nil {
		return
	}
	if v.conf != nil {
		v.finishSub()
	}
	if v.prompt != nil {
		v.cancelPrompt()
	}
	v.pending, v.deferred, v.putReq, v.haveTgt = nil, nil, nil, false
	if v.ins != nil {
		v.finishInsert()
	}
	if v.visual() {
		v.exitVisual()
	}
	v.mode = modeNormal
	v.cur, v.change = nil, false
	v.b.Want = -1
	v.fixCursor()
}

// Reset returns to normal mode and drops the pending state. It keeps registers, marks, and
// macros. It ends an open insert session in the buffer of the last Handle call.
func (v *Vim) Reset() {
	if v.b != nil {
		v.b.OnShift = v.shiftMarks
		defer func() { v.b.OnShift = nil }()
	}
	v.abort()
	v.recReg, v.rec = 0, nil
}

// normalKey collects the keys of a command in normal and visual mode and runs it.
func (v *Vim) normalKey(k keymap.Key) {
	if k.Name == "esc" {
		switch {
		case len(v.pending) > 0:
			v.pending = nil
		case v.visual():
			v.exitVisual()
		default:
			v.fx = append(v.fx, keymap.ExitToView())
		}
		return
	}
	v.pending = append(v.pending, k)
	c, st := parse(v.pending, v.visual())
	switch st {
	case stIncomplete:
		return
	case stInvalid:
		v.pending = nil
		v.fail()
		return
	}
	v.pending = nil
	v.runCmd(c, false)
}

// settle makes the finished command the target of the dot command.
func (v *Vim) settle() {
	if v.cur == nil || v.ins != nil || v.prompt != nil || v.conf != nil || v.deferred != nil {
		return
	}
	if v.change && v.inDot == 0 {
		v.dot = v.cur
	}
	v.cur, v.change = nil, false
}

// fixCursor puts the cursor on a char: normal mode cannot rest after the last one.
func (v *Vim) fixCursor() {
	b := v.b
	if b == nil {
		return
	}
	b.Row = max(0, min(b.Row, len(b.Lines)-1))
	n := len(b.Lines[b.Row])
	switch v.mode {
	case modeNormal:
		b.Col = max(0, min(b.Col, n-1))
	case modeVisual, modeVLine:
		b.Col = max(0, min(b.Col, n))
		if b.Col == n && n > 0 && b.Want < wantEOL {
			b.Col = n - 1
		}
	}
}

// Paste inserts text that comes from the terminal or from the system clipboard.
func (v *Vim) Paste(b *buffer.Buffer, text string) []keymap.Effect {
	v.b = b
	b.OnShift = v.shiftMarks
	rev := b.Rev
	v.fx = nil
	b.Clamp()
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	switch {
	case text == "":
	case v.prompt != nil:
		v.promptInsert(strings.ReplaceAll(text, "\n", " "))
	case v.mode == modeInsert || v.mode == modeReplace:
		b.InsertText(text)
	case v.conf != nil:
	default:
		before := false
		count := 1
		if v.putReq != nil {
			before, count = v.putReq.before, v.putReq.count
		}
		repl := v.putReq != nil && v.putReq.vis && v.visual()
		v.putReq = nil
		if v.visual() && !repl {
			v.exitVisual()
		}
		r := regval{text: text}
		if strings.HasSuffix(text, "\n") {
			r = regval{text: strings.TrimSuffix(text, "\n"), linewise: true}
		}
		b.BeginChange()
		if repl { // here before is true for P: the replaced text stays out of the registers
			v.replaceSelection(r, before, count)
			v.change = false
		} else {
			v.put(r, before, count)
		}
		b.EndChange()
		v.fixCursor()
	}
	return v.finish(rev)
}

// StatusLine returns the prompt text: the : and / prompts, the question of :s with the flag
// c, or the recording note. cursorCol is the end of the text for the last two.
func (v *Vim) StatusLine() (string, int, bool) {
	switch {
	case v.conf != nil:
		t := "replace with " + v.conf.job.Rep() + " (y/n/a/q)?"
		return t, len([]rune(t)), true
	case v.prompt != nil:
		return string(v.prompt.kind) + string(v.prompt.text), 1 + v.prompt.pos, true
	case v.recReg != 0:
		t := "recording @" + string(v.recReg)
		return t, len([]rune(t)), true
	}
	return "", 0, false
}

// firstNonBlank returns the index of the first char that is not a space or a tab.
func firstNonBlank(l []rune) int {
	for i, r := range l {
		if r != ' ' && r != '\t' {
			return i
		}
	}
	return max(0, len(l)-1)
}
