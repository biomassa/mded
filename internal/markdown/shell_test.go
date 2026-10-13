package markdown

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
)

// shellTypes returns the non-white-space tokens as "type:value" strings.
func shellTypes(t *testing.T, lang, src string) []string {
	t.Helper()
	it, err := lexers.Get(lang).Tokenise(nil, src)
	if err != nil {
		t.Fatal(err)
	}
	var out, all []string
	for _, tk := range it.Tokens() {
		all = append(all, tk.Value)
		if strings.TrimSpace(tk.Value) != "" {
			out = append(out, tk.Type.String()+":"+strings.TrimSpace(tk.Value))
		}
	}
	if got := strings.Join(all, ""); got != src {
		t.Fatalf("values changed: %q != %q", got, src)
	}
	return out
}

func TestShellLexerRegistered(t *testing.T) {
	for _, n := range []string{"sh", "bash", "zsh", "shell", "console", "shell-session", "ksh"} {
		if _, ok := lexers.Get(n).(*shellLexer); !ok {
			t.Errorf("lexers.Get(%q) is not the wrapper", n)
		}
	}
}

func TestShellTokens(t *testing.T) {
	cases := []struct{ lang, src, want string }{
		{"sh", "curl -LO https://x", "NameFunction:curl NameAttribute:-LO Text:https://x"},
		{"sh", "sudo rm -rf /usr/local/go", "NameFunction:sudo NameFunction:rm NameAttribute:-rf Text:/usr/local/go"},
		{"sh", "sudo -E make", "NameFunction:sudo NameAttribute:-E NameFunction:make"},
		{"sh", "echo 'a' >> ~/.profile", "NameBuiltin:echo LiteralStringSingle:'a' Operator:>> Text:~/.profile"},
		{"bash", "FOO=1 make build && ./run --verbose | tee log",
			"NameVariable:FOO Operator:= LiteralNumber:1 NameFunction:make Text:build Operator:&& NameFunction:./run NameAttribute:--verbose Operator:| NameFunction:tee Text:log"},
		{"sh", "git --git-dir=x log", "NameFunction:git NameAttribute:--git-dir Operator:= Text:x Text:log"},
		{"sh", "ls 2>&1 | wc", "NameFunction:ls Operator:2> Punctuation:& LiteralNumber:1 Operator:| NameFunction:wc"},
		{"sh", "a; b", "NameFunction:a Operator:; NameFunction:b"},
		{"console", "$ curl -LO https://x\nout -put | x\n", "GenericPrompt:$ NameFunction:curl NameAttribute:-LO Text:https://x GenericOutput:out -put | x"},
	}
	for _, c := range cases {
		if got := strings.Join(shellTypes(t, c.lang, c.src), " "); got != c.want {
			t.Errorf("%q:\n got %s\nwant %s", c.src, got, c.want)
		}
	}
}

func TestShellStyleColors(t *testing.T) {
	for _, name := range []string{"catppuccin-mocha", "catppuccin-latte", "catppuccin-frappe", "catppuccin-macchiato",
		"nord", "dracula", "gruvbox", "gruvbox-light", "tokyonight-night", "tokyonight-day",
		"rose-pine", "rose-pine-moon", "rose-pine-dawn", "onedark"} {
		st := chromastyles.Get(name)
		text := st.Get(chroma.Text).Colour
		for _, tt := range []chroma.TokenType{chroma.NameFunction, chroma.NameAttribute, chroma.Operator} {
			if c := st.Get(tt).Colour; !c.IsSet() || c == text {
				t.Errorf("style %s: %v has no own color", name, tt)
			}
		}
	}
	for _, cfg := range []ansi.StyleConfig{styles.DarkStyleConfig, styles.LightStyleConfig} {
		c := cfg.CodeBlock.Chroma
		for _, p := range []*string{c.NameFunction.Color, c.NameAttribute.Color, c.Operator.Color} {
			if p == nil || *p == *c.Text.Color {
				t.Error("default style: a shell type has no own color")
			}
		}
	}
}

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// colorBefore returns the last escape sequence before word on the first line with it.
func colorBefore(out, word string) string {
	for _, l := range strings.Split(out, "\n") {
		if i := strings.Index(l, word); i >= 0 {
			if m := sgr.FindAllString(l[:i], -1); len(m) > 0 {
				return m[len(m)-1]
			}
		}
	}
	return ""
}

func TestShellRenderColors(t *testing.T) {
	out := NewCache().Render("```sh\ncurl -LO https://x\n```\n", 60)
	s := strings.Join(out, "\n")
	cmd, flag, url := colorBefore(s, "curl"), colorBefore(s, "-LO"), colorBefore(s, "https")
	if cmd == "" || flag == "" || url == "" || cmd == url || flag == url || cmd == flag {
		t.Errorf("colors: curl %q, -LO %q, url %q", cmd, flag, url)
	}
}
