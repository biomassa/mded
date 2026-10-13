package markdown

import (
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Chroma's shell lexers give most of a shell line the Text type. The shell lexer below
// retypes command words, flags, and operators, so that a code block shows more colors.
// glamour finds a lexer with lexers.Get, which uses the name and the aliases of the lexer.
// Registering a lexer a second time replaces the first one, so the wrapper always wins.

func init() {
	for _, name := range []string{"Bash", "Bash Session"} {
		l := lexers.Get(name)
		if l == nil {
			continue
		}
		if _, done := l.(*shellLexer); done {
			continue
		}
		lexers.Register(&shellLexer{Lexer: l, session: name == "Bash Session"})
	}
}

// shellLexer runs a Chroma shell lexer and retypes some of its tokens.
// When session is true, the lexer is the console lexer: only the prompt lines change.
type shellLexer struct {
	chroma.Lexer
	session bool
}

// SetRegistry gives the registry to the wrapped lexer. It returns the wrapper.
func (s *shellLexer) SetRegistry(r *chroma.LexerRegistry) chroma.Lexer {
	s.Lexer = s.Lexer.SetRegistry(r)
	return s
}

// SetAnalyser gives the analyser to the wrapped lexer. It returns the wrapper.
func (s *shellLexer) SetAnalyser(f func(string) float32) chroma.Lexer {
	s.Lexer = s.Lexer.SetAnalyser(f)
	return s
}

// Tokenise runs the wrapped lexer and retypes its tokens.
func (s *shellLexer) Tokenise(o *chroma.TokeniseOptions, text string) (chroma.Iterator, error) {
	it, err := s.Lexer.Tokenise(o, text)
	if err != nil {
		return nil, err
	}
	return chroma.Literator(retypeShell(it.Tokens(), s.session)...), nil
}

var (
	// shellWrappers are commands that run the next command.
	shellWrappers = map[string]bool{"sudo": true, "doas": true, "env": true, "time": true,
		"exec": true, "xargs": true, "nohup": true}
	// shellStarters are keywords and marks that the next word is a command after.
	shellStarters = map[string]bool{"then": true, "do": true, "else": true, "elif": true, "if": true,
		"while": true, "until": true, "$(": true, "(": true, "!": true}
	// shellSeparators end a command. A new command starts after them.
	shellSeparators = map[string]bool{"|": true, "||": true, "&&": true, ";": true, "&": true, "|&": true}
	shellRedirect   = regexp.MustCompile(`^(\d*>>?|\d*<|&>>?|>&|<&)$`)
	shellAssign     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	shellNumber     = regexp.MustCompile(`^-\d+$`)
)

// shellPiece is a part of a token. Its type can change.
type shellPiece struct {
	typ chroma.TokenType
	val string
}

// shellSplit cuts Text tokens into words and white space. It drops empty tokens.
// The values of all pieces together are the values of all tokens together.
func shellSplit(toks []chroma.Token) []shellPiece {
	var out []shellPiece
	for _, t := range toks {
		if t.Value == "" {
			continue
		}
		if t.Type != chroma.Text {
			out = append(out, shellPiece{t.Type, t.Value})
			continue
		}
		v := t.Value
		for v != "" {
			n := 1
			space := v[0] == ' ' || v[0] == '\t' || v[0] == '\n' || v[0] == '\r'
			for n < len(v) && (v[n] == ' ' || v[n] == '\t' || v[n] == '\n' || v[n] == '\r') == space {
				n++
			}
			out = append(out, shellPiece{chroma.Text, v[:n]})
			v = v[n:]
		}
	}
	return out
}

// retypeShell changes the types of command words, flags, and operators.
// The values do not change. In a session, only the commands after a prompt change.
func retypeShell(toks []chroma.Token, session bool) []chroma.Token {
	ps := shellSplit(toks)
	var (
		cmd      = !session // the next word is a command
		live     = !session // the line has code: a prompt line, or any line of a script
		assign   = false    // inside FOO=value
		backtick = false    // inside `...`
		redirect = false    // the last word was a redirect: "&" in 2>&1 is not a separator
		last     string     // the last word
	)
	for i := range ps {
		p := &ps[i]
		if p.typ == chroma.Text && strings.TrimSpace(p.val) == "" {
			if strings.Contains(p.val, "\n") && last != `\` {
				cmd, assign, redirect, live = !session, false, false, !session
			} else if !strings.Contains(p.val, "\n") {
				assign = false
			}
			continue
		}
		if session && p.typ == chroma.GenericPrompt {
			cmd, live = true, true
			continue
		}
		last = p.val
		if !live || assign {
			continue
		}
		wasRedirect := redirect
		redirect = false
		switch {
		case p.typ == chroma.Text:
			w := p.val
			switch {
			case shellSeparators[w] && !(w == "&" && wasRedirect):
				p.typ, cmd = chroma.Operator, true
			case shellRedirect.MatchString(w):
				p.typ, redirect = chroma.Operator, true
			case strings.HasPrefix(w, "-") && len(w) > 1 && !shellNumber.MatchString(w) && w != "--":
				// "--flag=value": only the flag part has the type.
				if eq := strings.IndexByte(w, '='); eq > 0 {
					rest := ps[i+1:]
					ps = append(ps[:i+1], append([]shellPiece{{chroma.Text, w[eq:]}}, rest...)...)
					p = &ps[i]
					p.val = w[:eq]
				}
				p.typ = chroma.NameAttribute
			case shellAssign.MatchString(w) && cmd:
				// An assignment before a command: the command is still to come.
			case cmd:
				p.typ = chroma.NameFunction
				cmd = shellWrappers[w]
			}
		case p.typ == chroma.NameVariable && cmd && i+1 < len(ps) && ps[i+1].val == "=":
			assign = true
		case p.typ == chroma.LiteralStringBacktick:
			backtick = !backtick
			cmd = backtick
		case p.typ == chroma.Punctuation || p.typ == chroma.Operator:
			switch w := p.val; {
			case shellSeparators[w] && !(w == "&" && wasRedirect):
				p.typ, cmd = chroma.Operator, true
			case shellRedirect.MatchString(w):
				p.typ, redirect = chroma.Operator, true
			case w == "(":
				cmd = true
			}
		case p.typ == chroma.Keyword || p.typ == chroma.NameBuiltin:
			cmd = shellStarters[p.val] || (cmd && shellWrappers[p.val])
		case p.typ != chroma.Comment && p.typ != chroma.CommentSingle:
			cmd = false
		}
	}
	out := make([]chroma.Token, len(ps))
	for i, p := range ps {
		out[i] = chroma.Token{Type: p.typ, Value: p.val}
	}
	return out
}
