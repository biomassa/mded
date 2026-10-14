# Changelog

This file records all notable changes to this project.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- The panes have no frame. A line with the file name is at the top, and a line is at the bottom, above the status line. A vertical line separates the sidebar from the document. The top line of the pane with the focus has the accent color.

## [0.1.3] - 2026-10-14

### Changed

- A level 3 heading has the bright bold text of level 2, on its short bar.
- The text of a level 1 heading is in the middle of its bar. A heading that shows as source (the line with the cursor in edit mode) stays at the left.

## [0.1.2] - 2026-10-13

### Changed

- The mark of a picture that cannot show is `📷`, not `🖼`.
- The level 1 heading has a deeper bar with near-white text on a dark theme. Before, its text was black.
- The heading levels are easy to tell apart in all themes. Levels 1 and 2 have a bar to the full width, and levels 3 and 4 a short bar. The bar colors go from level 1 to the background in equal steps of perceived color difference. Levels 5 and 6 have no bar: level 5 has bold accent text, and level 6 plain accent text.

### Fixed

- The screen does not shift up by one row. The picture mark `🖼` with a variation selector was one cell wide for the renderer and two cells wide in the terminal (kitty), so its row wrapped. Before, the screen was correct only after a resize.
- Inline markdown in bold, italic, strikethrough, and link text shows formatted. Before, code in bold text showed its backticks.

## [0.1.1] - 2026-10-13

### Added

- In view mode, a mouse drag selects text. When you release the button, mded copies the markdown source of the selection to the system clipboard.
- Code blocks in shell languages (sh, bash, zsh, shell, console) show commands, flags, and operators in colors.
- The help (`?`, `F1`) shows the keys of all modes. The keys of the current mode come first. Long descriptions wrap.

### Changed

- All text, except the heading bars, starts one cell more to the right. Thus the body text is in line with the heading text.
- A click in view mode acts when you release the button.
- The accent of the default theme is blue (`#4180FF`), not red. It colors the focused border, the mode label, the cursor, the markers, and the headings.
- The heading bars use the accent of the theme again, also in the default theme. The steps between the levels are larger: level 1 is stronger, and each lower level is fainter. Levels 1 and 2 have bright bold text, levels 3 and 4 bold text, and levels 5 and 6 muted text that is not bold.

### Fixed

- A row of a code block refers to its own source line. Before, it referred to the line above it, so a double-click on a code line edited the line above.

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

[Unreleased]: https://github.com/biomassa/mded/compare/v0.1.3...HEAD
[0.1.3]: https://github.com/biomassa/mded/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/biomassa/mded/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/biomassa/mded/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/biomassa/mded/releases/tag/v0.1.0
