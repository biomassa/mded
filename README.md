# mded

mded is a markdown viewer and editor for the terminal.

## Features

- **View mode.** The document shows as formatted markdown: heading bars, lists, checkboxes, links, quotes, rules, code with syntax colors, and tables.
- **Edit mode with a live preview.** The line with the cursor shows its source. All other lines show formatted.
- **Two edit modes.** A conventional mode (as in most text editors) and a vim mode. `ctrl+g` changes between them.
- **Search with `/`**, as in neovim: regular expressions, smartcase, incremental search, and highlights of all matches.
- **Pictures** in kitty and Ghostty, through the kitty graphics protocol.
- **Sidebar** with a directory tree, a filter, and file operations.
- **Color themes**, with a theme picker.

## Install

```sh
go install github.com/biomassa/mded/cmd/mded@latest
```

## Start

| Command | Result |
|---|---|
| `mded` | Shows the sidebar with the current directory. |
| `mded DIR` | Shows the sidebar with DIR. |
| `mded FILE` | Opens FILE in view mode. If FILE does not exist, mded creates it when you save. |

Flags:

| Flag | Action |
|---|---|
| `--vim` | Use vim mode for editing. |
| `--theme NAME` | Use the theme NAME for this run. |
| `--version` | Show the version. |
| `-h`, `--help` | Show the help. |

## Modes

The file opens in view mode. Push `e` or `i` to edit. The cursor goes to its last position in this document.

- **Conventional mode.** `esc` goes back to view mode.
- **Vim mode.** `esc` goes from insert mode or visual mode to normal mode. `esc` in normal mode goes back to view mode.

`ctrl+g` changes the edit mode between conventional and vim. In view mode, it sets the mode that `e` and `i` open. The status line shows the mode: `VIEW`, `EDIT`, `NORMAL`, `INSERT`, `VISUAL`, `V-LINE`, or `REPLACE`.

## Save and quit

mded does not save by itself.

- `ctrl+s` saves in view mode, conventional mode, and vim normal mode. In vim mode, `:w` also saves.
- `q` in view mode or `ctrl+q` quits. If the document has unsaved changes, mded asks first: `s` saves, `d` discards, `esc` cancels.

mded keeps the file mode, the line endings, and the last line ending of the file.

## View mode

| Key | Action |
|---|---|
| `j` / `↓`, `k` / `↑` | Scroll one row. |
| `ctrl+d`, `ctrl+u` | Scroll half a page. |
| `space` / `pgdown`, `b` / `pgup` | Scroll one page. |
| `g` / `home`, `G` / `end` | Go to the top or the bottom. |
| `/` | Search. |
| `n`, `N` | Go to the next or the previous match. |
| `tab`, `shift+tab` | Select the next or the previous checkbox or link. |
| `space` | Change the selected checkbox. |
| `enter` | Follow the selected link. |
| `backspace`, `ctrl+o` | Go back to the previous document. |
| `e`, `i` | Edit. |
| `T` | Open the theme picker. |
| `?`, `F1` | Show the keys. |
| `q`, `ctrl+q` | Quit. |

A link to a file opens the file in mded. A link with `#heading` goes to that heading. Other links open in the default application of the system.

## Search

`/` opens the search prompt on the status line. In vim mode, `?` searches backward.

- The pattern is a [Go regular expression](https://pkg.go.dev/regexp/syntax). A pattern with no capital letter ignores case.
- mded goes to the first match while you type, and highlights all matches. `enter` keeps the position, and `esc` goes back.
- The highlights stay until you push `esc` in view mode, type `:noh` in vim mode, or start a new search.
- `n` and `N` go to the next and the previous match. At the end of the file, the search continues at the start.
- `↑` and `↓` in the prompt show the earlier patterns.

## Conventional mode

| Key | Action |
|---|---|
| Arrows, `home`, `end`, `pgup`, `pgdown` | Move. |
| `alt+←` / `alt+→`, `ctrl+←` / `ctrl+→` | Move one word. |
| `alt+backspace` | Delete a word. |
| `shift` + a move key | Select text. |
| `ctrl+a` | Select all. |
| `ctrl+c`, `ctrl+x`, `ctrl+v` | Copy, cut, and paste with the system clipboard. |
| `ctrl+z`, `ctrl+y` | Undo, redo. |
| `ctrl+b`, `ctrl+i`, `ctrl+k`, `ctrl+t` | Bold, italic, link, checkbox. |
| `tab` | Insert spaces (`tab_width`). |
| `enter` | Start a new line. On a list item, start the next item. On an empty item, end the list. |
| `ctrl+f` | Find and replace. |
| `ctrl+s` | Save. |
| `esc` | Go back to view mode. |

The clipboard goes through the terminal (OSC 52). kitty can ask you to allow a paste the first time.

## Vim mode

Vim mode has normal, insert, replace, visual, and visual line modes. It has:

- Motions: `h j k l w W b B e E ge 0 ^ $ gg G f F t T ; , % H M L { }`, `ctrl+d ctrl+u ctrl+f ctrl+b`, `n N * #`, and marks.
- Operators: `d c y > < gu gU g~`, with motions and text objects (`iw aw iW aW is as ip ap i" a" i' a' i( a( i[ a[ i{ a{ i< a<` and others).
- Edits: `x X s S r J ~ p P i a I A o O R u ctrl+r .`, with counts.
- Registers: `"a`–`"z`, `"0`–`"9`, `"-`, `"_`, and `"+` / `"*` for the system clipboard. Each yank and delete also goes to the system clipboard.
- Marks (`m`) and macros (`q`, `@`).
- Commands: `:w`, `:w FILE`, `:q`, `:q!`, `:wq`, `:x`, `:e FILE`, `:noh`, `:N`, and `:s` with ranges and the flags `g`, `i`, `I`, and `c`.

## Pictures

mded shows pictures in terminals with the kitty graphics protocol: kitty and Ghostty. mded does not show pictures in tmux.

- A picture is a reference `![alt](path)` to a local file. The path is relative to the directory of the document, or absolute. mded does not load pictures from the internet.
- The formats are PNG, JPEG, GIF (the first frame), and WebP.
- A picture shows at its natural size. If it is too large, mded makes it smaller to fit the width of the pane and `image_max_rows`.
- A picture alone on its line replaces the line. A picture in a line of text shows below the line.
- In edit mode, the line with the cursor shows its source, and its pictures do not show.
- Without picture support, mded shows `🖼` and the alt text.

## Sidebar

`ctrl+`` shows or hides the sidebar. `ctrl+w w`, `alt+h`, and `alt+l` move the focus between the sidebar and the document.

| Key | Action |
|---|---|
| `↑`, `↓` | Move. |
| `→`, `enter` | Expand a directory, or open a file. |
| `←` | Collapse a directory, or go to the parent directory. |
| `/` | Filter the names. `esc` removes the filter. |
| `ctrl+n` | Make a new file. |
| `ctrl+shift+n` | Make a new directory. |
| `F2` | Rename. |
| `delete` | Delete. mded asks first. |
| `ctrl+h` | Show or hide hidden files. |

The sidebar shows all files. Markdown files open formatted. Other text files open as plain text. Picture files show the picture. mded does not open other binary files.

Drag the border between the sidebar and the document to change the width. mded saves the width in the config file.

## Themes

`T` in view mode opens the theme picker. A move shows the theme. `enter` keeps it and saves it in the config file. `esc` goes back to the theme in use.

The themes are: default, catppuccin-mocha, catppuccin-latte, catppuccin-frappe, catppuccin-macchiato, nord, dracula, gruvbox-dark, gruvbox-light, tokyo-night, tokyo-night-day, rose-pine, rose-pine-moon, rose-pine-dawn, one-dark, magenta-geode, coral-sunset, lavender-fields-forever, vt100, and vt52. The `default` theme uses the colors of your terminal.

## Config

The config file is `~/.config/mded/config.toml`. All keys are optional.

| Key | Default | Values |
|---|---|---|
| `theme` | `"default"` | A theme name. |
| `default_mode` | `"conventional"` | `"conventional"` or `"vim"`. |
| `image_max_rows` | `20` | The maximum height of a picture, in rows. |
| `images` | `"auto"` | `"auto"`, `"on"`, or `"off"`. `"auto"` finds kitty and Ghostty. |
| `show_hidden` | `false` | Show hidden files in the sidebar at the start. |
| `tab_width` | `4` | The number of spaces for one indent. |
| `sidebar_width` | `30` | The width of the sidebar, in columns. |

## Mouse

- The wheel scrolls the pane under the mouse.
- In view mode, a click on a checkbox changes it, and a click on a link follows it. A double-click edits the line.
- In edit mode, a click puts the cursor, and a drag selects text.
- In the sidebar, a click selects and a double-click opens.

## Credits

The palettes of the themes come from [tideui](https://github.com/allisonhere/tideui) by Allie Bayless (MIT license).

## License

[MIT](LICENSE)
