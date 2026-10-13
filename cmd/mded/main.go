// Command mded is a markdown viewer and editor for the terminal.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/biomassa/mded/internal/config"
	"github.com/biomassa/mded/internal/keymap"
	"github.com/biomassa/mded/internal/keymap/vim"
	"github.com/biomassa/mded/internal/kitty"
	"github.com/biomassa/mded/internal/theme"
	"github.com/biomassa/mded/internal/tui"
	"github.com/biomassa/mded/internal/version"
)

func main() { os.Exit(run(os.Args[1:])) }

// run does the work of the command and returns the exit code.
func run(args []string) int {
	var usage bytes.Buffer
	fs := flag.NewFlagSet("mded", flag.ContinueOnError)
	fs.SetOutput(&usage)
	vimMode := fs.Bool("vim", false, "start with vim edit mode")
	themeName := fs.String("theme", "", "use theme `NAME` for this run")
	showVersion := fs.Bool("version", false, "print the version and exit")
	fs.Usage = func() {
		fmt.Fprintln(&usage, "usage: mded [flags] [FILE|DIR]")
		fs.PrintDefaults()
	}
	switch err := fs.Parse(args); err {
	case nil:
	case flag.ErrHelp:
		fmt.Print(usage.String())
		return 0
	default:
		fmt.Fprint(os.Stderr, usage.String())
		return 2
	}
	if *showVersion {
		fmt.Println("mded", version.String())
		return 0
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "mded: too many arguments")
		fs.Usage()
		fmt.Fprint(os.Stderr, usage.String())
		return 2
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "mded: warning:", err)
	}
	if *themeName != "" {
		if !validTheme(*themeName) {
			fmt.Fprintf(os.Stderr, "mded: unknown theme %q\nthemes:\n  %s\n", *themeName, strings.Join(theme.Names(), "\n  "))
			return 2
		}
		cfg.Theme = *themeName
	}
	theme.Apply(cfg.Theme, true) // "default" follows the terminal: the model corrects it later

	opts := tui.Options{
		Config: cfg,
		Vim:    *vimMode,
		NewVim: func(tabWidth int) keymap.Keymap { return vim.New(tabWidth) },
	}
	if fs.NArg() == 1 {
		opts.Path = fs.Arg(0)
	}
	var progOpts []tea.ProgramOption
	if kitty.Supported(cfg.Images) {
		w, h := kitty.CellSize()
		opts.Images = kitty.NewManager(w, h)
		// The image id is a color. A downsampled color would give another id.
		progOpts = append(progOpts, tea.WithColorProfile(colorprofile.TrueColor))
	}
	m, err := tui.New(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mded:", err)
		return 1
	}
	_, err = tea.NewProgram(m, progOpts...).Run()
	if s := m.Cleanup(); s != "" {
		os.Stdout.WriteString(s)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "mded:", err)
		return 1
	}
	return 0
}

// validTheme reports whether name is a theme name.
func validTheme(name string) bool {
	for _, n := range theme.Names() {
		if n == name {
			return true
		}
	}
	return false
}
