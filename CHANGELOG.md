# Changelog

This file records all notable changes to this project.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.2] - 2026-11-07

### Changed

- In the key hints of the status line, the keys are bold and have the accent color, as in godoist. The descriptions are muted.

## [0.3.1] - 2026-11-05

### Changed

- In view mode, `tab` and `shift+tab` move the focus to the sidebar, and show it if it is hidden. `]` and `[` now select the next and the previous checkbox or link. Thus `tab` switches between the sidebar and the document when you do not edit.

## [0.3.0] - 2026-11-05

### Added

- The sidebar sorts by name, time, size, or type (extension). `s` changes the sort, and `S` reverses it. The title shows the sort, and mded saves it in the config file.
- In the sidebar, `pgup` and `pgdown` move one screen, and `home` and `end` go to the first and the last entry.
- In the sidebar, `q` quits, `?` shows the keys, and `tab` moves the focus to the document.
- mded reads a file in the background, with the status `loading NAME…`. `esc` stops the load.

### Changed

- In the sidebar, `enter`, `space`, and a double-click open and close a directory. Before, `enter` on an open directory went to its first entry. `→` still does that.
- In the sidebar, directories have the accent color, markdown files the text color, and other files a muted color.
- The view hint shows `ctrl+e sidebar`.
- A file larger than 50 MB opens in view mode only. mded does not open a file larger than 100 MB.
- The unsaved mark `[+]` stays after a change that you take back by hand. An undo to the saved text removes it.

### Fixed

- A large file does not make mded use all the memory of the computer. Before, a 1 MB file needed more than 1 GB, and a 10 MB file could make the system stop the terminal. Now mded builds only the rows on the screen, a row needs much less memory, and an undo step keeps only the lines that it changed. A 9 MB file opens in less than 0.1 seconds with less than 100 MB.
- A key in edit mode answers fast also in a large file. Before, one key in a 9 MB file took 5 seconds.
- In vim mode, `H`, `M`, and `L` go to the top, the middle, and the bottom of the screen. Before, they counted from the first line of the document.

## [0.2.0] - 2026-10-18

### Added

- A backslash escape shows the mark as text: `\*` shows `*`.
- An autolink (`<https://example.com>`) shows its URL as a link. `tab` selects it in view mode.

### Changed

- Tables follow GitHub Flavored Markdown. The delimiter row can have one dash or more for each column, and the outer pipes are optional. Before, a table needed three dashes and a pipe at the start of each line.

## [0.1.4] - 2026-10-14

### Changed

- The panes have no frame. A line with the file name is at the top, and a line is at the bottom, above the status line. A vertical line separates the sidebar from the document. The top line of the pane with the focus has the accent color.

The release binaries of 0.1.3 already have this change. `go install` of 0.1.3 does not.

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

[Unreleased]: https://github.com/biomassa/mded/compare/v0.3.2...HEAD
[0.3.2]: https://github.com/biomassa/mded/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/biomassa/mded/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/biomassa/mded/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/biomassa/mded/compare/v0.1.4...v0.2.0
[0.1.4]: https://github.com/biomassa/mded/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/biomassa/mded/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/biomassa/mded/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/biomassa/mded/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/biomassa/mded/releases/tag/v0.1.0
