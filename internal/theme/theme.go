// Package theme holds the color themes of mded: the palettes, the derived colors, and the
// markdown styles that depend on them.
package theme

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"

	gansi "charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Theme colors. Apply sets them. The default is the dark "default" theme.
var (
	Accent   = "#4180FF" // the focus color
	Error    string      // errors (overdue in godoist)
	Good     string      // done, saved (today in godoist)
	Warn     string      // warnings (tomorrow in godoist)
	Purple   string      // code and special text (week in godoist)
	Muted    string
	Dim      string
	Border   string
	Text     string
	SelBg    string
	SelBgDim string
	BaseBg   string // terminal background, used for tinting
	// The markdown colors: links and inline code.
	Link, Code string

	darkTheme   bool
	chromaStyle string // the code block style of a palette theme, or ""
)

func init() { setDefault(true) }

// setDefault selects the dark or the light colors of the "default" theme.
func setDefault(dark bool) {
	darkTheme = dark
	Accent = "#4180FF" // blue
	chromaStyle = ""
	if dark {
		Error, Good, Warn, Purple = "#FF7066", "#25B84C", "#FF9A14", "#A970FF"
		Muted, Dim, Border = "#8A8A8A", "#5C5C5C", "#3D3D3D"
		Text = "#E6E6E6"
		SelBg, SelBgDim, BaseBg = "#363636", "#2A2A2A", "#1F1F1F"
		Link, Code = Good, Purple
		return
	}
	Error, Good, Warn, Purple = "#D1453B", "#058527", "#AD6200", "#692FC2"
	Muted, Dim, Border = "#666666", "#9A9A9A", "#D0D0D0"
	Text = "#202020"
	SelBg, SelBgDim, BaseBg = "#E4E4E4", "#EFEFEF", "#FFFFFF"
	Link, Code = Good, Purple
}

// C converts a hex string to a color.
func C(hex string) color.Color { return lipgloss.Color(hex) }

// St returns a style with the foreground color hex.
func St(hex string) lipgloss.Style { return lipgloss.NewStyle().Foreground(C(hex)) }

// parseHex splits "#RRGGBB" into its components. It returns gray for a bad value.
func parseHex(h string) (r, g, b float64) {
	if len(h) != 7 || h[0] != '#' {
		return 128, 128, 128
	}
	v, err := strconv.ParseUint(h[1:], 16, 32)
	if err != nil {
		return 128, 128, 128
	}
	return float64(v >> 16 & 0xff), float64(v >> 8 & 0xff), float64(v & 0xff)
}

// Mix blends a toward b by t (0 = a, 1 = b).
func Mix(a, b string, t float64) string {
	ar, ag, ab := parseHex(a)
	br, bg, bb := parseHex(b)
	l := func(x, y float64) int { return int(math.Round(x + (y-x)*t)) }
	return fmt.Sprintf("#%02X%02X%02X", l(ar, br), l(ag, bg), l(ab, bb))
}

// luminance is the relative brightness of a color, from 0 to 1.
func luminance(h string) float64 {
	r, g, b := parseHex(h)
	return (0.2126*r + 0.7152*g + 0.0722*b) / 255
}

// Fg makes a palette color readable on the terminal background.
func Fg(h string) string {
	if darkTheme && luminance(h) < 0.35 {
		return Mix(h, "#FFFFFF", 0.35)
	}
	if !darkTheme && luminance(h) > 0.6 {
		return Mix(h, "#000000", 0.35)
	}
	return h
}

// A palette is a color theme. The "default" theme has no palette: it uses the godoist
// "todoist" colors with a blue accent, and the terminal background selects its dark or
// light form. The other
// palettes come from tideui by Allie Bayless (github.com/allisonhere/tideui, MIT license).
type palette struct {
	name          string
	bg, fg        string // background and text
	border, focus string // unfocused and focused borders. focus is also the accent
	selected      string // selected row
	good          string // "done" green: saved, completed
	dimmed        string // secondary text
	errorHex      string // errors
}

// defaultTheme is the name of the default theme.
const defaultTheme = "default"

// palettes are the themes after "default", in picker order.
var palettes = []palette{
	{"catppuccin-mocha", "#1e1e2e", "#cdd6f4", "#6c7086", "#89b4fa", "#89b4fa", "#a6e3a1", "#585b70", "#f38ba8"},
	{"catppuccin-latte", "#eff1f5", "#4c4f69", "#9ca0b0", "#1e66f5", "#1e66f5", "#40a02b", "#8c8fa1", "#d20f39"},
	{"catppuccin-frappe", "#303446", "#c6d0f5", "#626880", "#8caaee", "#8caaee", "#a6d189", "#51576d", "#e78284"},
	{"catppuccin-macchiato", "#24273a", "#cad3f5", "#5b6078", "#8aadf4", "#8aadf4", "#a6da95", "#494d64", "#ed8796"},
	{"nord", "#2e3440", "#eceff4", "#4c566a", "#88c0d0", "#88c0d0", "#a3be8c", "#4c566a", "#bf616a"},
	{"dracula", "#282a36", "#f8f8f2", "#6272a4", "#bd93f9", "#bd93f9", "#50fa7b", "#6272a4", "#ff5555"},
	{"gruvbox-dark", "#282828", "#ebdbb2", "#504945", "#83a598", "#83a598", "#b8bb26", "#504945", "#fb4934"},
	{"gruvbox-light", "#fbf1c7", "#3c3836", "#bdae93", "#076678", "#076678", "#79740e", "#bdae93", "#cc241d"},
	{"tokyo-night", "#1a1b26", "#c0caf5", "#414868", "#7aa2f7", "#7aa2f7", "#9ece6a", "#414868", "#f7768e"},
	{"tokyo-night-day", "#e1e2e7", "#3760bf", "#a8aecb", "#2e7de9", "#2e7de9", "#587539", "#a8aecb", "#f52a65"},
	{"rose-pine", "#191724", "#e0def4", "#403d52", "#c4a7e7", "#c4a7e7", "#9ccfd8", "#403d52", "#eb6f92"},
	{"rose-pine-moon", "#232136", "#e0def4", "#44415a", "#c4a7e7", "#c4a7e7", "#9ccfd8", "#44415a", "#eb6f92"},
	{"rose-pine-dawn", "#faf4ed", "#575279", "#d7d2be", "#907aa9", "#907aa9", "#286983", "#d7d2be", "#b4637a"},
	{"one-dark", "#282c34", "#abb2bf", "#3e4451", "#61afef", "#61afef", "#98c379", "#3e4451", "#e06c75"},
	{"magenta-geode", "#47003c", "#f3b0dc", "#aa4d84", "#c83fa9", "#c83fa9", "#f3b0dc", "#77176e", "#ff7062"},
	{"coral-sunset", "#444154", "#fec9c1", "#fc8b79", "#ff7062", "#ff7062", "#fec9c1", "#7a637f", "#ff7062"},
	{"lavender-fields-forever", "#382d72", "#e5ccf4", "#b7c2c6", "#a080e1", "#a080e1", "#e5ccf4", "#5c509c", "#ff7062"},
	{"vt100", "#000000", "#33ff33", "#145214", "#00ff00", "#00ff00", "#66ff66", "#3dcc3d", "#ff6b6b"},
	{"vt52", "#000000", "#ffcc66", "#6b4e14", "#ffb020", "#ffb020", "#ffe6a8", "#a67c2e", "#ff6666"},
}

// Names returns all theme names, "default" first.
func Names() []string {
	names := []string{defaultTheme}
	for _, p := range palettes {
		names = append(names, p.name)
	}
	return names
}

// themeName is the theme in use. themeBg is its background, or "" for the terminal
// background ("default" theme).
var (
	themeName = defaultTheme
	themeBg   string
)

// Current returns the name of the theme in use.
func Current() string { return themeName }

// Bg returns the background color of the theme, or "" for the terminal background.
func Bg() string { return themeBg }

// Dark reports whether the theme has a dark background.
func Dark() bool { return darkTheme }

// Apply sets the colors of theme name. termDark is the terminal background of the last
// report, for the "default" theme. An unknown name gives the "default" theme.
func Apply(name string, termDark bool) {
	for _, p := range palettes {
		if p.name == name {
			applyPalette(p)
			return
		}
	}
	themeName, themeBg = defaultTheme, ""
	setDefault(termDark)
}

// applyPalette maps a palette onto the colors. The godoist meanings stay: Error is the
// error color, Good is the green, the accent is the focus color. Warn and Purple are mixes
// that keep their hue family where the palette has none: orange from the error color and
// yellow, purple from the error and focus colors.
func applyPalette(p palette) {
	themeName, themeBg = p.name, p.bg
	darkTheme = luminance(p.bg) < 0.5
	BaseBg = p.bg
	Text = p.fg
	Accent = p.focus
	Border = p.border
	Dim = Readable(p.dimmed, p.bg, 3)
	Muted = Readable(Mix(p.fg, p.bg, 0.4), p.bg, 4.5)
	SelBg, SelBgDim = Mix(p.selected, p.bg, 0.72), Mix(p.selected, p.bg, 0.85)
	Error = Readable(p.errorHex, p.bg, 3)
	Good = Readable(p.good, p.bg, 3)
	Warn = Readable(Mix(p.errorHex, "#FFC000", 0.55), p.bg, 3)
	Purple = Readable(Mix(p.errorHex, p.focus, 0.6), p.bg, 3)
	Link = Readable(p.focus, p.bg, 3)
	Code = Purple
	// All heading levels use the accent. HeadingStyle makes the bar fainter for each level.
	chromaStyle = chromaStyles[p.name]
}

// chromaStyles are the Chroma code styles that match a palette. A palette without a match
// gets code colors from its own colors (see themeReader).
var chromaStyles = map[string]string{
	"catppuccin-mocha": "catppuccin-mocha", "catppuccin-latte": "catppuccin-latte",
	"catppuccin-frappe": "catppuccin-frappe", "catppuccin-macchiato": "catppuccin-macchiato",
	"nord": "nord", "dracula": "dracula", "gruvbox-dark": "gruvbox", "gruvbox-light": "gruvbox-light",
	"tokyo-night": "tokyonight-night", "tokyo-night-day": "tokyonight-day",
	"rose-pine": "rose-pine", "rose-pine-moon": "rose-pine-moon", "rose-pine-dawn": "rose-pine-dawn",
	"one-dark": "onedark",
}

// themeReader gives the markdown reader the colors of a palette theme: the text, links,
// quotes, rules, inline code, and the code blocks.
func themeReader(st *gansi.StyleConfig) {
	col := func(h string) *string { return &h }
	st.Document.Color = col(Text)
	st.Paragraph.Color = col(Text)
	st.BlockQuote.Color = col(Muted)
	st.HorizontalRule.Color = col(Border)
	st.Link.Color, st.LinkText.Color = col(Link), col(Link)
	st.Item.Color, st.Enumeration.Color = col(Text), col(Text)
	st.Code.Color = col(Code)
	st.Code.BackgroundColor = col(Mix(Code, BaseBg, 0.85))
	st.Table.Color = col(Text)
	st.CodeBlock.Color = col(Text)
	if chromaStyle != "" {
		st.CodeBlock.Theme, st.CodeBlock.Chroma = chromaStyle, nil
		return
	}
	prim := func(h string) gansi.StylePrimitive { return gansi.StylePrimitive{Color: col(h)} }
	st.CodeBlock.Theme = ""
	st.CodeBlock.Chroma = &gansi.Chroma{
		Text: prim(Text), Error: prim(Error), Comment: prim(Muted), CommentPreproc: prim(Muted),
		Keyword: prim(Link), KeywordReserved: prim(Link), KeywordNamespace: prim(Link), KeywordType: prim(Purple),
		Operator: prim(Purple), Punctuation: prim(Muted), Name: prim(Text), NameBuiltin: prim(Purple),
		NameTag: prim(Link), NameAttribute: prim(Warn), NameClass: prim(Purple), NameConstant: prim(Warn),
		NameDecorator: prim(Warn), NameFunction: prim(Link), Literal: prim(Warn),
		LiteralNumber: prim(Warn), LiteralString: prim(Good), LiteralStringEscape: prim(Purple),
		GenericDeleted: prim(Error), GenericInserted: prim(Good),
	}
}

// Contrast is the WCAG contrast ratio of two colors, from 1 to 21.
func Contrast(a, b string) float64 {
	la, lb := relLum(a), relLum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// relLum is the WCAG relative luminance of a color.
func relLum(h string) float64 {
	r, g, b := parseHex(h)
	lin := func(v float64) float64 {
		v /= 255
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// Readable moves h toward white or black (away from bg) until it has the contrast min
// against bg, so that dim palette colors stay readable. It keeps the hue.
func Readable(h, bg string, min float64) string {
	to := "#FFFFFF"
	if luminance(bg) > 0.5 {
		to = "#000000"
	}
	out := h
	for t := 0.1; Contrast(out, bg) < min && t <= 1; t += 0.1 {
		out = Mix(h, to, t)
	}
	return out
}

// Label is the name shown in the picker: "default" shows that it follows the terminal.
func Label(name string) string {
	if name == defaultTheme {
		return fmt.Sprintf("%s (terminal colors)", name)
	}
	return name
}

// headingShades is how far the bar of each level goes from the level 1 bar toward the
// background. The steps are large, so that the levels are easy to tell apart.
var headingShades = [6]float64{0, 0.3, 0.5, 0.65, 0.78, 0.87}

// headingAccent is the color of the heading bars: the accent of the theme, readable on the
// background.
func headingAccent() string { return Readable(Accent, BaseBg, 3) }

// headingLight is the bright text of levels 1 and 2: near white on a dark theme, near black
// on a light theme, with a tint of the accent.
func headingLight() string {
	if darkTheme {
		return Mix(headingAccent(), "#FFFFFF", 0.85)
	}
	return Mix(headingAccent(), "#000000", 0.6)
}

// headingTop is the bar of level 1: the accent, made deeper (toward black on a dark theme,
// toward white on a light theme) until the bright text is readable on it.
func headingTop() string {
	to := "#000000"
	if !darkTheme {
		to = "#FFFFFF"
	}
	h, text := headingAccent(), headingLight()
	bar := h
	for t := 0.05; t <= 0.9 && Contrast(text, bar) < 4.5; t += 0.05 {
		bar = Mix(h, to, t)
	}
	return bar
}

// HeadingColors returns the text and the bar color of a heading level. All levels use the
// accent. Level 1 has the strongest bar, and each lower level goes further toward the
// background. The text is bright for levels 1 and 2, the text color for levels 3 and 4, and
// the muted color for levels 5 and 6.
func HeadingColors(level int) (text, bar string) {
	lv := max(1, min(level, 6))
	switch {
	case lv <= 2:
		text = headingLight()
	case lv <= 4:
		text = Text
	default:
		text = Muted
	}
	bar = Mix(headingTop(), BaseBg, headingShades[lv-1])
	return Readable(text, bar, 4.5), bar
}

// HeadingStyle is the bar style of a heading level: a tinted background, with bold text for
// levels 1 to 4.
func HeadingStyle(level int) lipgloss.Style {
	text, bar := HeadingColors(level)
	return lipgloss.NewStyle().Background(C(bar)).Foreground(C(text)).Bold(level <= 4)
}

// HeadingBar draws a heading as a full-width bar.
func HeadingBar(level int, text string, w int) string {
	t := ansi.Truncate(text, max(1, w-3), "…")
	return HeadingStyle(level).Render(" " + t + strings.Repeat(" ", max(0, w-2-lipgloss.Width(t))))
}

// ReaderStyle is the Glamour dark or light style with clean headings (no "##" marks) and
// ☐ / ☑ checkboxes. Links are OSC 8 hyperlinks, and fenced code gets syntax colors.
func ReaderStyle() gansi.StyleConfig {
	st := styles.DarkStyleConfig
	if !darkTheme {
		st = styles.LightStyleConfig
	}
	st.H1.Prefix, st.H1.Suffix = " ", " "
	for _, h := range []*gansi.StyleBlock{&st.H2, &st.H3, &st.H4, &st.H5, &st.H6} {
		h.Prefix = ""
	}
	st.Task.Ticked, st.Task.Unticked = "☑ ", "☐ "
	if themeBg != "" {
		themeReader(&st)
	}
	return st
}
