# Changelog

This file records all notable changes to this project.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- View mode: the document shows as formatted markdown. Headings show as colored bars. Lists, checkboxes, links, quotes, rules, code, and tables show formatted.
- Edit mode with a live preview. The line with the cursor shows its source. All other lines show formatted.
- Two edit modes: a conventional mode and a vim mode. `ctrl+g` changes between them.
- Search with `/`, as in neovim: regular expressions, smartcase, incremental search, and highlights of all matches.
- Pictures in kitty and Ghostty, through the kitty graphics protocol.
- A picture picker (`ctrl+p`, or `:img` in vim mode). It inserts a reference to the picture below the cursor line.
- A sidebar with a directory tree. It can create, rename, and delete files and directories.
- Start with a file (`mded FILE`), a directory (`mded DIR`), or the current directory (`mded`).
- Color themes from godoist, with a theme picker (`T`).
- A config file: `~/.config/mded/config.toml`.
