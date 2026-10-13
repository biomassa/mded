# mded

mded is a markdown viewer and editor for the terminal.

> This software was developed with the assistance of a LLM.

## Features

- **View mode.** The document shows as formatted markdown: heading bars, lists, checkboxes, links, quotes, rules, code with syntax colors, and tables.
- **Edit mode with a live preview.** The line with the cursor shows its source. All other lines show formatted.
- **Two edit modes.** A conventional mode (as in most text editors) and a vim mode. `ctrl+g` changes between them.
- **Search with `/`**, as in neovim: regular expressions, smartcase, incremental search, and highlights of all matches.
- **Pictures** in kitty and Ghostty, through the kitty graphics protocol, and a picker that inserts pictures.
- **Sidebar** with a directory tree, a filter, and file operations.
- **Color themes**, with a theme picker.

## Installation

### Install Go

If you do not have Go, install it with the package manager of your system.

macOS ([Homebrew](https://brew.sh)):

```sh
brew install go
```

Debian and Ubuntu:

```sh
sudo apt install golang-go
```

Fedora, RHEL, and CentOS Stream:

```sh
sudo dnf install golang
```

Arch Linux and Manjaro:

```sh
sudo pacman -S go
```

openSUSE:

```sh
sudo zypper install go
```

Any Linux, with the official archive from [go.dev](https://go.dev/dl/) (use `arm64` in place of `amd64` on ARM):

```sh
curl -LO https://go.dev/dl/go1.27.1.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.27.1.linux-amd64.tar.gz
echo 'export PATH="$PATH:/usr/local/go/bin"' >> ~/.profile
```

Examine the version with `go version`. Some distributions have a Go version that is older than 1.27. Go 1.21 or later can download the necessary version itself. If `go install` fails because the version is too old, add `GOTOOLCHAIN=go1.27.1` before the command:

```sh
GOTOOLCHAIN=go1.27.1 go install -trimpath -ldflags="-s -w" github.com/biomassa/mded/cmd/mded@latest
```

### Install mded

Each [release](https://github.com/biomassa/mded/releases) has binaries for Linux and macOS (amd64 and arm64). You do not need Go for them. Download the archive for your system, then extract `mded` to a directory in your `PATH`:

```sh
tar -xzf mded_0.1.0_linux_amd64.tar.gz
install mded_0.1.0_linux_amd64/mded ~/.local/bin/
```

On macOS, a downloaded binary can get a quarantine flag. If macOS refuses to start it, remove the flag with `xattr -d com.apple.quarantine ~/.local/bin/mded`.

Or install the latest release with Go:

```sh
go install -trimpath -ldflags="-s -w" github.com/biomassa/mded/cmd/mded@latest
```

The flags remove the debug data, as in the release binaries. The program is then smaller, and it works the same.

Go puts the binary in `$(go env GOPATH)/bin`, usually `~/go/bin`. Make sure that this directory is in your `PATH`:

```sh
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.profile
```

To build from the source:

```sh
git clone https://github.com/biomassa/mded.git
cd mded
go build -trimpath -ldflags="-s -w" -o mded ./cmd/mded
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

In edit mode, the cursor is a blinking underscore. In vim normal mode and visual mode, it is a blinking block. The cursor has the accent color of the theme.

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
| `enter` | With no selected item and with the search highlights on: edit at the current match. |
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
- In view mode, `enter` opens edit mode with the cursor at the start of the current match.
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
| `ctrl+b`, `ctrl+i`, `ctrl+l`, `ctrl+t` | Bold, italic, link, checkbox. |
| `ctrl+k` | Delete the line. With a selection, delete all the lines of the selection. |
| `tab` | Insert spaces (`tab_width`). |
| `enter` | Start a new line. On a list item, start the next item. On an empty item, end the list. |
| `ctrl+f` | Find and replace. |
| `ctrl+p` | Insert a picture (see [Insert a picture](#insert-a-picture)). |
| `ctrl+s` | Save. |
| `esc` | Go back to view mode. |

The clipboard goes through the terminal (OSC 52). kitty can ask you to allow a paste the first time.

## Vim mode

Vim mode has normal, insert, replace, visual, and visual line modes. It has:

- Motions: `h j k l w W b B e E ge 0 ^ $ gg G f F t T ; , % H M L { }`, `ctrl+d ctrl+u ctrl+f ctrl+b`, `n N * #`, and marks.
- Operators: `d c y > < gu gU g~`, with motions and text objects (`iw aw iW aW is as ip ap i" a" i' a' i( a( i[ a[ i{ a{ i< a<` and others).
- Edits: `x X s S r J ~ p P i a I A o O R u ctrl+r .`, with counts.
- Registers: `"a`–`"z`, `"0`–`"9`, `"-`, `"_`, and `"+` / `"*` for the system clipboard. Each yank and delete also goes to the system clipboard.
- `o` and `O` on a list item start a new item.
- Marks (`m`) and macros (`q`, `@`).
- `ctrl+p` in normal mode and insert mode, or `:img`, inserts a picture.
- Commands: `:w`, `:w FILE`, `:q`, `:q!`, `:wq`, `:x`, `:e FILE`, `:noh`, `:N`, and `:s` with ranges and the flags `g`, `i`, `I`, and `c`.

The patterns of `/` and `:s` use the Go syntax: `(`, `)`, `|`, `+`, `?`, and `{` are special characters. `:s` also accepts the vim forms `\(`, `\)`, `\|`, `\<`, and `\>`. In the replacement, `\1`–`\9` and `&` insert the groups and the match.

Vim mode does not have visual block mode or `ctrl+o` in insert mode.

## Pictures

mded shows pictures in terminals with the kitty graphics protocol: kitty and Ghostty. mded does not show pictures in tmux.

- A picture is a reference `![alt](path)` to a local file. The path is relative to the directory of the document, or absolute. mded does not load pictures from the internet.
- The formats are PNG, JPEG, GIF (the first frame), and WebP.
- A picture shows at its natural size. If it is too large, mded makes it smaller to fit the width of the pane and `image_max_rows`.
- A picture alone on its line replaces the line. A picture in a line of text shows below the line.
- In edit mode, the line with the cursor shows its source, and its pictures do not show.
- Without picture support, mded shows `🖼` and the alt text, or the file name if the alt text is empty.

### Insert a picture

`ctrl+p` in edit mode opens the picture picker. In vim mode, `:img` also opens it.

- The picker starts in the directory of the document. It shows directories and picture files (PNG, JPEG, GIF, and WebP). The first line shows the directory.
- `↑` and `↓` move. `→` opens a directory, and `←` goes to the parent directory. Type text to filter the names. `enter` picks a file, and `esc` closes the picker.
- The right side of the picker shows the picture under the cursor. Without picture support, it shows the file size and the size in pixels.
- After you pick a file, type the alt text on the status line. The file name is the default. `enter` inserts the picture, and `esc` goes back to the picker.
- mded inserts the line `![alt](path)` below the cursor line. The path is relative to the directory of the document. In the path, a space becomes `%20`, and `%`, `(`, and `)` become `%25`, `%28`, and `%29`. The cursor stays on its line, so the picture shows at once. One undo removes the line.

## Sidebar

`ctrl+`` shows or hides the sidebar. `ctrl+e` moves the focus between the sidebar and the document, in all modes. `alt+h` moves the focus to the sidebar, and `alt+l` to the document.

| Key | Action |
|---|---|
| `↑`, `↓` | Move. |
| `→`, `enter` | Expand a directory, or open a file. On an expanded directory, go to its first entry. |
| `←` | Collapse a directory, or go to the parent entry. On the top row, show the parent directory. |
| `/` | Filter the names in the full tree. `enter` keeps the filter, and `esc` removes it. |
| `ctrl+n` | Make a new file. |
| `ctrl+shift+n` | Make a new directory. |
| `F2` | Rename. |
| `delete` | Delete. mded asks first. |
| `ctrl+h` | Show or hide hidden files. |

`ctrl+e` and `alt+h` also show a hidden sidebar.

The sidebar shows all files. Markdown files open formatted. Other text files open as plain text. Picture files show the picture. mded does not open other binary files.

Drag the border between the sidebar and the document to change the width. mded saves the width in the config file.

## Themes

`T` in view mode opens the theme picker. A move shows the theme. `enter` keeps it and saves it in the config file. `esc` goes back to the theme in use.

The themes are: default, catppuccin-mocha, catppuccin-latte, catppuccin-frappe, catppuccin-macchiato, nord, dracula, gruvbox-dark, gruvbox-light, tokyo-night, tokyo-night-day, rose-pine, rose-pine-moon, rose-pine-dawn, one-dark, magenta-geode, coral-sunset, lavender-fields-forever, vt100, and vt52. The `default` theme uses the colors of your terminal. The heading bars use the accent of the theme. Each lower level has a fainter bar. Levels 1 and 2 have bright bold text, levels 3 and 4 bold text, and levels 5 and 6 muted text.

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
- In view mode, a click on a checkbox changes it, and a click on a link follows it. A double-click edits the line. A double-click on a checkbox does not change it.
- In edit mode, a click puts the cursor, and a drag selects text. In vim mode, a drag starts visual mode.
- In the sidebar, a click selects and a double-click opens.

## Versions

mded uses [Semantic Versioning](https://semver.org). Before version 1.0.0, a minor version (0.2.0) can change keys, commands, and file formats. [CHANGELOG.md](CHANGELOG.md) lists the changes of each version. `mded --version` shows the version.

## Credits

The palettes of the themes come from [tideui](https://github.com/allisonhere/tideui) by Allie Bayless (MIT license).

## License

[MIT](LICENSE)
