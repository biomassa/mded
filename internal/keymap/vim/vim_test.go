package vim

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/biomassa/mded/internal/buffer"
	"github.com/biomassa/mded/internal/keymap"
)

// special maps the <name> tokens of a key script to key names.
var special = map[string]string{
	"esc": "esc", "cr": "enter", "bs": "backspace", "a-bs": "alt+backspace", "c-r": "ctrl+r",
	"c-g": "ctrl+g", "c-p": "ctrl+p", "c-s": "ctrl+s", "left": "left", "right": "right", "up": "up",
	"down": "down", "tab": "tab", "del": "delete", "c-d": "ctrl+d", "c-u": "ctrl+u",
	"c-f": "ctrl+f", "c-b": "ctrl+b", "home": "home", "end": "end",
}

// keys turns a script such as "2dw<esc>" into keys.
func keys(script string) []keymap.Key {
	var out []keymap.Key
	for len(script) > 0 {
		if script[0] == '<' {
			if j := strings.IndexByte(script, '>'); j > 0 {
				name := script[1:j]
				if name == "lt" {
					out = append(out, keymap.Key{Name: "<", Text: "<"})
					script = script[j+1:]
					continue
				}
				if sp, ok := special[name]; ok {
					out = append(out, keymap.Key{Name: sp})
					script = script[j+1:]
					continue
				}
			}
		}
		r, n := utf8.DecodeRuneInString(script)
		script = script[n:]
		k := keymap.Key{Name: string(r), Text: string(r)}
		if r == ' ' {
			k.Name = "space"
		}
		out = append(out, k)
	}
	return out
}

// run feeds a key script and returns all effects.
func run(v *Vim, b *buffer.Buffer, script string) []keymap.Effect {
	var all []keymap.Effect
	for _, k := range keys(script) {
		all = append(all, v.Handle(b, k, 40, 10)...)
	}
	return all
}

// statuses returns the status messages of the effects.
func statuses(fx []keymap.Effect) []string {
	var out []string
	for _, e := range fx {
		if e.Kind == keymap.KindStatus {
			out = append(out, e.Text)
		}
	}
	return out
}

func hasStatus(fx []keymap.Effect, text string) bool {
	for _, s := range statuses(fx) {
		if s == text {
			return true
		}
	}
	return false
}

func TestCommands(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		row, col int
		script   string
		want     string
		wr, wc   int
	}{
		// Motions and counts.
		{"l count", "abcdef", 0, 0, "3l", "abcdef", 0, 3},
		{"l stops at end", "abc", 0, 0, "9l", "abc", 0, 2},
		{"h", "abcdef", 0, 4, "2h", "abcdef", 0, 2},
		{"j k keep column", "abcdef\nab\nabcdef", 0, 4, "jj", "abcdef\nab\nabcdef", 2, 4},
		{"k count", "a\nb\nc\nd", 3, 0, "2k", "a\nb\nc\nd", 1, 0},
		{"w", "foo bar baz", 0, 0, "w", "foo bar baz", 0, 4},
		{"2w", "foo bar baz", 0, 0, "2w", "foo bar baz", 0, 8},
		{"w punct", "foo.bar", 0, 0, "w", "foo.bar", 0, 3},
		{"W", "foo.bar baz", 0, 0, "W", "foo.bar baz", 0, 8},
		{"w next line", "foo\nbar", 0, 0, "w", "foo\nbar", 1, 0},
		{"w empty line", "foo\n\nbar", 0, 0, "w", "foo\n\nbar", 1, 0},
		{"b", "foo bar", 0, 6, "b", "foo bar", 0, 4},
		{"b prev line", "foo\nbar", 1, 0, "b", "foo\nbar", 0, 0},
		{"e", "foo bar", 0, 0, "e", "foo bar", 0, 2},
		{"e again", "foo bar", 0, 2, "e", "foo bar", 0, 6},
		{"ge", "foo bar", 0, 5, "ge", "foo bar", 0, 2},
		{"0 $ ^", "  foo bar", 0, 4, "$", "  foo bar", 0, 8},
		{"caret", "  foo bar", 0, 6, "^", "  foo bar", 0, 2},
		{"zero", "  foo bar", 0, 6, "0", "  foo bar", 0, 0},
		{"gg G", "a\nb\nc", 1, 0, "G", "a\nb\nc", 2, 0},
		{"gg", "a\nb\nc", 2, 0, "gg", "a\nb\nc", 0, 0},
		{"NG", "a\nb\nc\nd", 0, 0, "3G", "a\nb\nc\nd", 2, 0},
		{"Ngg", "a\nb\nc\nd", 0, 0, "2gg", "a\nb\nc\nd", 1, 0},
		{"f", "a,b,c", 0, 0, "f,", "a,b,c", 0, 1},
		{"2f", "a,b,c", 0, 0, "2f,", "a,b,c", 0, 3},
		{"F", "a,b,c", 0, 4, "F,", "a,b,c", 0, 3},
		{"t", "a,b,c", 0, 0, "t,", "a,b,c", 0, 0},
		{"t from before", "abc,def", 0, 0, "t,", "abc,def", 0, 2},
		{"T", "a,bcd", 0, 4, "T,", "a,bcd", 0, 2},
		{"; repeat", "a,b,c,d", 0, 0, "f,;", "a,b,c,d", 0, 3},
		{", reverse", "a,b,c,d", 0, 0, "f,;,", "a,b,c,d", 0, 1},
		{"t ; skips adjacent", "a,b,c,d", 0, 0, "t,;", "a,b,c,d", 0, 2},
		{"percent", "a(b[c]d)e", 0, 1, "%", "a(b[c]d)e", 0, 7},
		{"percent back", "a(b[c]d)e", 0, 7, "%", "a(b[c]d)e", 0, 1},
		{"percent multiline", "{\n a\n}", 0, 0, "%", "{\n a\n}", 2, 0},
		{"H", "1\n2\n3\n4\n5", 4, 0, "H", "1\n2\n3\n4\n5", 0, 0},
		{"L", "1\n2\n3\n4\n5", 0, 0, "L", "1\n2\n3\n4\n5", 4, 0},
		{"M", "1\n2\n3\n4\n5", 0, 0, "M", "1\n2\n3\n4\n5", 2, 0},
		{"2H", "1\n2\n3\n4\n5", 4, 0, "2H", "1\n2\n3\n4\n5", 1, 0},
		{"paragraph fwd", "a\nb\n\nc\n\nd", 0, 0, "}", "a\nb\n\nc\n\nd", 2, 0},
		{"paragraph fwd 2", "a\nb\n\nc\n\nd", 0, 0, "2}", "a\nb\n\nc\n\nd", 4, 0},
		{"paragraph back", "a\nb\n\nc\n\nd", 5, 0, "{", "a\nb\n\nc\n\nd", 4, 0},
		{"arrow keys", "ab\ncd", 0, 0, "<right><down><left><up>", "ab\ncd", 0, 0},
		{"ctrl+d", "1\n2\n3\n4\n5\n6\n7\n8", 0, 0, "<c-d>", "1\n2\n3\n4\n5\n6\n7\n8", 5, 0},

		// Operators.
		{"x", "abc", 0, 1, "x", "ac", 0, 1},
		{"x last", "abc", 0, 2, "x", "ab", 0, 1},
		{"3x", "abcdef", 0, 1, "3x", "aef", 0, 1},
		{"X", "abc", 0, 2, "X", "ac", 0, 1},
		{"dw", "foo bar baz", 0, 4, "dw", "foo baz", 0, 4},
		{"dw last word", "foo bar\nbaz", 0, 4, "dw", "foo \nbaz", 0, 3},
		{"2dw", "a b c d", 0, 0, "2dw", "c d", 0, 0},
		{"d2w", "a b c d", 0, 0, "d2w", "c d", 0, 0},
		{"cw", "foo bar", 0, 0, "cwxy<esc>", "xy bar", 0, 1},
		{"cw on last char", "foo bar", 0, 2, "cwX<esc>", "foX bar", 0, 2},
		{"2cw", "a b c", 0, 0, "2cwZ<esc>", "Z c", 0, 0},
		{"ciw", "foo bar", 0, 5, "ciwX<esc>", "foo X", 0, 4},
		{"diw", "foo bar baz", 0, 5, "diw", "foo  baz", 0, 4},
		{"daw", "foo bar baz", 0, 5, "daw", "foo baz", 0, 4},
		{"daw last", "foo bar", 0, 5, "daw", "foo", 0, 2},
		{"diW", "a.b c", 0, 1, "diW", " c", 0, 0},
		{"di(", "f(a, b)", 0, 3, "di(", "f()", 0, 2},
		{"da(", "f(a, b) x", 0, 3, "da(", "f x", 0, 1},
		{"dib", "f(a)", 0, 2, "dib", "f()", 0, 2},
		{"di( nested", "f(a(b)c)", 0, 4, "di(", "f(a()c)", 0, 4},
		{"d2i(", "f(a(b)c)", 0, 4, "d2i(", "f()", 0, 2},
		{"di[", "x[1,2]", 0, 3, "di[", "x[]", 0, 2},
		{"di{ lines", "if {\n  a\n  b\n}", 1, 2, "di{", "if {\n}", 1, 0},
		{"da{", "x {a} y", 0, 3, "da{", "x  y", 0, 2},
		{"diB", "{a}", 0, 1, "diB", "{}", 0, 1},
		{"di<", "<abc>", 0, 2, "di<", "<>", 0, 1},
		{"di\"", `say "hi there" ok`, 0, 6, `di"`, `say "" ok`, 0, 5},
		{"da\"", `say "hi" ok`, 0, 6, `da"`, `say ok`, 0, 4},
		{"di\" before", `say "hi" ok`, 0, 0, `di"`, `say "" ok`, 0, 5},
		{"ci'", "a 'x y' b", 0, 4, "ci'Z<esc>", "a 'Z' b", 0, 3},
		{"di`", "a `x` b", 0, 3, "di`", "a `` b", 0, 3},
		{"dip", "a\nb\n\nc", 0, 0, "dip", "\nc", 0, 0},
		{"dap", "a\nb\n\nc", 0, 0, "dap", "c", 0, 0},
		{"dis", "One. Two. Three.", 0, 6, "dis", "One.  Three.", 0, 5},
		{"das", "One. Two. Three.", 0, 6, "das", "One. Three.", 0, 5},
		{"dd", "a\nb\nc", 1, 0, "dd", "a\nc", 1, 0},
		{"dd last", "a\nb", 1, 0, "dd", "a", 0, 0},
		{"dd only line", "a", 0, 0, "dd", "", 0, 0},
		{"2dd", "a\nb\nc\nd", 1, 0, "2dd", "a\nd", 1, 0},
		{"dd too many", "a\nb\nc", 1, 0, "9dd", "a", 0, 0},
		{"dj", "a\nb\nc", 0, 0, "dj", "c", 0, 0},
		{"dk", "a\nb\nc", 2, 0, "dk", "a", 0, 0},
		{"dG", "a\nb\nc", 1, 0, "dG", "a", 0, 0},
		{"dgg", "a\nb\nc", 1, 0, "dgg", "c", 0, 0},
		{"D", "foo bar", 0, 3, "D", "foo", 0, 2},
		{"2D", "ab\ncd\nef", 0, 1, "2D", "a\nef", 0, 0},
		{"C", "foo bar", 0, 3, "CX<esc>", "fooX", 0, 3},
		{"cc", "  foo\nb", 0, 3, "ccX<esc>", "X\nb", 0, 0},
		{"S", "foo\nb", 0, 1, "SX<esc>", "X\nb", 0, 0},
		{"s", "abc", 0, 1, "sX<esc>", "aXc", 0, 1},
		{"3s", "abcde", 0, 1, "3sX<esc>", "aXe", 0, 1},
		{"d$", "foo bar", 0, 3, "d$", "foo", 0, 2},
		{"d0", "foo bar", 0, 4, "d0", "bar", 0, 0},
		{"d^", "  foo bar", 0, 6, "d^", "  bar", 0, 2},
		{"de", "foo bar", 0, 0, "de", " bar", 0, 0},
		{"db", "foo bar", 0, 4, "db", "bar", 0, 0},
		{"dfx", "abxcd", 0, 0, "dfx", "cd", 0, 0},
		{"dtx", "abxcd", 0, 0, "dtx", "xcd", 0, 0},
		{"dFx", "abxcd", 0, 4, "dFx", "abd", 0, 2},
		{"d%", "f(a)b", 0, 1, "d%", "fb", 0, 1},
		{"d}", "a\nb\n\nc", 0, 0, "d}", "\nc", 0, 0},
		{"d} from mid line", "ab\nb\n\nc", 0, 1, "d}", "a\n\nc", 0, 0},
		{"yy p", "a\nb", 0, 0, "yyp", "a\na\nb", 1, 0},
		{"yy P", "a\nb", 1, 0, "yyP", "a\nb\nb", 1, 0},
		{"2yy p", "a\nb\nc", 0, 0, "2yyjp", "a\nb\na\nb\nc", 2, 0},
		{"yw p", "foo bar", 0, 0, "ywwP", "foo foo bar", 0, 7},
		{"p count", "a", 0, 0, "yl3p", "aaaa", 0, 3},
		{"dd p", "a\nb\nc", 0, 0, "ddp", "b\na\nc", 1, 0},
		{"xp swap", "ab", 0, 0, "xp", "ba", 0, 1},
		{"p multiline charwise", "a b\nc", 0, 0, "vjy$p", "a ba b\nc\nc", 0, 3},
		{"3p linewise", "x", 0, 0, "yy2p", "x\nx\nx", 1, 0},
		{"J", "a\n  b\nc", 0, 0, "J", "a b\nc", 0, 1},
		{"3J", "a\nb\nc\nd", 0, 0, "3J", "a b c\nd", 0, 3},
		{"gJ", "a\n b", 0, 0, "gJ", "a b", 0, 1},
		{"~", "abc", 0, 0, "~", "Abc", 0, 1},
		{"3~", "abcd", 0, 0, "3~", "ABCd", 0, 3},
		{"r", "abc", 0, 1, "rX", "aXc", 0, 1},
		{"3r", "abcd", 0, 0, "3rX", "XXXd", 0, 2},
		{"r too long", "ab", 0, 0, "3rX", "ab", 0, 0},
		{"r enter", "ab", 0, 1, "r<cr>", "a\n", 1, 0},
		{"guw", "ABC DEF", 0, 0, "guw", "abc DEF", 0, 0},
		{"gUw", "abc def", 0, 0, "gUw", "ABC def", 0, 0},
		{"g~w", "aBc def", 0, 0, "g~w", "AbC def", 0, 0},
		{"guu", "ABC\nDEF", 0, 1, "guu", "abc\nDEF", 0, 1},
		{"gUU", "abc\ndef", 0, 1, "gUU", "ABC\ndef", 0, 1},
		{"g~~", "aB", 0, 0, "g~~", "Ab", 0, 0},
		{"gUiw", "foo bar", 0, 5, "gUiw", "foo BAR", 0, 4},
		{">>", "a\nb", 0, 0, ">>", "    a\nb", 0, 4},
		{"<<", "      a", 0, 6, "<<", "  a", 0, 2},
		{"2>>", "a\nb\nc", 0, 0, "2>>", "    a\n    b\nc", 0, 4},
		{">j", "a\nb\nc", 0, 0, ">j", "    a\n    b\nc", 0, 4},
		{"yiw P", "foo bar", 0, 4, "yiwP", "foo barbar", 0, 6},

		// Insert and open.
		{"i", "ab", 0, 1, "iX<esc>", "aXb", 0, 1},
		{"a", "ab", 0, 0, "aX<esc>", "aXb", 0, 1},
		{"A", "ab", 0, 0, "AX<esc>", "abX", 0, 2},
		{"I", "  ab", 0, 3, "IX<esc>", "  Xab", 0, 2},
		{"o", "a\nb", 0, 0, "oX<esc>", "a\nX\nb", 1, 0},
		{"O", "a\nb", 1, 0, "OX<esc>", "a\nX\nb", 1, 0},
		{"3ix", "a", 0, 0, "3ix<esc>", "xxxa", 0, 2},
		{"3o", "a", 0, 0, "3ox<esc>", "a\nx\nx\nx", 3, 0},
		{"insert enter list", "- a", 0, 2, "A<cr>b<esc>", "- a\n- b", 1, 2},
		{"insert backspace", "ab", 0, 1, "ix<bs><bs><esc>", "b", 0, 0},
		{"insert alt+bs", "foo bar", 0, 7, "A<a-bs><esc>", "foo ", 0, 3},
		{"insert tab", "a", 0, 0, "i<tab><esc>", "    a", 0, 3},
		{"insert arrows", "ab", 0, 0, "A<left>X<esc>", "aXb", 0, 1},
		{"insert esc moves left", "ab", 0, 0, "Ax<esc>", "abx", 0, 2},
		{"insert multi-line text", "a", 0, 0, "Ab<cr>c<esc>", "ab\nc", 1, 0},

		// Replace mode.
		{"R", "abcd", 0, 0, "RXY<esc>", "XYcd", 0, 1},
		{"R past end", "ab", 0, 1, "RXYZ<esc>", "aXYZ", 0, 3},
		{"R backspace", "abcd", 0, 0, "RXY<bs><esc>", "Xbcd", 0, 0},
		{"R backspace appended", "ab", 0, 1, "RXY<bs><bs><esc>", "ab", 0, 0},

		// Undo and redo.
		{"undo insert session", "a", 0, 0, "Abc<esc>u", "a", 0, 0},
		{"undo twice", "a", 0, 0, "xix<esc>uu", "a", 0, 0},
		{"redo", "a", 0, 0, "Ab<esc>u<c-r>", "ab", 0, 1},
		{"undo cw", "foo bar", 0, 0, "cwX<esc>u", "foo bar", 0, 0},
		{"2u", "abc", 0, 0, "xx2u", "abc", 0, 0},

		// Dot.
		{"dot x", "abcd", 0, 0, "x.", "cd", 0, 0},
		{"dot count", "abcdef", 0, 0, "x3.", "ef", 0, 0},
		{"dot dd", "a\nb\nc", 0, 0, "dd.", "c", 0, 0},
		{"dot cw", "a b c", 0, 0, "cwX<esc>w.", "X X c", 0, 2},
		{"dot A", "a\nb", 0, 0, "A!<esc>j.", "a!\nb!", 1, 1},
		{"dot o", "a", 0, 0, "ox<esc>.", "a\nx\nx", 2, 0},
		{"dot dw", "a b c d", 0, 0, "dw.", "c d", 0, 0},
		{"dot i", "a", 0, 0, "ix<esc>.", "xxa", 0, 0},
		{"dot register", "a\nb\nc", 0, 0, "\"add.\"ap", "c\nb", 1, 0},
		{"dot p", "a", 0, 0, "ylp.", "aaa", 0, 2},
		{"dot r", "abc", 0, 0, "rXl.", "XXc", 0, 1},
		{"dot >>", "a", 0, 0, ">>.", "        a", 0, 8},
		{"dot J", "a\nb\nc", 0, 0, "J.", "a b c", 0, 3},
		{"dot after undo keeps", "abc", 0, 0, "xu.", "bc", 0, 0},

		// Registers.
		{"named yank put", "foo\nbar", 0, 0, "\"ayyj\"ap", "foo\nbar\nfoo", 2, 0},
		{"unnamed after named", "foo", 0, 0, "\"ayiwp", "ffoooo", 0, 3},
		{"append upper", "a\nb", 0, 0, "\"ayyj\"Ayy\"ap", "a\nb\na\nb", 2, 0},
		{"append charwise", "ab cd", 0, 0, "\"ayiww\"Ayiw$\"ap", "ab cdabcd", 0, 8},
		{"yank register 0", "a\nb", 0, 0, "yyjdd\"0p", "a\na", 1, 0},
		{"delete shifts 1 2", "a\nb\nc", 0, 0, "dddd\"1p\"2p", "c\nb\na", 2, 0},
		{"small delete register -", "foo bar", 0, 0, "dwyl\"-P", "foo bar", 0, 3},
		{"black hole", "a\nb", 0, 0, "yyj\"_ddp", "a\na", 1, 0},
		{"black hole keeps unnamed", "ab", 0, 0, "yl\"_xp", "ba", 0, 1},
		{"register digits d", "a b", 0, 0, "dw\"-p", "ba ", 0, 2},
		{"numbered not for named", "a\nb", 0, 0, "\"add\"1p", "b", 0, 0},

		// Visual.
		{"v d", "abcdef", 0, 1, "vld", "adef", 0, 1},
		{"v y p", "abc", 0, 0, "vly$p", "abcab", 0, 4},
		{"v c", "abcdef", 0, 1, "vlcX<esc>", "aXdef", 0, 1},
		{"v x", "abc", 0, 0, "vx", "bc", 0, 0},
		{"v >", "a\nb", 0, 0, "vj>", "    a\n    b", 0, 4},
		{"v 2>", "a", 0, 0, "v2>", "        a", 0, 8},
		{"v <", "    a\n    b", 0, 0, "vj<", "a\nb", 0, 0},
		{"v ~", "abc", 0, 0, "vl~", "ABc", 0, 0},
		{"v u", "ABC", 0, 0, "vlu", "abC", 0, 0},
		{"v U", "abc", 0, 0, "vlU", "ABc", 0, 0},
		{"v J", "a\nb\nc", 0, 0, "vjJ", "a b\nc", 0, 1},
		{"v o", "abcdef", 0, 1, "vlohd", "def", 0, 0},
		{"v esc", "abc", 0, 0, "vl<esc>x", "ac", 0, 1},
		{"v dollar d", "ab\ncd", 0, 0, "v$d", "cd", 0, 0},
		{"v iw", "foo bar", 0, 5, "viwd", "foo ", 0, 3},
		{"v aw", "foo bar baz", 0, 5, "vawd", "foo baz", 0, 4},
		{"v i(", "f(a b)", 0, 3, "vi(d", "f()", 0, 2},
		{"v ip", "a\nb\n\nc", 0, 0, "vipd", "\nc", 0, 0},
		{"v with register", "ab", 0, 0, "v\"ayl\"ap", "aba", 0, 2},
		{"v count motion", "abcdef", 0, 0, "v3ld", "ef", 0, 0},
		{"V d", "a\nb\nc", 1, 0, "Vd", "a\nc", 1, 0},
		{"V j d", "a\nb\nc", 0, 0, "Vjd", "c", 0, 0},
		{"V y p", "a\nb", 0, 0, "Vyjp", "a\nb\na", 2, 0},
		{"V >", "a\nb", 0, 0, "V>", "    a\nb", 0, 4},
		{"V c", "a\nb\nc", 0, 0, "VjcX<esc>", "X\nc", 0, 0},
		{"V J", "a\nb\nc", 0, 0, "VjjJ", "a b c", 0, 3},
		{"V U", "ab\ncd", 0, 0, "VU", "AB\ncd", 0, 0},
		{"V to v", "abc", 0, 0, "Vvld", "c", 0, 0},
		{"v to V", "a\nb\nc", 0, 0, "vVd", "b\nc", 0, 0},
		{"visual y goes to start", "abcdef", 0, 4, "vhhy", "abcdef", 0, 2},

		// Marks.
		{"mark backtick", "abc\ndef", 0, 2, "majj`a", "abc\ndef", 0, 2},
		{"mark quote", "abc\n  def", 0, 2, "majj'a", "abc\n  def", 0, 0},
		{"d to mark", "abcdef", 0, 1, "ma$d`a", "af", 0, 1},
		{"d line to mark", "a\nb\nc", 0, 0, "majjd'a", "", 0, 0},
		{"back to previous", "a\nb\nc", 0, 0, "G``", "a\nb\nc", 0, 0},

		// Search motions.
		{"search n", "a x\nb x\nc x", 0, 0, "/x<cr>n", "a x\nb x\nc x", 1, 2},
		{"search N", "a x\nb x\nc x", 0, 0, "/x<cr>nN", "a x\nb x\nc x", 0, 2},
		{"search backward", "x a\nb\nx c", 2, 2, "?x<cr>", "x a\nb\nx c", 2, 0},
		{"? then n goes up", "x\nx\nx", 2, 0, "?x<cr>n", "x\nx\nx", 0, 0},
		{"d search", "a b c d", 0, 0, "d/c<cr>", "c d", 0, 0},
		{"d n", "a x b x c", 0, 0, "/x<cr>0dn", "x b x c", 0, 0},
		{"search esc restores", "a\nb\nc", 0, 0, "/c<esc>", "a\nb\nc", 0, 0},
		{"search live then esc", "a\nb\nc", 0, 0, "/b<esc>x", "\nb\nc", 0, 0},
		{"star", "foo bar\nfoo", 0, 0, "*", "foo bar\nfoo", 1, 0},
		{"star wraps", "foo bar\nfoo", 1, 0, "*", "foo bar\nfoo", 0, 0},
		{"hash", "foo bar\nfoo", 1, 0, "#", "foo bar\nfoo", 0, 0},
		{"star whole word", "foo foobar foo", 0, 0, "*", "foo foobar foo", 0, 11},
		{"search smartcase", "a Foo foo", 0, 0, "/Foo<cr>", "a Foo foo", 0, 2},
		{"search history", "ab cd ab", 0, 0, "/cd<cr>/<up><cr>", "ab cd ab", 0, 3},
		{"search empty reuses", "x x x", 0, 0, "/x<cr>/<cr>", "x x x", 0, 4},
		{"search uses regexp", "a1 b22", 0, 0, "/[a-z]2+<cr>", "a1 b22", 0, 3},
		{"search not found stays", "abc", 0, 1, "/zzz<cr>", "abc", 0, 1},
		{"search bad pattern", "abc", 0, 1, "/(<cr>", "abc", 0, 1},

		// Ex.
		{":N", "a\nb\nc", 0, 0, ":3<cr>", "a\nb\nc", 2, 0},
		{":$", "a\nb\nc", 0, 0, ":$<cr>", "a\nb\nc", 2, 0},
		{":N too big", "a\nb", 0, 0, ":99<cr>", "a\nb", 1, 0},
		{":s", "aa bb", 0, 0, ":s/a/x/<cr>", "xa bb", 0, 0},
		{":s g", "aa bb", 0, 0, ":s/a/x/g<cr>", "xx bb", 0, 0},
		{":%s", "a\na\nb", 0, 0, ":%s/a/x/<cr>", "x\nx\nb", 1, 0},
		{":s range", "a\na\na\na", 0, 0, ":2,3s/a/x/<cr>", "a\nx\nx\na", 2, 0},
		{":s dot dollar", "a\na\na", 0, 0, ":.,$s/a/x/<cr>", "x\nx\nx", 2, 0},
		{":s other delim", "a/b", 0, 0, ":s#/#-#<cr>", "a-b", 0, 0},
		{":s escaped delim", "a/b", 0, 0, `:s/\//-/<cr>`, "a-b", 0, 0},
		{":s groups", "ab", 0, 0, `:s/\(a\)\(b\)/\2\1/<cr>`, "ba", 0, 0},
		{":s amp", "ab", 0, 0, ":s/b/[&]/<cr>", "a[b]", 0, 0},
		{":s newline", "a,b", 0, 0, `:s/,/\n/<cr>`, "a\nb", 1, 0},
		{":s i flag", "ABC", 0, 0, ":s/b/x/i<cr>", "AxC", 0, 0},
		{":s I flag", "abc ABC", 0, 0, ":s/b/x/I<cr>", "axc ABC", 0, 0},
		{":s smartcase", "a B b", 0, 0, ":s/B/x/g<cr>", "a x b", 0, 0},
		{":s word boundary", "cat concat", 0, 0, `:s/\<cat\>/x/g<cr>`, "x concat", 0, 0},
		{":s no replacement", "abc", 0, 0, ":s/b<cr>", "ac", 0, 0},
		{":s visual range", "a\na\na", 0, 0, "Vj:s/a/x/<cr>", "x\nx\na", 1, 0},
		{":s mark range", "a\na\na", 0, 0, "majjmb:'a,'bs/a/x/<cr>", "x\nx\nx", 2, 0},
		{":s undo is one step", "a\na", 0, 0, ":%s/a/x/<cr>u", "a\na", 0, 0},
		{":s empty pattern uses search", "ab ab", 0, 0, "/b<cr>:s//X/g<cr>", "aX aX", 0, 0},
		{":s go regexp", "aaa", 0, 0, ":s/a+/x/<cr>", "x", 0, 0},
		{":s confirm y n", "a a a", 0, 0, ":s/a/x/gc<cr>yny", "x a x", 0, 0},
		{":s confirm a", "a a a", 0, 0, ":s/a/x/gc<cr>na", "a x x", 0, 0},
		{":s confirm q", "a a a", 0, 0, ":s/a/x/gc<cr>yq", "x a a", 0, 0},
		{":s confirm esc", "a a a", 0, 0, ":s/a/x/gc<cr>y<esc>", "x a a", 0, 0},
		{":s confirm one undo", "a a", 0, 0, ":s/a/x/gc<cr>yyu", "a a", 0, 0},
		{":s confirm multiline", "a a\na", 0, 0, ":%s/a/x/gc<cr>yyy", "x x\nx", 1, 0},
		{":noh", "a", 0, 0, "/a<cr>:noh<cr>", "a", 0, 0},
		{"ex backspace", "a\nb\nc", 0, 0, ":4<bs>3<cr>", "a\nb\nc", 2, 0},
		{"ex history", "a\nb\nc", 0, 0, ":3<cr>gg:<up><cr>", "a\nb\nc", 2, 0},

		// Macros.
		{"macro", "a\nb\nc", 0, 0, "qaA!<esc>jq@a", "a!\nb!\nc", 2, 0},
		{"macro count", "a\nb\nc\nd", 0, 0, "qaA!<esc>jq2@a", "a!\nb!\nc!\nd", 3, 0},
		{"macro @@", "a\nb\nc\nd", 0, 0, "qaA!<esc>jq@a@@", "a!\nb!\nc!\nd", 3, 0},
		{"macro stops on fail", "a\nb", 0, 0, "qaA!<esc>jq9@a", "a!\nb!", 1, 1},
		{"macro recursive", "a\nb\nc", 0, 0, "qaqqaA!<esc>j@aq@a", "a!\nb!\nc!", 2, 1},
		{"macro append", "a", 0, 0, "qaA1<esc>qqAA2<esc>q@a", "a1212", 0, 4},
		{"macro with search", "x a\nx b", 0, 0, "qa/x<cr>iY<esc>q@a", "x a\nYYx b", 1, 1},
		{"macro dot", "abcd", 0, 0, "qaxq@a.", "d", 0, 0},
		{"macro undo", "abc", 0, 0, "qaxq@au", "bc", 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v, b := New(4), buffer.New(tc.text)
			b.Row, b.Col = tc.row, tc.col
			run(v, b, tc.script)
			if got := b.Text(); got != tc.want {
				t.Errorf("text = %q, want %q", got, tc.want)
			}
			if b.Row != tc.wr || b.Col != tc.wc {
				t.Errorf("cursor = %d,%d, want %d,%d", b.Row, b.Col, tc.wr, tc.wc)
			}
		})
	}
}

func TestOpenLineContinuesList(t *testing.T) {
	tests := []struct {
		name, text, script, want string
		row, col                 int
	}{
		{"o bullet", "- item", "oabc<esc>", "- item\n- abc", 1, 4},
		{"o checkbox open", "- [ ] x", "o<esc>", "- [ ] x\n- [ ] ", 1, 5},
		{"o checkbox done", "  - [x] x", "oy<esc>", "  - [x] x\n  - [ ] y", 1, 8},
		{"o number", "3. x", "oy<esc>", "3. x\n4. y", 1, 3},
		{"O number", "3. x", "Oy<esc>", "3. y\n3. x", 0, 3},
		{"O bullet", "- x", "Oy<esc>", "- y\n- x", 0, 2},
		{"o empty item", "  - ", "oy<esc>", "  - \n  y", 1, 2},
		{"o plain", "text", "oy<esc>", "text\ny", 1, 0},
		{"2o bullet", "- a", "2ob<esc>", "- a\n- b\n- b", 2, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v, b := New(4), buffer.New(tc.text)
			run(v, b, tc.script)
			if b.Text() != tc.want || b.Row != tc.row || b.Col != tc.col {
				t.Fatalf("got %q at %d:%d, want %q at %d:%d", b.Text(), b.Row, b.Col, tc.want, tc.row, tc.col)
			}
			run(v, b, "u")
			if b.Text() != tc.text {
				t.Fatalf("undo gave %q", b.Text())
			}
		})
	}
}

func TestStartVisual(t *testing.T) {
	v, b := New(4), buffer.New("abcdef\nxyz")
	v.StartVisual(b, 0, 1)
	b.Row, b.Col = 1, 1
	if v.Mode() != "visual" || !b.SelOn || b.SelRow != 0 || b.SelCol != 1 {
		t.Fatalf("mode %s sel %v %d:%d", v.Mode(), b.SelOn, b.SelRow, b.SelCol)
	}
	run(v, b, "d")
	if b.Text() != "az" {
		t.Fatalf("text %q", b.Text())
	}
	v.StartVisual(b, 0, 0)
	v.LeaveVisual(b)
	if v.Mode() != "normal" || b.SelOn {
		t.Fatal("LeaveVisual")
	}
	run(v, b, "i")
	v.StartVisual(b, 0, 0)
	if v.Mode() != "insert" {
		t.Fatal("insert mode must stay")
	}
}
