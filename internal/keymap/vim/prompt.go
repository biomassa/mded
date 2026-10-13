package vim

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/biomassa/mded/internal/buffer"
	"github.com/biomassa/mded/internal/keymap"
)

// prompt is the : prompt or the / and ? prompts on the status line.
type prompt struct {
	kind rune // ':' '/' '?'
	text []rune
	pos  int // cursor, in runes

	hist  int    // index in the history, -1 while the user types a new text
	draft []rune // the text that the user typed before the history walk

	// The state to restore if the user cancels a search.
	orig      pos
	prevPat   string
	prevBack  bool
	prevHL    bool
	searchSet bool
}

// confirm is the state of :s with the flag c.
type confirm struct {
	job *buffer.SubJob
	pat string
	any bool // the job found a match
}

// openPrompt opens the prompt of kind. An operator waits in deferred for a search.
func (v *Vim) openPrompt(kind rune, initial string, deferred *cmd) {
	b := v.b
	p := &prompt{kind: kind, text: []rune(initial), hist: -1}
	p.pos = len(p.text)
	if kind != ':' {
		p.orig = pos{b.Row, b.Col}
		p.prevPat, p.prevBack, p.prevHL = b.Search.Pattern, b.Search.Backward, b.Search.Highlight
	}
	v.prompt, v.deferred = p, deferred
}

// history returns the history list of the prompt.
func (v *Vim) history(p *prompt) *[]string {
	if p.kind == ':' {
		return &v.exHist
	}
	return &v.searchHist
}

// promptKey edits the text of the prompt.
func (v *Vim) promptKey(k keymap.Key) {
	p := v.prompt
	if v.cur != nil {
		v.cur.body = append(v.cur.body, k)
	}
	before := string(p.text)
	switch {
	case k.Name == "esc":
		v.cancelPrompt()
		return
	case k.Name == "enter":
		v.submitPrompt()
		return
	case k.Name == "backspace":
		if len(p.text) == 0 {
			v.cancelPrompt()
			return
		}
		if p.pos > 0 {
			p.text = append(p.text[:p.pos-1], p.text[p.pos:]...)
			p.pos--
		}
	case k.Name == "alt+backspace":
		i := p.pos
		for i > 0 && !buffer.IsWordRune(p.text[i-1]) {
			i--
		}
		for i > 0 && buffer.IsWordRune(p.text[i-1]) {
			i--
		}
		p.text = append(p.text[:i], p.text[p.pos:]...)
		p.pos = i
	case k.Name == "delete":
		if p.pos < len(p.text) {
			p.text = append(p.text[:p.pos], p.text[p.pos+1:]...)
		}
	case k.Name == "left":
		p.pos = max(0, p.pos-1)
	case k.Name == "right":
		p.pos = min(len(p.text), p.pos+1)
	case k.Name == "home":
		p.pos = 0
	case k.Name == "end":
		p.pos = len(p.text)
	case k.Name == "up":
		v.walkHistory(p, -1)
	case k.Name == "down":
		v.walkHistory(p, 1)
	case keymap.IsTyped(k):
		v.promptInsert(k.Text)
		return
	}
	if string(p.text) != before {
		v.liveSearch()
	}
}

// promptInsert puts text at the cursor of the prompt.
func (v *Vim) promptInsert(s string) {
	p := v.prompt
	rs := []rune(s)
	nt := append(append(append([]rune(nil), p.text[:p.pos]...), rs...), p.text[p.pos:]...)
	p.text = nt
	p.pos += len(rs)
	v.liveSearch()
}

// walkHistory goes one entry back (d = -1) or forward in the history of the prompt.
func (v *Vim) walkHistory(p *prompt, d int) {
	h := *v.history(p)
	if len(h) == 0 {
		return
	}
	if p.hist < 0 {
		if d > 0 {
			return
		}
		p.draft, p.hist = append([]rune(nil), p.text...), len(h)
	}
	p.hist += d
	switch {
	case p.hist < 0:
		p.hist = 0
	case p.hist >= len(h):
		p.hist = -1
		p.text = append([]rune(nil), p.draft...)
		p.pos = len(p.text)
		return
	}
	p.text = []rune(h[p.hist])
	p.pos = len(p.text)
}

// liveSearch jumps to the first match while the user types a search pattern.
func (v *Vim) liveSearch() {
	p := v.prompt
	if p == nil || p.kind == ':' {
		return
	}
	b := v.b
	b.Row, b.Col = p.orig.row, p.orig.col
	pat := string(p.text)
	p.searchSet = true
	if err := b.SetSearch(pat, p.kind == '?'); err != nil || pat == "" {
		return
	}
	b.SearchJump(false)
}

// cancelPrompt closes the prompt. A cancelled search restores the position and the search.
func (v *Vim) cancelPrompt() {
	p := v.prompt
	v.prompt, v.deferred = nil, nil
	if p.kind == ':' {
		return
	}
	b := v.b
	b.Row, b.Col = p.orig.row, p.orig.col
	if p.searchSet {
		_ = b.SetSearch(p.prevPat, p.prevBack)
		if !p.prevHL {
			b.ClearHighlight()
		}
	}
}

// pushHistory adds text to a history list. It skips an empty text and a repeat.
func pushHistory(h *[]string, text string) {
	if text == "" || (len(*h) > 0 && (*h)[len(*h)-1] == text) {
		return
	}
	*h = append(*h, text)
}

// submitPrompt runs the text of the prompt.
func (v *Vim) submitPrompt() {
	p := v.prompt
	text := string(p.text)
	def := v.deferred
	v.prompt, v.deferred = nil, nil
	pushHistory(v.history(p), text)
	if p.kind == ':' {
		v.runEx(text)
		return
	}
	b := v.b
	pat := text
	if pat == "" {
		pat = p.prevPat
	}
	b.Row, b.Col = p.orig.row, p.orig.col
	if pat == "" {
		v.status("no previous regular expression", true)
		v.fail()
		return
	}
	if err := b.SetSearch(pat, p.kind == '?'); err != nil {
		v.status("bad pattern: "+err.Error(), true)
		v.fail()
		return
	}
	wrapped, ok := b.SearchJump(false)
	if !ok {
		v.status("pattern not found: "+pat, true)
		b.Row, b.Col = p.orig.row, p.orig.col
		v.fail()
		return
	}
	if wrapped {
		v.wrapMessage(p.kind == '?')
	}
	if def == nil {
		v.marks['\''] = p.orig
		return
	}
	v.target, v.haveTgt = pos{b.Row, b.Col}, true
	b.Row, b.Col = p.orig.row, p.orig.col
	v.runCmd(*def, true)
}

// ---- ex commands ----

// parseAddr reads one line address at s[i:]. It returns the line (from 0) and the index
// after the address. ok is false if there is no address.
func (v *Vim) parseAddr(s string, i int) (line, next int, ok bool, err string) {
	b := v.b
	last := len(b.Lines) - 1
	if i >= len(s) {
		return 0, i, false, ""
	}
	switch c := s[i]; {
	case c == '.':
		line, i = b.Row, i+1
	case c == '$':
		line, i = last, i+1
	case c >= '0' && c <= '9':
		j := i
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		n, _ := strconv.Atoi(s[i:j])
		line, i = max(0, n-1), j
	case c == '\'':
		if i+1 >= len(s) {
			return 0, i, false, "mark not set"
		}
		m, found := v.marks[rune(s[i+1])]
		if !found {
			return 0, i, false, "mark not set"
		}
		line, i = m.row, i+2
	case c == '+' || c == '-':
		line = b.Row
	default:
		return 0, i, false, ""
	}
	for i < len(s) && (s[i] == '+' || s[i] == '-') {
		sign := 1
		if s[i] == '-' {
			sign = -1
		}
		i++
		j := i
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		n := 1
		if j > i {
			n, _ = strconv.Atoi(s[i:j])
		}
		line += sign * n
		i = j
	}
	return line, i, true, ""
}

// parseRange reads the range at the start of an ex command. n is the number of addresses.
func (v *Vim) parseRange(s string) (r1, r2, n int, rest, err string) {
	b := v.b
	if strings.HasPrefix(s, "%") {
		return 0, len(b.Lines) - 1, 2, s[1:], ""
	}
	a1, i, ok, e := v.parseAddr(s, 0)
	if e != "" {
		return 0, 0, 0, "", e
	}
	if !ok && !strings.HasPrefix(s, ",") {
		return b.Row, b.Row, 0, s, ""
	}
	if !ok {
		a1 = b.Row
	}
	if i < len(s) && s[i] == ',' {
		a2, j, ok2, e := v.parseAddr(s, i+1)
		if e != "" {
			return 0, 0, 0, "", e
		}
		if !ok2 {
			a2, j = b.Row, i+1
		}
		return min(a1, a2), max(a1, a2), 2, s[j:], ""
	}
	return a1, a1, 1, s[i:], ""
}

// isSubstitute reports whether an ex command is :s with a delimiter.
func isSubstitute(s string) bool {
	if len(s) < 2 || s[0] != 's' {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s[1:])
	return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsSpace(r) && r != '"' && r != '|'
}

// runEx runs an ex command line.
func (v *Vim) runEx(line string) {
	b := v.b
	s := strings.TrimLeft(line, ": \t")
	if strings.TrimSpace(s) == "" {
		return
	}
	r1, r2, n, rest, err := v.parseRange(s)
	if err != "" {
		v.status(err, true)
		v.fail()
		return
	}
	rest = strings.TrimSpace(rest)
	last := len(b.Lines) - 1
	if rest == "" {
		if n > 0 { // a range with no command goes to its last line
			v.marks['\''] = pos{b.Row, b.Col}
			b.Row = max(0, min(last, r2))
			b.Col, b.Want = firstNonBlank(b.Lines[b.Row]), -1
		}
		return
	}
	if isSubstitute(rest) {
		if r1 < 0 || r2 > last {
			v.status("invalid range", true)
			v.fail()
			return
		}
		v.substitute(r1, r2, rest[1:])
		return
	}
	name := rest
	args := ""
	if i := strings.IndexAny(rest, " \t"); i >= 0 {
		name, args = rest[:i], strings.TrimSpace(rest[i:])
	}
	bang := strings.HasSuffix(name, "!")
	name = strings.TrimSuffix(name, "!")
	switch name {
	case "w", "write":
		if args == "" {
			v.fx = append(v.fx, keymap.Save())
		} else {
			v.fx = append(v.fx, keymap.SaveAs(args))
		}
	case "q", "quit":
		switch {
		case bang:
			v.fx = append(v.fx, keymap.ForceQuit())
		case b.Dirty():
			v.status("unsaved changes · :q! discards · :wq saves", true)
		default:
			v.fx = append(v.fx, keymap.Quit())
		}
	case "wq", "x", "xit", "exit":
		v.fx = append(v.fx, keymap.SaveQuit())
	case "e", "edit":
		if args == "" {
			v.status("argument required", true)
			return
		}
		v.fx = append(v.fx, keymap.EditFile(args))
	case "noh", "nohl", "nohlsearch":
		b.ClearHighlight()
	default:
		v.status("not an editor command: "+rest, true)
		v.fail()
	}
}

// splitSub splits the text after :s into the pattern, the replacement, and the flags. d is
// the delimiter. A backslash before the delimiter takes the delimiter as a plain char.
func splitSub(s string, d rune) (pat, rep, flags string) {
	parts := make([]string, 0, 3)
	var sb strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		switch {
		case rs[i] == '\\' && i+1 < len(rs):
			if rs[i+1] == d {
				sb.WriteRune(d)
			} else {
				sb.WriteRune(rs[i])
				sb.WriteRune(rs[i+1])
			}
			i++
		case rs[i] == d && len(parts) < 2:
			parts = append(parts, sb.String())
			sb.Reset()
		default:
			sb.WriteRune(rs[i])
		}
	}
	parts = append(parts, sb.String())
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	return parts[0], parts[1], parts[2]
}

// substitute runs :s. text is the command after the s: delimiter, pattern, and the rest.
func (v *Vim) substitute(r1, r2 int, text string) {
	b := v.b
	d, _ := utf8.DecodeRuneInString(text)
	pat, rep, flags := splitSub(text[utf8.RuneLen(d):], d)
	fl, err := buffer.ParseSubFlags(strings.TrimSpace(flags))
	if err != nil {
		v.status(err.Error(), true)
		v.fail()
		return
	}
	job, err := b.NewSubstitute(r1, r2, pat, rep, fl)
	if err != nil {
		msg := err.Error()
		if !strings.HasPrefix(msg, "no previous") {
			msg = "bad pattern: " + msg
		}
		v.status(msg, true)
		v.fail()
		return
	}
	v.conf = &confirm{job: job, pat: pat}
	if fl.Confirm {
		v.confirmNext()
		return
	}
	job.ReplaceRest()
	v.conf.any = job.Result().Count > 0
	v.finishSub()
}

// confirmKey answers the question of :s with the flag c.
func (v *Vim) confirmKey(k keymap.Key) {
	c := v.conf
	switch sym(k) {
	case "y":
		c.job.Replace()
		v.confirmNext()
	case "n":
		c.job.Skip()
		v.confirmNext()
	case "a":
		c.job.ReplaceRest()
		v.finishSub()
	case "q", "esc":
		v.finishSub()
	}
}

// confirmNext goes to the next match, or it ends the job if there is none.
func (v *Vim) confirmNext() {
	if _, ok := v.conf.job.Next(); !ok {
		v.finishSub()
		return
	}
	v.conf.any = true
}

// finishSub ends :s: it closes the undo step, shows the result, and sets the search.
func (v *Vim) finishSub() {
	b := v.b
	c := v.conf
	v.conf = nil
	res := c.job.Finish()
	switch {
	case !c.any:
		shown := c.pat
		if shown == "" {
			shown = b.Search.Pattern
		}
		v.status("pattern not found: "+shown, true)
		v.fail()
		return
	case res.Count > 0:
		v.status(fmt.Sprintf("%d substitutions on %d lines", res.Count, res.Lines), false)
	}
	if c.pat != "" {
		_ = b.SetSearch(buffer.VimToGo(c.pat), b.Search.Backward)
	}
}
