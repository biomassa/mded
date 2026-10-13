// Package config reads and writes the mded settings.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the content of the config file.
type Config struct {
	Theme        string `toml:"theme"`          // the theme kept in the theme picker (T)
	DefaultMode  string `toml:"default_mode"`   // "conventional" or "vim"
	ImageMaxRows int    `toml:"image_max_rows"` // maximum image height in rows
	Images       string `toml:"images"`         // "auto", "on", or "off"
	ShowHidden   bool   `toml:"show_hidden"`    // hidden files in the sidebar at start
	TabWidth     int    `toml:"tab_width"`      // indent unit in spaces
	SidebarWidth int    `toml:"sidebar_width"`  // sidebar width in columns
}

// Defaults returns the settings of a missing config file.
func Defaults() Config {
	return Config{
		Theme:        "default",
		DefaultMode:  "conventional",
		ImageMaxRows: 20,
		Images:       "auto",
		TabWidth:     4,
		SidebarWidth: 30,
	}
}

// validate replaces a bad value with its default.
func (c *Config) validate() {
	d := Defaults()
	if c.Theme == "" {
		c.Theme = d.Theme
	}
	if c.DefaultMode != "conventional" && c.DefaultMode != "vim" {
		c.DefaultMode = d.DefaultMode
	}
	if c.Images != "auto" && c.Images != "on" && c.Images != "off" {
		c.Images = d.Images
	}
	if c.ImageMaxRows < 1 {
		c.ImageMaxRows = d.ImageMaxRows
	}
	if c.TabWidth < 1 {
		c.TabWidth = d.TabWidth
	}
	if c.SidebarWidth < 1 {
		c.SidebarWidth = d.SidebarWidth
	}
}

// Path returns the config file location (~/.config/mded/config.toml).
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mded", "config.toml"), nil
}

// Load reads the config file. A missing file gives the defaults. A missing or bad key
// gives its default.
func Load() (Config, error) {
	c := Defaults()
	p, err := Path()
	if err != nil {
		return c, err
	}
	// Decoding into the defaults keeps the default of each key that the file does not set.
	if _, err := toml.DecodeFile(p, &c); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Defaults(), fmt.Errorf("reading %s: %w", p, err)
	}
	c.validate()
	return c, nil
}

// SaveTheme writes the theme into the config file and keeps the other keys.
func SaveTheme(name string) error { return saveKey("theme", name) }

// SaveSidebarWidth writes the sidebar width into the config file and keeps the other keys.
func SaveSidebarWidth(w int) error { return saveKey("sidebar_width", int64(w)) }

// saveKey reads the file as it is, sets one key, and writes the file.
func saveKey(key string, val any) error {
	p, err := Path()
	if err != nil {
		return err
	}
	m := map[string]any{}
	if _, err := toml.DecodeFile(p, &m); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading %s: %w", p, err)
	}
	m[key] = val
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(m); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	// The file has no secrets, so it is readable by all.
	return os.WriteFile(p, buf.Bytes(), 0o644)
}
