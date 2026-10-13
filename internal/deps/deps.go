// Package deps pins the dependencies until the packages that use them exist.
package deps

import (
	_ "charm.land/bubbles/v2/textinput"
	_ "charm.land/bubbletea/v2"
	_ "charm.land/glamour/v2"
	_ "charm.land/lipgloss/v2"
	_ "github.com/BurntSushi/toml"
	_ "github.com/charmbracelet/colorprofile"
	_ "github.com/charmbracelet/x/ansi"
	_ "golang.org/x/image/webp"
)
