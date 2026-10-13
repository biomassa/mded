package markdown

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/biomassa/mded/internal/theme"
)

// StyleKind is the style class of a rune in a markdown source line.
type StyleKind int

// The style kinds of a source line.
const (
	KindText StyleKind = iota
	KindHeading
	KindMarker
	KindBold
	KindItalic
	KindCode
	KindLink
	KindQuote
	KindFence
)

var (
	listPrefix = regexp.MustCompile(`^(\s*)([-*+]|\d+[.)])(\s+)(\[[ xX]\]\s)?`)
	boldSpan   = regexp.MustCompile(`\*\*[^*]+\*\*|__[^_]+__`)
	italicSpan = regexp.MustCompile(`(^|[^*])\*[^*\s][^*]*\*`)
	codeSpan   = regexp.MustCompile("`[^`]+`")
	linkSpan   = regexp.MustCompile(`\[[^\]]*\]\([^)]*\)`)
)

// LineKinds gives each rune of a source line its style kind. inFence is true inside a
// fenced code block.
func LineKinds(l string, inFence bool) []StyleKind {
	k := make([]StyleKind, utf8.RuneCountInString(l))
	mark := func(re *regexp.Regexp, kind StyleKind) {
		for _, loc := range re.FindAllStringIndex(l, -1) {
			a := utf8.RuneCountInString(l[:loc[0]])
			b := a + utf8.RuneCountInString(l[loc[0]:loc[1]])
			for i := a; i < b; i++ {
				if k[i] == KindText {
					k[i] = kind
				}
			}
		}
	}
	fill := func(kind StyleKind) []StyleKind {
		for i := range k {
			k[i] = kind
		}
		return k
	}
	trim := strings.TrimSpace(l)
	switch {
	case inFence || strings.HasPrefix(trim, "```"):
		return fill(KindFence)
	case strings.HasPrefix(trim, "#"):
		return fill(KindHeading)
	case strings.HasPrefix(trim, ">"):
		return fill(KindQuote)
	}
	if p := listPrefix.FindString(l); p != "" {
		for i := range utf8.RuneCountInString(p) {
			k[i] = KindMarker
		}
	}
	mark(codeSpan, KindCode)
	mark(linkSpan, KindLink)
	mark(boldSpan, KindBold)
	mark(italicSpan, KindItalic)
	return k
}

// KindStyle is the style of a style kind.
func KindStyle(kind StyleKind) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(theme.C(theme.Text))
	switch kind {
	case KindHeading:
		return s.Foreground(theme.C(theme.Fg(theme.Accent))).Bold(true)
	case KindMarker:
		return s.Foreground(theme.C(theme.Fg(theme.Accent)))
	case KindBold:
		return s.Bold(true)
	case KindItalic:
		return s.Italic(true)
	case KindCode, KindFence:
		return s.Foreground(theme.C(theme.Code))
	case KindLink:
		return s.Foreground(theme.C(theme.Link)).Underline(true)
	case KindQuote:
		return s.Foreground(theme.C(theme.Muted)).Italic(true)
	}
	return s
}

// WrapLine splits a line into screen segments of at most w runes, at spaces if possible.
// It returns the start of each segment.
func WrapLine(l []rune, w int) []int {
	w = max(1, w)
	starts := []int{0}
	for s := 0; len(l)-s > w; {
		cut := s + w
		for i := cut; i > s+w/2; i-- {
			if l[i-1] == ' ' {
				cut = i
				break
			}
		}
		starts = append(starts, cut)
		s = cut
	}
	return starts
}

// HeadingLevel returns the level (1 to 6) and the text of an ATX heading line, or 0.
func HeadingLevel(l string) (int, string) {
	t := strings.TrimLeft(l, " ")
	n := 0
	for n < len(t) && n < 7 && t[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || (n < len(t) && t[n] != ' ') {
		return 0, ""
	}
	return n, strings.TrimSpace(strings.TrimRight(strings.TrimSpace(t[n:]), "#"))
}
