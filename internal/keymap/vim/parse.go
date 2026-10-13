package vim

import (
	"strings"

	"github.com/biomassa/mded/internal/keymap"
)

// Parse results.
const (
	stIncomplete = iota // the command needs more keys
	stInvalid           // the keys are not a command
	stDone              // the command is complete
)

// cmd is a parsed normal or visual command.
type cmd struct {
	reg    rune   // register name, 0 for none
	n1, n2 int    // count before the operator and count before the motion
	has1   bool   // n1 was typed
	has2   bool   // n2 was typed
	op     string // operator: d c y > < gu gU g~
	name   string // motion, command, or text object, for example "w", "gg", "iw", "x"
	arg    rune   // argument of f F t T ' ` r m q @
	dbl    bool   // doubled operator: dd, >>, guu
	obj    bool   // name is a text object
	vline  bool   // visual: the operator works on whole lines (X D Y C S R)
	big    bool   // a delete goes to register 1 even if it is small
	body   []keymap.Key
}

// count returns the total count, 1 if none was typed.
func (c *cmd) count() int {
	n := 1
	if c.has1 {
		n *= max(1, c.n1)
	}
	if c.has2 {
		n *= max(1, c.n2)
	}
	return n
}

// hasCount reports whether the user typed a count.
func (c *cmd) hasCount() bool { return c.has1 || c.has2 }

// sym returns the text that a key stands for: its text if it types text, else its name.
func sym(k keymap.Key) string {
	if keymap.IsTyped(k) {
		return k.Text
	}
	return k.Name
}

// aliases maps the names of special keys to the vim keys that they replace.
var aliases = map[string]string{
	"left": "h", "right": "l", "up": "k", "down": "j", "home": "0", "end": "$",
	"pgdown": "ctrl+f", "pgup": "ctrl+b", "delete": "x",
}

// canon returns the vim key for k.
func canon(k keymap.Key) string {
	s := sym(k)
	if !keymap.IsTyped(k) {
		if a, ok := aliases[s]; ok {
			return a
		}
	}
	return s
}

func digit(s string) (int, bool) {
	if len(s) == 1 && s[0] >= '0' && s[0] <= '9' {
		return int(s[0] - '0'), true
	}
	return 0, false
}

// validReg reports whether r names a register.
func validReg(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
		strings.ContainsRune(`"-+*_`, r)
}

// parser reads keys one by one.
type parser struct {
	ks []keymap.Key
	i  int
	c  cmd
}

// peek returns the next key without taking it.
func (p *parser) peek() (string, bool) {
	if p.i >= len(p.ks) {
		return "", false
	}
	return canon(p.ks[p.i]), true
}

// read takes the next key and adds it to the body of the command.
func (p *parser) read() (string, bool) {
	if p.i >= len(p.ks) {
		return "", false
	}
	p.c.body = append(p.c.body, p.ks[p.i])
	p.i++
	return canon(p.ks[p.i-1]), true
}

// parse reads the keys of one command. vis selects the visual mode commands.
func parse(ks []keymap.Key, vis bool) (cmd, int) {
	p := &parser{ks: ks}
	// Registers and the first count come first, in any order.
	for {
		s, ok := p.peek()
		if !ok {
			return p.c, stIncomplete
		}
		if s == `"` {
			if p.i+1 >= len(ks) {
				return p.c, stIncomplete
			}
			r := []rune(canon(ks[p.i+1]))
			if len(r) != 1 || !validReg(r[0]) {
				return p.c, stInvalid
			}
			p.c.reg = r[0]
			p.i += 2
			continue
		}
		if d, isD := digit(s); isD && (d != 0 || p.c.has1) {
			p.c.n1, p.c.has1 = p.c.n1*10+d, true
			p.i++
			continue
		}
		break
	}
	s, _ := p.read()
	st := p.command(s, vis)
	return p.c, st
}

// visualOps maps the visual keys that act on the selection to an operator.
var visualOps = map[string]struct {
	op    string
	vline bool
}{
	"d": {"d", false}, "x": {"d", false}, "c": {"c", false}, "s": {"c", false},
	"y": {"y", false}, ">": {">", false}, "<": {"<", false},
	"~": {"g~", false}, "u": {"gu", false}, "U": {"gU", false},
	"X": {"d", true}, "D": {"d", true}, "Y": {"y", true},
	"C": {"c", true}, "S": {"c", true}, "R": {"c", true},
}

// command reads the rest of a command that starts with the key s.
func (p *parser) command(s string, vis bool) int {
	c := &p.c
	if vis {
		if v, ok := visualOps[s]; ok {
			c.op, c.vline = v.op, v.vline
			return stDone
		}
		switch s {
		case "J", "o", "O", "v", "V", ":", "m":
			c.name = s
			if s == "m" {
				return p.withArg()
			}
			return stDone
		case "i", "a":
			return p.object(s)
		}
	}
	switch s {
	case "d", "c", "y", ">", "<":
		return p.operand(s)
	case "g":
		s2, ok := p.read()
		if !ok {
			return stIncomplete
		}
		switch s2 {
		case "u", "U", "~":
			if vis {
				c.op = "g" + s2
				return stDone
			}
			return p.operand("g" + s2)
		case "J":
			c.name = "gJ"
			return stDone
		}
		return p.motionG(s2)
	}
	if !vis {
		switch s {
		case "x", "X", "s", "S", "J", "~", "p", "P", "i", "a", "I", "A", "o", "O", "R", "u",
			"ctrl+r", ".", "D", "C", "Y", "v", "V", ":", "ctrl+s":
			c.name = s
			return stDone
		case "r", "m", "q", "@":
			c.name = s
			return p.withArg()
		}
	}
	return p.motion(s, false)
}

// withArg reads the one key that a command takes as its argument.
func (p *parser) withArg() int {
	a, ok := p.read()
	if !ok {
		return stIncomplete
	}
	r := []rune(a)
	if len(r) != 1 {
		if a == "enter" && p.c.name == "r" {
			p.c.arg = '\n'
			return stDone
		}
		return stInvalid
	}
	p.c.arg = r[0]
	return stDone
}

// operand reads what follows an operator: a count, a doubled key, a text object, or a motion.
func (p *parser) operand(opKey string) int {
	c := &p.c
	c.op = opKey
	for {
		s, ok := p.peek()
		if !ok {
			return stIncomplete
		}
		d, isD := digit(s)
		if !isD || (d == 0 && !c.has2) {
			break
		}
		c.n2, c.has2 = c.n2*10+d, true
		p.i++
	}
	s, _ := p.read()
	last := opKey[len(opKey)-1:]
	switch {
	case len(opKey) == 1 && s == opKey, len(opKey) == 2 && s == last:
		c.dbl = true
		return stDone
	case s == "g":
		s2, ok := p.read()
		if !ok {
			return stIncomplete
		}
		if len(opKey) == 2 && s2 == last {
			c.dbl = true
			return stDone
		}
		return p.motionG(s2)
	case s == "i" || s == "a":
		return p.object(s)
	}
	return p.motion(s, true)
}

// object reads the char of a text object.
func (p *parser) object(kind string) int {
	a, ok := p.read()
	if !ok {
		return stIncomplete
	}
	if len(a) != 1 || !strings.Contains("wWspb()[]{}B<>\"'`", a) {
		return stInvalid
	}
	p.c.name, p.c.arg, p.c.obj = kind+a, rune(a[0]), true
	return stDone
}

// motionG finishes a motion that starts with g.
func (p *parser) motionG(s2 string) int {
	switch s2 {
	case "g", "e", "E":
		p.c.name = "g" + s2
		return stDone
	}
	return stInvalid
}

// motion finishes a motion that starts with the key s. op is true after an operator.
func (p *parser) motion(s string, op bool) int {
	c := &p.c
	switch s {
	case "h", "j", "k", "l", "w", "W", "b", "B", "e", "E", "0", "^", "$", "G", "H", "M", "L",
		"%", "{", "}", ";", ",", "n", "N", "*", "#", "/", "?":
		c.name = s
		return stDone
	case "ctrl+d", "ctrl+u", "ctrl+f", "ctrl+b":
		if op {
			return stInvalid
		}
		c.name = s
		return stDone
	case "f", "F", "t", "T", "'", "`":
		c.name = s
		return p.withArg()
	case "g":
		s2, ok := p.read()
		if !ok {
			return stIncomplete
		}
		return p.motionG(s2)
	}
	return stInvalid
}
