package vim

import (
	"testing"

	"github.com/biomassa/mded/internal/buffer"
	"github.com/biomassa/mded/internal/keymap"
)

// textCase is a command test: the text and cursor before, the keys, the text and cursor
// after. The wanted values come from vim 9.2.
type textCase struct {
	name     string
	text     string
	row, col int
	script   string
	want     string
	wr, wc   int
}

func runTextCases(t *testing.T, tests []textCase) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := buffer.New(tc.text)
			b.Row, b.Col = tc.row, tc.col
			v := New(4)
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

// TestVisualDot checks that the dot command repeats a change from visual mode.
func TestVisualDot(t *testing.T) {
	runTextCases(t, []textCase{
		{"v d l", "abcdefgh", 0, 0, "vld.", "efgh", 0, 0},
		{"v d 3 chars", "abcdefghij", 0, 1, "vlld.", "ahij", 0, 1},
		{"v d clamps at line end", "abcdefg", 0, 4, "vlld.", "abc", 0, 2},
		{"v d across lines same column", "abcdef\nghijkl\nmnopqr\nstuvwx\nyz1234", 0, 1, "vjlld.", "aqr\nstuvwx\nyz1234", 0, 1},
		{"v d across lines short last line", "abcdef\nghijkl\nmn\nstuvwx\nyz1234", 0, 1, "vjllld.", "astuvwx\nyz1234", 0, 1},
		{"v d 3 lines", "abcdef\nghijkl\nmnopqr\nstuvwx\nyz1234\n567890", 0, 2, "vjjld.", "ab34\n567890", 0, 2},
		{"v d dollar", "abc def\nghi jkl\nmno", 0, 0, "v$d.", "mno", 0, 0},
		{"v d dollar two lines", "ab\ncd\nef\ngh\nij", 0, 1, "vj$d.", "aij", 0, 1},
		{"v d count dot", "abcdefghij", 0, 0, "vld3.", "efghij", 0, 0},
		{"v x", "abcdefgh", 0, 0, "vlx.", "efgh", 0, 0},
		{"v c", "abcdefgh", 0, 0, "vlcX<esc>l.", "XXefgh", 0, 1},
		{"v c multi line", "abc\ndef\nghi\njkl\nmno", 0, 0, "vjcX<esc>j.", "Xef\nXkl\nmno", 1, 0},
		{"v c word typed", "aa bb cc dd", 0, 0, "vecfoo<esc>w.", "foo foo cc dd", 0, 6},
		{"v y not a change", "abcd efgh", 0, 0, "xvly$.", "bcd efg", 0, 6},
		{"v y then d dot", "abcd efgh ij", 0, 0, "vlyvld.", " efgh ij", 0, 0},
		{"v >", "a\nb\nc\nd", 0, 0, "vj>jj.", "    a\n    b\n    c\n    d", 2, 4},
		{"v 2>", "a\nb", 0, 0, "v2>j.", "        a\n        b", 1, 8},
		{"v <", "        a\n        b\n        c\n        d", 0, 0, "vj<jj.", "    a\n    b\n    c\n    d", 2, 4},
		{"v ~", "abcdef", 0, 0, "vl~l.", "AbCdef", 0, 1},
		{"v u", "ABCDEF", 0, 0, "vlull.", "abcdEF", 0, 2},
		{"v U", "abcdef", 0, 0, "vlUll.", "ABCDef", 0, 2},
		{"v gu", "ABCDEF", 0, 0, "vlgull.", "abcdEF", 0, 2},
		{"v J", "a\nb\nc\nd\ne\nf", 0, 0, "vjJj.", "a b\nc d\ne\nf", 1, 1},
		{"v J three", "a\nb\nc\nd\ne\nf", 0, 0, "vjjJj.", "a b c\nd e f", 1, 3},
		{"v r", "abcdefgh", 0, 0, "vlrxll.", "xxxxefgh", 0, 2},
		{"v r multi line", "abc\ndef\nghi\njkl", 0, 0, "vjrxjj.", "xxx\nxef\nxxx\nxkl", 2, 0},
		{"v p", "ab cd ef gh", 0, 0, "yiwwviwpw.", "ab ab cd gh", 0, 7},
		{"v p chars", "abcdef xy", 0, 7, "ylvhp0.", " xcdefxy", 0, 1},
		{"V d", "a\nb\nc\nd\ne\nf", 0, 0, "Vjd.", "e\nf", 0, 0},
		{"V d clamps", "a\nb\nc", 0, 0, "Vjd.", "", 0, 0},
		{"V d near end", "a\nb\nc\nd", 0, 0, "jjVjd.", "a", 0, 0},
		{"V d three", "a\nb\nc\nd\ne\nf\ng", 0, 0, "Vjjd.", "g", 0, 0},
		{"V c", "a\nb\nc\nd\ne\nf", 0, 0, "VjcX<esc>j.", "X\nX\ne\nf", 1, 0},
		{"V >", "a\nb\nc\nd", 0, 0, "Vj>jj.", "    a\n    b\n    c\n    d", 2, 4},
		{"V <", "    a\n    b\n    c\n    d", 0, 0, "Vj<jj.", "a\nb\nc\nd", 2, 0},
		{"V U", "ab\ncd\nef\ngh", 0, 0, "VjUjj.", "AB\nCD\nEF\nGH", 2, 0},
		{"V ~", "ab\ncd\nef\ngh", 0, 0, "Vj~jj.", "AB\nCD\nEF\nGH", 2, 0},
		{"V u", "AB\nCD\nEF\nGH", 0, 0, "Vjujj.", "ab\ncd\nef\ngh", 2, 0},
		{"V J", "a\nb\nc\nd\ne", 0, 0, "VjJj.", "a b\nc d\ne", 1, 1},
		{"V r", "abc\ndef\nghi\njkl", 0, 1, "Vjrxjj.", "xxx\nxxx\nxxx\nxxx", 2, 0},
		{"V p", "a\nb\nc\nd\ne", 0, 0, "yyjVpjj.", "a\na\nc\nb\ne", 3, 0},
		{"V y no repeat", "a\nb\nc", 0, 0, "Vyjx.", "a\n\nc", 1, 0},
		{"v with register", "abcdef", 0, 0, "vl\"ad.", "ef", 0, 0},
		{"X in v", "a\nb\nc\nd\ne\nf", 0, 0, "vjX.", "e\nf", 0, 0},
		{"dot twice", "abcdefghij", 0, 0, "vld..", "ghij", 0, 0},
		// Display columns: a tab goes to the next multiple of 8, a wide rune takes 2 columns.
		{"v d tab span", "a\tb\tc\td\te\tf", 0, 0, "vld.", "c\td\te\tf", 0, 0},
		{"v d tab span from tab", "a\tb\tc\td\te\tf", 0, 1, "vld.", "ad\te\tf", 0, 1},
		{"v d tab two chars", "a\tb\tc\td\te\tf", 0, 0, "vlld.", "d\te\tf", 0, 0},
		{"v d tab end of selection", "ab\tcdef\tghijklmnopq", 0, 1, "vlld.", "ahijklmnopq", 0, 1},
		{"v d tab dot after tab", "ab\tcdef\tghijklmnopq", 0, 3, "vlld.", "ab\tghijklmnopq", 0, 3},
		{"v d tab dot mid line", "abcdefghijkl\tmnopqrst", 0, 0, "vllllllld.", "mnopqrst", 0, 0},
		{"v c tab", "a\tb\tc\td\te\tf\tg\th", 0, 0, "vlcX<esc>.", "Xc\td\te\tf\tg\th", 0, 0},
		{"v c tab three", "a\tb\tc\td\te\tf\tg\th", 0, 0, "vlllcX<esc>.", "Xe\tf\tg\th", 0, 0},
		{"v d tab leading", "\tab\tcd\tefghijkl", 0, 0, "vld.", "d\tefghijkl", 0, 0},
		{"v d tab leading indent", "        ab\tc\t\t\td", 0, 0, "vlld.", "  ab\tc\t\t\td", 0, 0},
		{"v d wide", "中文字符abc中文字符defghijkl", 0, 0, "vld.", "abc中文字符defghijkl", 0, 0},
		{"v d wide from second", "中文字符abc中文字符defghijkl", 0, 1, "vld.", "中c中文字符defghijkl", 0, 1},
		{"v d wide single", "中文字符abc中文字符defghijkl", 0, 1, "vd.", "中符abc中文字符defghijkl", 0, 1},
		{"v d wide ascii span", "中文字符abc中文字符defghijkl", 0, 0, "vlllld.", "defghijkl", 0, 0},
		{"v d wide mixed", "a中b文c字d中中中中efgh", 0, 0, "vlld.", "d中中中中efgh", 0, 0},
		{"v d wide mixed from wide", "a中b文c字d中中中中efgh", 0, 1, "vlld.", "a中中中efgh", 0, 1},
		{"v d wide tab mixed", "中a\tb中c\td中e\tfghijklmnop", 0, 0, "vlllld.", "e\tfghijklmnop", 0, 0},
		{"v x wide", "中文字符abc中文字符defghijkl", 0, 0, "vlx.", "abc中文字符defghijkl", 0, 0},
		{"v c wide", "中文字符abc中文字符defghijkl", 0, 1, "vlcX<esc>.", "中Xbc中文字符defghijkl", 0, 1},
		{"v d across lines tab last line", "abcdefghijkl\nxyz\tw\tq\tmmmmmmm\nabcdefghijkl\nmn中op", 0, 0, "vjllld.", "ijkl\nmn中op", 0, 0},
		{"v d across lines tab end col", "abcdefghijkl\nxyz\tw\tq\tmmmmmmm\nabcdefghijkl\nmn中op", 0, 0, "vjllllld.", "mn中op", 0, 0},
		{"v d across lines tab first col", "abcdefghijkl\nxyz\tw\tq\tmmmmmmm\nabcdefghijkl\nmn中op", 0, 0, "vjlllld.", "jkl\nmn中op", 0, 0},
		{"v d across lines wide", "abcdefghijkl\n中a中b中c中d中e中f\nabcdefghijklmnopq\nmn中op", 0, 0, "vjlld.", "fghijklmnopq\nmn中op", 0, 0},
		{"v d across lines wide odd", "abcdefghijkl\n中a中b中c中d中e中f\nabcdefghijklmnopq\nmn中op", 0, 0, "vjllld.", "ghijklmnopq\nmn中op", 0, 0},
		{"v d across lines wide end", "abcdefghijkl\n中a中b中c中d中e中f\nabcdefghijklmnopq\nmn中op", 0, 0, "vjllllld.", "jklmnopq\nmn中op", 0, 0},
		{"v d across lines tab line two", "abcdefghijklmnop\nab\tcd\tef\tgh\nabcdefghijklmnop\nxyz", 0, 0, "vjllllld.", "\nxyz", 0, 0},
		{"v d across lines tab line two b", "abcdefghijklmnop\nab\tcd\tef\tgh\nabcdefghijklmnop\nxyz", 0, 0, "vjlllllld.", "xyz", 0, 0},
		{"v d across lines tab line two c", "abcdefghijklmnop\n\tab\tcd\tef\tgh\nabcdefghijklmnopqrstuvwxyz\nxyz", 0, 0, "vjlld.", "klmnopqrstuvwxyz\nxyz", 0, 0},
		{"v d across lines tab line two d", "abcdefghijklmnop\n\tab\tcd\tef\tgh\nabcdefghijklmnopqrstuvwxyz\nxyz", 0, 0, "vjllld.", "qrstuvwxyz\nxyz", 0, 0},
	})
}

// TestVisualPut checks p and P in visual and visual line mode.
func TestVisualPut(t *testing.T) {
	runTextCases(t, []textCase{
		{"charwise by charwise", "foo bar", 0, 0, "yiwwviwp", "foo foo", 0, 6},
		{"charwise by charwise P", "foo bar", 0, 0, "yiwwviwP", "foo foo", 0, 6},
		{"charwise at line end", "foo bar", 0, 0, "yiw$viwp", "foo foo", 0, 6},
		{"charwise at line end P", "foo bar", 0, 0, "yiw$viwP", "foo foo", 0, 6},
		{"unnamed after p", "foo bar baz", 0, 0, "yiwwviwp$p", "foo foo bazbar", 0, 13},
		{"unnamed after P", "foo bar baz", 0, 0, "yiwwviwP$p", "foo foo bazfoo", 0, 13},
		{"P twice", "foo bar baz", 0, 0, "yiwwviwPwviwP", "foo foo foo", 0, 10},
		{"p twice swaps", "foo bar baz", 0, 0, "yiwwviwpwviwp", "foo foo bar", 0, 10},
		{"charwise multi line sel", "ab cd\nef gh\nij", 0, 0, "ywjvlp", "ab cd\nab  gh\nij", 1, 2},
		{"multi line reg into char", "ab\ncd\nef gh", 0, 0, "vjyjjvlp", "ab\ncd\nab\nc gh", 2, 0},
		{"multi line sel by char reg", "ab\ncd\nef gh", 0, 0, "ylvjp", "ad\nef gh", 0, 0},
		{"count", "ab cd", 0, 0, "yiwwviw3p", "ab ababab", 0, 8},
		{"empty register fails", "ab cd", 0, 0, "vlp", " cd", 0, 0},
		{"named register", "ab cd ef", 0, 0, "\"ayiwwyiwwviw\"ap", "ab cd ab", 0, 7},
		{"linewise by charwise", "one\ntwo three\nfour", 0, 0, "yyjwviwp", "one\ntwo \none\n\nfour", 2, 0},
		{"linewise by charwise start", "one\ntwo three\nfour", 0, 0, "yyjviwp", "one\n\none\n three\nfour", 2, 0},
		{"linewise by charwise whole line", "one\ntwo\nfour", 0, 0, "yyjviwp", "one\n\none\n\nfour", 2, 0},
		{"linewise by charwise P", "one\ntwo three\nfour", 0, 0, "yyjwviwP", "one\ntwo \none\n\nfour", 2, 0},
		{"linewise by charwise multi", "one\ntwo\nthree\nfour", 0, 0, "yjjjvp", "one\ntwo\n\none\ntwo\nhree\nfour", 3, 0},
		{"charwise by linewise", "one\ntwo\nthree", 0, 0, "yiwjVp", "one\none\nthree", 1, 0},
		{"charwise by linewise last", "one\ntwo\nthree", 0, 0, "yiwGVp", "one\ntwo\none", 2, 0},
		{"charwise by linewise multi", "one\ntwo\nthree\nfour", 0, 0, "yiwjVjp", "one\none\nfour", 1, 0},
		{"charwise by linewise whole", "one\ntwo", 0, 0, "yiwVjp", "one", 0, 0},
		{"charwise by linewise whole G", "one\ntwo", 0, 0, "yiwVGp", "one", 0, 0},
		{"linewise by linewise", "one\ntwo\nthree", 0, 0, "yyjVp", "one\none\nthree", 1, 0},
		{"linewise by linewise last", "one\ntwo\nthree", 0, 0, "yyGVp", "one\ntwo\none", 2, 0},
		{"linewise by linewise two", "one\ntwo\nthree\nfour", 0, 0, "yyjVjp", "one\none\nfour", 1, 0},
		{"linewise by linewise whole", "one\ntwo\nthree", 0, 0, "yyVGp", "one", 0, 0},
		{"linewise by linewise last two", "one\ntwo\nthree\nfour", 0, 0, "yyjjVjp", "one\ntwo\none", 2, 0},
		{"linewise by linewise P", "one\ntwo\nthree", 0, 0, "yyjVP", "one\none\nthree", 1, 0},
		{"linewise count", "one\ntwo\nthree", 0, 0, "yyjV2p", "one\none\none\nthree", 1, 0},
		{"linewise reg two lines", "a\nb\nc\nd", 0, 0, "yjjjVp", "a\nb\na\nb\nd", 2, 0},
		{"linewise reg two lines mid", "a\nb\nc\nd\ne", 0, 0, "yjjjVp", "a\nb\na\nb\nd\ne", 2, 0},
		{"unnamed after V p", "one\ntwo\nthree", 0, 0, "yyjVpGp", "one\none\nthree\ntwo", 3, 0},
		{"unnamed after V P", "one\ntwo\nthree", 0, 0, "yyjVPGp", "one\none\nthree\none", 3, 0},
		{"small delete register", "ab cd", 0, 0, "yiwwviwp\"-p", "ab abcd", 0, 6},
		{"numbered register after multi", "ab\ncd\nef", 0, 0, "yiwjvjp\"1p", "ab\nabcd\nef", 1, 2},
		{"V charwise reg multi lines", "a\nb\nc\nd", 0, 0, "vjyGVp", "a\nb\nc\na\nb", 3, 0},
		{"v dollar", "foo bar\nbaz", 0, 0, "yiwwv$p", "foo foobaz", 0, 6},
		{"single char", "abc", 0, 0, "ylvlp", "ac", 0, 0},
		{"v p at end of line cursor", "abc def", 0, 0, "yiw$vp", "abc deabc", 0, 8},
	})
}

// TestVisualReplace checks r in visual modes.
func TestVisualReplace(t *testing.T) {
	runTextCases(t, []textCase{
		{"v l", "abcdef", 0, 1, "vlrx", "axxdef", 0, 1},
		{"v one", "abc", 0, 1, "vrx", "axc", 0, 1},
		{"v back", "abcdef", 0, 3, "vhhrx", "axxxef", 0, 1},
		{"v multi line", "abc\ndef\nghi", 0, 1, "vjrx", "axx\nxxf\nghi", 0, 1},
		{"v dollar", "abc\ndef", 0, 1, "v$rx", "axx\ndef", 0, 1},
		{"v multi dollar", "abc\ndef\nghi", 0, 1, "vj$rx", "axx\nxxx\nghi", 0, 1},
		{"V", "abc\ndef\nghi", 0, 1, "Vjrx", "xxx\nxxx\nghi", 0, 0},
		{"V single", "abc\ndef", 1, 2, "Vrx", "abc\nxxx", 1, 0},
		{"V empty line", "abc\n\nghi", 0, 0, "Vjjrx", "xxx\n\nxxx", 0, 0},
		{"v empty line", "abc\n\nghi", 0, 1, "vjjrx", "axx\n\nxxi", 0, 1},
		{"v space", "abc def", 0, 0, "veer ", "       ", 0, 0},
		{"v iw", "foo bar", 0, 0, "viwrx", "xxx bar", 0, 0},
	})
}

// TestSentenceObjects checks is and as, with counts and over line breaks.
func TestSentenceObjects(t *testing.T) {
	runTextCases(t, []textCase{
		{"is first", "One. Two. Three.", 0, 1, "dis", " Two. Three.", 0, 0},
		{"is middle", "One. Two. Three.", 0, 6, "dis", "One.  Three.", 0, 5},
		{"is last", "One. Two. Three.", 0, 12, "dis", "One. Two. ", 0, 9},
		{"as first", "One. Two. Three.", 0, 1, "das", "Two. Three.", 0, 0},
		{"as middle", "One. Two. Three.", 0, 6, "das", "One. Three.", 0, 5},
		{"as last", "One. Two. Three.", 0, 12, "das", "One. Two.", 0, 8},
		{"is on white", "One.  Two.", 0, 4, "dis", "One.Two.", 0, 4},
		{"as on white", "One.  Two. Three.", 0, 4, "das", "One. Three.", 0, 4},
		{"2is", "One. Two. Three.", 0, 1, "d2is", "Two. Three.", 0, 0},
		{"3is", "One. Two. Three. Four.", 0, 1, "d3is", " Three. Four.", 0, 0},
		{"2as", "One. Two. Three.", 0, 1, "d2as", "Three.", 0, 0},
		{"3as", "One. Two. Three. Four.", 0, 1, "d3as", "Four.", 0, 0},
		{"2as last", "One. Two. Three.", 0, 6, "d2as", "One.", 0, 3},
		{"5as too many", "One. Two. Three.", 0, 6, "d5as", "One.", 0, 3},
		{"question and bang", "Is it? Yes! Ok.", 0, 8, "das", "Is it? Ok.", 0, 7},
		{"no space after dot", "a.b c. d", 0, 0, "das", "d", 0, 0},
		{"closing paren", "(One.) Two.", 0, 1, "das", "Two.", 0, 0},
		{"closing quote", "He said \"hi.\" Then left.", 0, 3, "das", "Then left.", 0, 0},
		{"closing quote 2", "He said \"hi.\" Then left.", 0, 18, "das", "He said \"hi.\"", 0, 12},
		{"closing bracket", "[One.] Two.", 0, 8, "dis", "[One.] ", 0, 6},
		{"multi line sentence", "One two\nthree. Four five\nsix.", 0, 1, "das", "Four five\nsix.", 0, 0},
		{"multi line second", "One two\nthree. Four five\nsix.", 1, 9, "das", "One two\nthree.", 1, 5},
		{"multi line is", "One two\nthree. Four five\nsix.", 0, 1, "dis", " Four five\nsix.", 0, 0},
		{"multi line is second", "One two\nthree. Four five\nsix.", 2, 1, "dis", "One two\nthree. ", 1, 6},
		{"end of line sentence", "One.\nTwo.\nThree.", 1, 1, "das", "One.\nThree.", 1, 0},
		{"end of line sentence is", "One.\nTwo.\nThree.", 1, 1, "dis", "One.\nThree.", 1, 0},
		{"end of line first", "One.\nTwo.\nThree.", 0, 1, "das", "Two.\nThree.", 0, 0},
		{"end of line last", "One.\nTwo.\nThree.", 2, 1, "das", "One.\nTwo.\n", 2, 0},
		{"paragraph ends sentence", "One two\n\nThree four", 0, 1, "das", "\nThree four", 0, 0},
		{"paragraph ends sentence is", "One two\n\nThree four", 0, 1, "dis", "\nThree four", 0, 0},
		{"paragraph second", "One two\n\nThree four", 2, 1, "das", "One two\n\n", 2, 0},
		{"paragraph second is", "One two\n\nThree four", 2, 1, "dis", "One two\n\n", 2, 0},
		{"on empty line is", "One.\n\nTwo.", 1, 0, "dis", "One.\nTwo.", 1, 0},
		{"on empty line as", "One.\n\nTwo.", 1, 0, "das", "One.\nTwo.", 1, 0},
		{"2is across lines", "One.\nTwo.\nThree.", 0, 1, "d2is", "Two.\nThree.", 0, 0},
		{"3is across lines", "One.\nTwo.\nThree.", 0, 1, "d3is", "Three.", 0, 0},
		{"2as across lines", "One.\nTwo.\nThree.", 0, 1, "d2as", "Three.", 0, 0},
		{"2as across paragraph", "One.\n\nTwo.", 0, 1, "d2as", "Two.", 0, 0},
		{"indent next line", "One.\n  Two.\n  Three.", 0, 1, "das", "Two.\n  Three.", 0, 0},
		{"indent is", "One.\n  Two.\n  Three.", 1, 4, "dis", "One.\n  Three.", 1, 2},
		{"trailing spaces at line end", "One.   \nTwo.", 0, 1, "das", "Two.", 0, 0},
		{"trailing spaces para", "One.   \n\nTwo.", 0, 1, "das", "\nTwo.", 0, 0},
		{"change as", "One. Two. Three.", 0, 6, "casX<esc>", "One. XThree.", 0, 5},
		{"change is", "One. Two. Three.", 0, 6, "cisX<esc>", "One. X Three.", 0, 5},
		{"yank is", "One. Two. Three.", 0, 6, "yis$p", "One. Two. Three.Two.", 0, 19},
		{"one sentence only", "Just one", 0, 3, "das", "", 0, 0},
		{"one sentence only is", "Just one", 0, 3, "dis", "", 0, 0},
		{"ellipsis", "Wait... what? Yes.", 0, 2, "das", "what? Yes.", 0, 0},
		{"abbrev", "Mr. Smith left. Bye.", 0, 8, "das", "Mr. Bye.", 0, 4},
		{"visual is", "One. Two. Three.", 0, 6, "visd", "One.  Three.", 0, 5},
		{"visual as", "One. Two. Three.", 0, 6, "vasd", "One. Three.", 0, 5},
		{"visual as multi", "One.\nTwo.\nThree.", 1, 1, "vasd", "One.\n\nThree.", 1, 0},
		{"leading blank lines", "\n\nOne.", 2, 0, "das", "\n\n", 2, 0},
		{"mid line sentence end of line", "x. One.\nTwo.\nc", 0, 4, "dis", "x. \nTwo.\nc", 0, 2},
		{"mid line sentence end of line as", "x. One.\nTwo.\nc", 0, 4, "das", "x.\nTwo.\nc", 0, 1},
		{"start of line sentence mid end", "One. Two.\nThree.", 0, 7, "dis", "One. \nThree.", 0, 4},
		{"3is on blank lines", "ab.\n\n\ncd", 0, 0, "d3is", "cd", 0, 0},
		{"is after paragraph", "A.\n\n  B. C.", 2, 3, "dis", "A.\n\n   C.", 2, 2},
		{"as at second sentence line", "A b.\nC d. E.", 1, 1, "das", "A b.\nE.", 1, 0},
		{"2as over line end", "A b.\nC d. E.", 0, 1, "d2as", "E.", 0, 0},
		{"yank as paste", "One. Two.", 0, 6, "yas$p", "One. Two. Two.", 0, 13},
		{"is first of two empty lines", "a.\n\n\nb.", 1, 0, "dis", "a.\n\nb.", 2, 0},
		{"is second of two empty lines", "a.\n\n\nb.", 2, 0, "dis", "a.\n\nb.", 2, 0},
		{"as first of two empty lines", "a.\n\n\nb.", 1, 0, "das", "a.\n\nb.", 2, 0},
		{"2is first of two empty lines", "a.\n\n\nb.", 1, 0, "d2is", "a.\n\nb.", 2, 0},
		{"3is first of three empty lines", "a.\n\n\n\nb.", 1, 0, "d3is", "a.\n\n", 2, 0},
		{"is first of three empty lines", "a.\n\n\n\nb.", 1, 0, "dis", "a.\n\n\nb.", 3, 0},
		{"is middle of three empty lines", "a.\n\n\n\nb.", 2, 0, "dis", "a.\n\n\nb.", 3, 0},
		{"is last of three empty lines", "a.\n\n\n\nb.", 3, 0, "dis", "a.\n\n\nb.", 3, 0},
		{"visual is first of two empty lines", "a.\n\n\nb.", 1, 0, "visd", "a.\nb.", 1, 0},
		{"visual is first of three empty lines", "a.\n\n\n\nb.", 1, 0, "visd", "a\n\nb.", 0, 0},
		{"visual as first of two empty lines", "a.\n\n\nb.", 1, 0, "vasd", "a.\nb.", 1, 0},
		{"is first of two empty lines at start", "\n\n\nb.", 0, 0, "dis", "\n\nb.", 2, 0},
		{"is second of two empty lines at start", "\n\n\nb.", 1, 0, "dis", "\n\nb.", 2, 0},
		{"visual is empty lines at start", "\n\n\nb.", 0, 0, "visd", "b.", 0, 0},
		{"2is empty lines at start", "\n\n\nb.", 0, 0, "d2is", "\n\nb.", 2, 0},
		{"is empty lines at end", "a.\n\n\n", 1, 0, "dis", "\n\n", 0, 0},
		{"is two empty lines then text", "a b.\n\n\nc d.", 1, 0, "dis", "a b.\n\nc d.", 2, 0},
		{"is two empty lines then indent", "a.\n\n\n  b.", 1, 0, "dis", "a.\n\n  b.", 2, 2},
		{"as two empty lines then indent", "a.\n\n\n  b.", 1, 0, "das", "a.\n\nb.", 2, 0},
		{"is indent after empty lines", "a.\n\n\n  b.", 3, 0, "dis", "a.\n\n\n  ", 3, 1},
		{"is at text start with indent", "  a. b.", 0, 0, "dis", " b.", 0, 0},
		{"is on indent at text start", "  a. b.", 0, 1, "dis", " b.", 0, 0},
		{"as at text start with indent", "  a. b.", 0, 0, "das", "b.", 0, 0},
		{"as at text start with indent in sentence", "  a. b.", 0, 3, "das", "b.", 0, 0},
		{"is at text start two lines", "  a.\n  b.", 0, 0, "dis", "  b.", 0, 2},
		{"as at text start two lines", "  a.\n  b.", 0, 0, "das", "b.", 0, 0},
		{"visual is at text start with indent", "  a. b.", 0, 0, "visd", " b.", 0, 0},
		{"visual as at text start with indent", "  a. b.", 0, 2, "vasd", "b.", 0, 0},
		{"is after one empty line at start", "\n  a.", 0, 0, "dis", "  a.", 0, 2},
		{"as after one empty line at start", "\n  a.", 0, 0, "das", "a.", 0, 0},
		{"as after two empty lines at start", "\n\n  a. b.", 1, 0, "das", "\na. b.", 1, 0},
		{"is on indent after empty line at start", "\n\n  a. b.", 2, 0, "dis", "\n\n   b.", 2, 2},
		{"2is indent at start", "  a. b.", 0, 0, "d2is", "b.", 0, 0},
		{"3is indent at start", "  a. b.", 0, 0, "d3is", "", 0, 0},
	})
}

// TestMarksMove checks that marks follow added and removed lines. want is the line of
// mark a after the keys, or -1 if the mark is gone.
func TestMarksMove(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		row, col int
		script   string
		want     string
		mark     int
	}{
		{"insert above moves down", "a\nb\nc", 1, 0, "maggOx<esc>", "x\na\nb\nc", 2},
		{"insert below no move", "a\nb\nc", 1, 0, "majox<esc>", "a\nb\nc\nx", 1},
		{"p linewise above", "a\nb\nc", 2, 0, "maggyyP", "a\na\nb\nc", 3},
		{"p linewise below", "a\nb\nc", 2, 0, "maggyyp", "a\na\nb\nc", 3},
		{"delete above moves up", "a\nb\nc\nd", 2, 0, "maggdd", "b\nc\nd", 1},
		{"delete marked line", "a\nb\nc", 1, 0, "madd", "a\nc", -1},
		{"delete range before mark", "a\nb\nc\nd", 2, 0, "magg2dd", "c\nd", 0},
		{"delete range includes mark", "a\nb\nc\nd", 1, 0, "magg3dd", "d", -1},
		{"delete below mark", "a\nb\nc\nd", 1, 0, "majdd", "a\nb\nd", 1},
		{"dj", "a\nb\nc\nd", 2, 0, "maggdj", "c\nd", 0},
		{"dG", "a\nb\nc\nd", 2, 0, "maggjdG", "a", -1},
		{"enter in insert", "ab\nc\nd", 1, 0, "maggli<cr><esc>", "a\nb\nc\nd", 2},
		{"split marked line", "ab\nc", 0, 1, "mai<cr><esc>", "a\nb\nc", 0},
		{"J joins marked", "a\nb\nc", 1, 0, "maggJ", "a b\nc", 0},
		{"J below mark", "a\nb\nc\nd", 3, 0, "maggJ", "a b\nc\nd", 2},
		{"3J", "a\nb\nc\nd", 2, 0, "maggJ", "a b\nc\nd", 1},
		{"charwise multi delete mark inside", "ab\ncd\nef", 1, 0, "magglv2jd", "a", -1},
		{"charwise multi delete mark end line", "ab\ncd\nef\ngh", 2, 0, "magglvjjd", "a\ngh", 0},
		{"charwise multi delete mark below", "ab\ncd\nef\ngh", 3, 0, "magglvjd", "a\nef\ngh", 2},
		{"visual p linewise above", "a\nb\nc\nd", 3, 0, "maggyyjVp", "a\na\nc\nd", 3},
		{"visual p replaces lines", "a\nb\nc\nd", 3, 0, "maggyyjVjp", "a\na\nd", 2},
		{"visual p replaces marked line", "a\nb\nc\nd", 1, 0, "maggyyjVp", "a\na\nc\nd", -1},
		{"visual p charwise multi", "a b\nc\nd", 2, 0, "maggyiwjvjp", "a b\na", 1},
		{"visual d lines", "a\nb\nc\nd", 3, 0, "maggVjd", "c\nd", 1},
		{"o with count", "a\nb\nc", 2, 0, "maggo<esc>", "a\n\nb\nc", 3},
		{"two marks", "a\nb\nc\nd", 1, 0, "majmbggOx<esc>", "x\na\nb\nc\nd", 2},
		{"dot repeat shifts", "a\nb\nc\nd", 3, 0, "maggddj.", "b\nd", 1},
		{"yank put twice", "a\nb\nc", 2, 0, "maggyy2P", "a\na\na\nb\nc", 4},
		{"cc keeps mark", "a\nb\nc", 1, 0, "maccx<esc>", "a\nx\nc", 1},
		{"c lines range", "a\nb\nc\nd", 3, 0, "maggc2jx<esc>", "x\nd", 1},
		{"D no line change", "ab\nc", 1, 0, "maggD", "\nc", 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := buffer.New(tc.text)
			b.Row, b.Col = tc.row, tc.col
			v := New(4)
			run(v, b, tc.script)
			if got := b.Text(); got != tc.want {
				t.Errorf("text = %q, want %q", got, tc.want)
			}
			got := -1
			if p, ok := v.marks['a']; ok {
				got = p.row
			}
			if got != tc.mark {
				t.Errorf("mark a on line %d, want %d", got, tc.mark)
			}
		})
	}
}

// TestMarksUndo checks that undo and redo move the marks as vim does. Marks a and b are the
// lines after the keys, or -1 if the mark is gone. Vim also brings back the marks that a
// change deleted or moved, when undo or redo goes over that change.
func TestMarksUndo(t *testing.T) {
	tests := []struct {
		name, text, script, want string
		a, b                     int
	}{
		{"undo dd above mark", "a\nb\nc\nd\ne", "jjmaggddu", "a\nb\nc\nd\ne", 2, -1},
		{"redo dd above mark", "a\nb\nc\nd\ne", "jjmaggddu<c-r>", "b\nc\nd\ne", 1, -1},
		{"undo 2dd above mark", "a\nb\nc\nd\ne", "jjmagg2ddu", "a\nb\nc\nd\ne", 2, -1},
		{"redo 2dd above mark", "a\nb\nc\nd\ne", "jjmagg2ddu<c-r>", "c\nd\ne", 0, -1},
		{"undo O above mark", "a\nb\nc\nd\ne", "jjmaggOx<esc>u", "a\nb\nc\nd\ne", 2, -1},
		{"redo O above mark", "a\nb\nc\nd\ne", "jjmaggOx<esc>u<c-r>", "x\na\nb\nc\nd\ne", 3, -1},
		{"O on marked line", "a\nb\nc\nd\ne", "jjmaOx<esc>", "a\nb\nx\nc\nd\ne", 3, -1},
		{"undo O on marked line", "a\nb\nc\nd\ne", "jjmaOx<esc>u", "a\nb\nc\nd\ne", 2, -1},
		{"redo O on marked line", "a\nb\nc\nd\ne", "jjmaOx<esc>u<c-r>", "a\nb\nx\nc\nd\ne", 3, -1},
		{"o below mark", "a\nb\nc\nd\ne", "jjmaox<esc>", "a\nb\nc\nx\nd\ne", 2, -1},
		{"undo o below mark", "a\nb\nc\nd\ne", "jjmaox<esc>u", "a\nb\nc\nd\ne", 2, -1},
		{"undo dd of marked line", "a\nb\nc\nd\ne", "jmaddu", "a\nb\nc\nd\ne", 1, -1},
		{"redo dd of marked line", "a\nb\nc\nd\ne", "jmaddu<c-r>", "a\nc\nd\ne", -1, -1},
		{"undo dd below mark", "a\nb\nc\nd\ne", "jmajddu", "a\nb\nc\nd\ne", 1, -1},
		{"redo dd below mark", "a\nb\nc\nd\ne", "jmajddu<c-r>", "a\nb\nd\ne", 1, -1},
		{"undo J above mark", "a\nb\nc\nd\ne", "jjmaggJu", "a\nb\nc\nd\ne", 2, -1},
		{"redo J above mark", "a\nb\nc\nd\ne", "jjmaggJu<c-r>", "a b\nc\nd\ne", 1, -1},
		{"undo 3J over mark", "a\nb\nc\nd\ne", "jjmagg3Ju", "a\nb\nc\nd\ne", 2, -1},
		{"redo 3J over mark", "a\nb\nc\nd\ne", "jjmagg3Ju<c-r>", "a b c\nd\ne", 0, -1},
		{"undo p above mark", "a\nb\nc\nd\ne", "jjmaggyyPu", "a\nb\nc\nd\ne", 2, -1},
		{"redo p above mark", "a\nb\nc\nd\ne", "jjmaggyyPu<c-r>", "a\na\nb\nc\nd\ne", 3, -1},
		{"undo Vjd over mark", "a\nb\nc\nd\ne", "jjmaggVjdu", "a\nb\nc\nd\ne", 2, -1},
		{"redo Vjd over mark", "a\nb\nc\nd\ne", "jjmaggVjdu<c-r>", "c\nd\ne", 0, -1},
		{"undo dG over mark", "a\nb\nc\nd\ne", "jjmaggdGu", "a\nb\nc\nd\ne", 2, -1},
		{"undo x keeps mark", "a\nb\nc\nd\ne", "jjmaggxu", "a\nb\nc\nd\ne", 2, -1},
		{"undo O below mark keeps mark", "a\nb\nc\nd\ne", "jjmaGOx<esc>u", "a\nb\nc\nd\ne", 2, -1},
		{"undo O deletes mark on new line", "a\nb\nc\nd\ne", "jjmaggOx<esc>mbu", "a\nb\nc\nd\ne", 2, -1},
		{"redo O brings back mark on new line", "a\nb\nc\nd\ne", "jjmaggOx<esc>mbu<c-r>", "x\na\nb\nc\nd\ne", 3, 0},
		{"undo dd two marks", "a\nb\nc\nd\ne", "jmajmbggddu", "a\nb\nc\nd\ne", 1, 2},
		{"redo dd two marks", "a\nb\nc\nd\ne", "jmajmbggddu<c-r>", "b\nc\nd\ne", 0, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := buffer.New(tc.text)
			v := New(4)
			run(v, b, tc.script)
			if got := b.Text(); got != tc.want {
				t.Errorf("text = %q, want %q", got, tc.want)
			}
			for name, want := range map[rune]int{'a': tc.a, 'b': tc.b} {
				got := -1
				if p, ok := v.marks[name]; ok {
					got = p.row
				}
				if got != want {
					t.Errorf("mark %c on line %d, want %d", name, got, want)
				}
			}
		})
	}
}

// TestVisualPutClipboard checks that the replaced text goes to the system clipboard for p,
// and that P leaves registers and clipboard alone.
func TestVisualPutClipboard(t *testing.T) {
	b := buffer.New("foo bar")
	v := New(4)
	run(v, b, "yiwwv")
	fx := run(v, b, "ep")
	if c := clipboards(fx); len(c) != 1 || c[0] != "bar" {
		t.Errorf("p clipboard = %q", c)
	}
	if b.Text() != "foo foo" {
		t.Errorf("text = %q", b.Text())
	}
	b = buffer.New("foo bar")
	v = New(4)
	run(v, b, "yiwwv")
	fx = run(v, b, "eP")
	if c := clipboards(fx); len(c) != 0 {
		t.Errorf("P clipboard = %q", c)
	}
	// Linewise: the lines go to the clipboard with a final line break.
	b = buffer.New("a\nb\nc")
	v = New(4)
	run(v, b, "yyj")
	fx = run(v, b, "Vp")
	if c := clipboards(fx); len(c) != 1 || c[0] != "b\n" {
		t.Errorf("V p clipboard = %q", c)
	}
}

// TestVisualPutSystemClipboard checks "+p in visual mode: it asks the terminal, and the
// answer replaces the selection.
func TestVisualPutSystemClipboard(t *testing.T) {
	b := buffer.New("foo bar")
	v := New(4)
	run(v, b, "w")
	fx := run(v, b, "viw\"+p")
	if !keymap.Has(fx, keymap.KindReadClipboard) || v.Mode() != "visual" {
		t.Fatalf("no clipboard read: %v, mode %s", fx, v.Mode())
	}
	fx = v.Paste(b, "XY")
	if b.Text() != "foo XY" || v.Mode() != "normal" || !keymap.Has(fx, keymap.KindChanged) {
		t.Errorf("text %q mode %s", b.Text(), v.Mode())
	}
	if b.UndoDepth() != 1 {
		t.Errorf("undo depth %d", b.UndoDepth())
	}
	run(v, b, "u")
	if b.Text() != "foo bar" {
		t.Errorf("undo: %q", b.Text())
	}
}

// TestMarkHook checks the life of the hooks in the buffer: they are in it after the first
// key, so that an edit of the UI between the keys moves the marks. Reset and a new buffer
// take them out.
func TestMarkHook(t *testing.T) {
	b := buffer.New("a\nb\nc")
	v := New(4)
	if b.OnShift != nil {
		t.Fatal("hook set before the first key")
	}
	run(v, b, "jma")
	if b.OnShift == nil || b.OnSnap == nil || b.OnRestore == nil {
		t.Fatal("hooks not set after Handle")
	}
	b.InsertLines(0, "x") // an edit that the UI makes
	if got := v.marks['a'].row; got != 2 {
		t.Errorf("mark a on line %d after a line insert, want 2", got)
	}
	b.DeleteLines(0, 0)
	if got := v.marks['a'].row; got != 1 {
		t.Errorf("mark a on line %d after a line delete, want 1", got)
	}
	b2 := buffer.New("p\nq")
	run(v, b2, "l")
	if b.OnShift != nil || b.OnSnap != nil || b.OnRestore != nil {
		t.Error("the old buffer keeps the hooks")
	}
	if b2.OnShift == nil {
		t.Error("the new buffer has no hook")
	}
	v.Reset()
	if b2.OnShift != nil || b2.OnSnap != nil || b2.OnRestore != nil {
		t.Error("hooks stay set after Reset")
	}
	v.Paste(b, "z\ny")
	if b.OnShift == nil {
		t.Error("hook not set after Paste")
	}
}

// TestVisualUndo checks that a repeat and a visual change are one undo step each.
func TestVisualUndo(t *testing.T) {
	tests := []struct {
		name, text, script, want string
	}{
		{"dot", "abcdef", "vld.u", "cdef"},
		{"dot twice", "abcdefgh", "vld.u", "cdefgh"},
		{"r", "abcdef", "lvlrxu", "abcdef"},
		{"p", "ab cd", "yiwwviwpu", "ab cd"},
		{"V p", "a\nb\nc", "yyjVpu", "a\nb\nc"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := buffer.New(tc.text)
			run(New(4), b, tc.script)
			if b.Text() != tc.want {
				t.Errorf("text = %q, want %q", b.Text(), tc.want)
			}
		})
	}
}
