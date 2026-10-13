# Changelog

This file records all notable changes to this project.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- The heading bars use the accent of the theme again, also in the default theme. The steps between the levels are larger: level 1 is stronger, and each lower level is fainter. Levels 1 and 2 have bright bold text, levels 3 and 4 bold text, and levels 5 and 6 muted text that is not bold.

## [0.1.0] - 2026-10-13

### Added

- View mode: the document shows as formatted markdown. Headings show as colored bars. Lists, checkboxes, links, quotes, rules, code with syntax colors, and tables show formatted.
- In view mode, `tab` selects checkboxes and links. `space` changes a checkbox, and `enter` follows a link. A link to a markdown file opens it in mded, and `backspace` goes back.
- Edit mode with a live preview. The line with the cursor shows its source. All other lines show formatted.
- Two edit modes: a conventional mode and a vim mode. `ctrl+g` changes between them.
- Conventional mode: selection with `shift`, the system clipboard, undo and redo, format keys, find and replace (`ctrl+f`), and `ctrl+k` to delete a line.
- Vim mode: normal, insert, replace, visual, and visual line modes, with motions, operators, text objects, counts, registers, marks, macros, `.`, and `:s`.
- Search with `/`, as in neovim: regular expressions, smartcase, incremental search, and highlights of all matches. In view mode, `enter` edits the document at the current match.
- Pictures in kitty and Ghostty, through the kitty graphics protocol.
- A picture picker (`ctrl+p`, or `:img` in vim mode). It inserts a reference to the picture below the cursor line.
- The edit cursor is the cursor of the terminal: a blinking underscore, and a blinking block in vim normal mode.
- A sidebar with a directory tree and a filter. It can create, rename, and delete files and directories. `ctrl+e` moves the focus between the sidebar and the document.
- Start with a file (`mded FILE`), a directory (`mded DIR`), or the current directory (`mded`).
- Color themes from godoist, with a theme picker (`T`). Each heading level has its own color in all themes.
- A config file: `~/.config/mded/config.toml`.

[Unreleased]: https://github.com/biomassa/mded/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/biomassa/mded/releases/tag/v0.1.0
