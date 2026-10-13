// Package tree is the directory browser of the sidebar: the tree model, its keys, the prompts,
// and the rows it draws.
package tree

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/biomassa/mded/internal/theme"
	"github.com/biomassa/mded/internal/ui"
)

// ActionKind tells the caller what to do after a key or a click.
type ActionKind int

// The kinds of action.
const (
	None   ActionKind = iota
	Open              // open the file in Path
	Status            // show Text in the status line
	Error             // show Text as an error
	Opened            // reserved
)

// Action is the result of a key or a click.
type Action struct {
	Kind       ActionKind
	Path, Text string
}

// filterLimit is the maximum number of entries that a filter walk reads.
const filterLimit = 20000

// mode is the input state of the tree.
type mode int

const (
	modeTree   mode = iota
	modeFilter      // the filter input is open
	modeNewFile
	modeNewDir
	modeRename
	modeDelete
)

// node is one entry of the tree.
type node struct {
	name     string
	path     string
	isDir    bool
	expanded bool
	loaded   bool // the children are read
	children []*node
	parent   *node
}

// hidden reports whether the entry is a hidden one.
func (n *node) hidden() bool { return strings.HasPrefix(n.name, ".") }

// row is one visible line of the tree.
type row struct {
	n     *node
	depth int
}

// Tree is the sidebar directory browser.
type Tree struct {
	root       *node
	showHidden bool

	filter     string // the active filter text, or ""
	filterRoot *node  // the filtered copy of the tree, or nil

	rows       []row // the cache of visible rows
	dirty      bool
	cursor     int
	cursorPath string
	top        int
	height     int  // the height of the last View
	follow     bool // the next View scrolls to the cursor

	openPath string

	mode   mode
	input  textinput.Model
	target string // the entry that a prompt or dialog works on
}

// New returns a tree for the directory root.
func New(root string, showHidden bool) (*Tree, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}
	t := &Tree{showHidden: showHidden, dirty: true, follow: true}
	t.root = &node{name: filepath.Base(abs), path: abs, isDir: true, expanded: true}
	t.cursorPath = abs
	_ = t.load(t.root)
	ti := textinput.New()
	s := textinput.DefaultStyles(theme.Dark())
	s.Cursor.Blink = false
	ti.SetStyles(s)
	ti.Prompt = ""
	t.input = ti
	return t, nil
}

// Root returns the root directory.
func (t *Tree) Root() string { return t.root.path }

// Hints returns the key hints of the sidebar.
func (t *Tree) Hints() string {
	return "→ open · / filter · ctrl+n new · F2 rename · del delete · ctrl+h hidden"
}

// Title returns the root path for the pane title.
func (t *Tree) Title() string {
	p := t.root.path
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if p == home {
			p = "~"
		} else if strings.HasPrefix(p, home+string(filepath.Separator)) {
			p = "~" + p[len(home):]
		}
	}
	if t.filter != "" {
		p += " · /" + t.filter
	}
	return p
}

// readDir reads the entries of a directory, sorted: directories first, then by lower-case name.
func (t *Tree) readDir(dir string, parent *node) ([]*node, error) {
	ents, err := os.ReadDir(dir)
	var out []*node
	for _, e := range ents {
		name := e.Name()
		if !t.showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		isDir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 {
			if st, err := os.Stat(filepath.Join(dir, name)); err == nil {
				isDir = st.IsDir()
			}
		}
		out = append(out, &node{name: name, path: filepath.Join(dir, name), isDir: isDir, parent: parent})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.isDir != b.isDir {
			return a.isDir
		}
		la, lb := strings.ToLower(a.name), strings.ToLower(b.name)
		if la != lb {
			return la < lb
		}
		return a.name < b.name
	})
	return out, err
}

// load reads the children of a directory node.
func (t *Tree) load(n *node) error {
	kids, err := t.readDir(n.path, n)
	n.children, n.loaded = kids, true
	return err
}

// reload reads the expanded directories again. It keeps the state of the entries that remain.
func (t *Tree) reload(n *node) {
	old := map[string]*node{}
	for _, c := range n.children {
		old[c.name] = c
	}
	_ = t.load(n)
	for i, c := range n.children {
		o := old[c.name]
		if o == nil || o.isDir != c.isDir {
			continue
		}
		n.children[i] = o
		if o.isDir && o.expanded {
			t.reload(o)
		} else if o.isDir {
			o.children, o.loaded = nil, false
		}
	}
}

// visRoot returns the root of the visible tree.
func (t *Tree) visRoot() *node {
	if t.filterRoot != nil {
		return t.filterRoot
	}
	return t.root
}

// flatten adds the visible rows of n to the cache.
func (t *Tree) flatten(n *node, depth int) {
	t.rows = append(t.rows, row{n, depth})
	if n.isDir && n.expanded {
		for _, c := range n.children {
			t.flatten(c, depth+1)
		}
	}
}

// vis returns the visible rows. It builds them again only after a change.
func (t *Tree) vis() []row {
	if !t.dirty {
		return t.rows
	}
	t.rows = t.rows[:0]
	t.flatten(t.visRoot(), 0)
	t.dirty = false
	t.cursor = t.indexOf(t.cursorPath)
	t.cursorPath = t.rows[t.cursor].n.path
	t.follow = true
	return t.rows
}

// indexOf finds the row of a path. It falls back to the nearest parent that is visible.
func (t *Tree) indexOf(path string) int {
	for p := path; ; p = filepath.Dir(p) {
		for i, r := range t.rows {
			if r.n.path == p {
				return i
			}
		}
		if filepath.Dir(p) == p {
			return 0
		}
	}
}

// current returns the node under the cursor.
func (t *Tree) current() *node {
	rows := t.vis()
	return rows[t.cursor].n
}

// moveTo puts the cursor on row i.
func (t *Tree) moveTo(i int) {
	rows := t.vis()
	i = max(0, min(i, len(rows)-1))
	t.cursor = i
	t.cursorPath = rows[i].n.path
	t.follow = true
}

// reveal expands the parents of path and returns the deepest entry that it found.
func (t *Tree) reveal(path string) *node {
	rel, err := filepath.Rel(t.root.path, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil
	}
	n := t.root
	if rel == "." {
		return n
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if !n.loaded {
			_ = t.load(n)
		}
		var next *node
		for _, c := range n.children {
			if c.name == part {
				next = c
				break
			}
		}
		if next == nil {
			break
		}
		n.expanded = true
		n = next
	}
	t.dirty = true
	return n
}

// SetOpen marks the open file and reveals it.
func (t *Tree) SetOpen(path string) {
	t.openPath = path
	if n := t.reveal(path); n != nil {
		if t.filterRoot == nil {
			t.cursorPath = n.path
		} else if t.hasPath(path) {
			t.cursorPath = path
		}
	}
	t.dirty = true
	t.vis()
}

// hasPath reports whether the filtered tree has an entry for path.
func (t *Tree) hasPath(path string) bool {
	var walk func(n *node) bool
	walk = func(n *node) bool {
		if n.path == path {
			return true
		}
		for _, c := range n.children {
			if walk(c) {
				return true
			}
		}
		return false
	}
	return t.filterRoot != nil && walk(t.filterRoot)
}

// Refresh reads the directories again. It keeps the expansion and the cursor.
func (t *Tree) Refresh() {
	t.reload(t.root)
	t.rebuildFilter()
	t.dirty = true
	t.vis()
}

// buildFilter walks the whole tree under the root and keeps the matches and their parents.
func (t *Tree) buildFilter() {
	q := strings.ToLower(t.filter)
	count := 0
	var walk func(n *node) bool
	walk = func(n *node) bool {
		kids, _ := t.readDir(n.path, n)
		keep := []*node{}
		for _, c := range kids {
			count++
			if count > filterLimit {
				break
			}
			match := strings.Contains(strings.ToLower(c.name), q)
			sub := false
			if c.isDir && !isLink(c.path) && count <= filterLimit {
				sub = walk(c)
			}
			if match || sub {
				c.loaded, c.expanded = true, c.isDir
				keep = append(keep, c)
			}
		}
		n.children, n.loaded = keep, true
		return len(keep) > 0
	}
	r := &node{name: t.root.name, path: t.root.path, isDir: true, expanded: true}
	walk(r)
	t.filterRoot = r
}

// isLink reports whether path is a symbolic link.
func isLink(path string) bool {
	st, err := os.Lstat(path)
	return err == nil && st.Mode()&os.ModeSymlink != 0
}

// rebuildFilter makes the filtered tree again when a filter is active.
func (t *Tree) rebuildFilter() {
	if t.filter == "" {
		t.filterRoot = nil
		return
	}
	t.buildFilter()
}

// setFilter sets the filter text and builds the filtered tree.
func (t *Tree) setFilter(s string) {
	if s == t.filter {
		return
	}
	t.filter = s
	t.rebuildFilter()
	t.dirty = true
	t.vis()
}

// clearFilter ends the filter. The cursor stays on its entry.
func (t *Tree) clearFilter() {
	if t.filter == "" {
		return
	}
	t.filter, t.filterRoot = "", nil
	t.reveal(t.cursorPath)
	t.dirty = true
	t.vis()
}

// Scroll moves the view by d rows.
func (t *Tree) Scroll(d int) {
	n := len(t.vis())
	t.top = max(0, min(t.top+d, n-max(t.height, 1)))
	t.follow = false
}

// Click selects row y of the last View. A double click acts like enter.
func (t *Tree) Click(y int, double bool) Action {
	if t.mode != modeTree {
		return Action{}
	}
	i := t.top + y
	rows := t.vis()
	if y < 0 || i >= len(rows) {
		return Action{}
	}
	t.moveTo(i)
	t.follow = false
	if double {
		return t.enter()
	}
	return Action{}
}

// Prompt returns the status line text of an open prompt, filter, or dialog.
func (t *Tree) Prompt() (string, bool) {
	switch t.mode {
	case modeFilter:
		return "/" + t.input.View(), true
	case modeNewFile:
		return "new file: " + t.input.View(), true
	case modeNewDir:
		return "new directory: " + t.input.View(), true
	case modeRename:
		return "rename to: " + t.input.View(), true
	case modeDelete:
		name := filepath.Base(t.target)
		q := "Delete " + name + "?"
		if st, err := os.Lstat(t.target); err == nil && st.IsDir() {
			q = "Delete " + name + " and all its content?"
		}
		return q + "  y delete · esc cancel", true
	}
	return "", false
}

// openInput opens a text prompt with a label mode and a start value.
func (t *Tree) openInput(m mode, value string) {
	t.mode = m
	t.input.SetValue(value)
	t.input.CursorEnd()
	t.input.Focus()
}

// closeInput ends the prompt.
func (t *Tree) closeInput() {
	t.mode = modeTree
	t.input.Blur()
	t.input.SetValue("")
}

// HandleKey handles a key while the sidebar has the focus.
func (t *Tree) HandleKey(msg tea.KeyPressMsg) (Action, tea.Cmd) {
	switch t.mode {
	case modeFilter:
		return t.filterKey(msg)
	case modeNewFile, modeNewDir, modeRename:
		return t.promptKey(msg)
	case modeDelete:
		return t.deleteKey(msg), nil
	}
	rows := t.vis()
	switch msg.String() {
	case "up":
		t.moveTo(t.cursor - 1)
	case "down":
		t.moveTo(t.cursor + 1)
	case "right":
		return t.enter(), nil
	case "enter":
		return t.enter(), nil
	case "left":
		t.left()
	case "/":
		t.openInput(modeFilter, t.filter)
	case "esc":
		t.clearFilter()
	case "ctrl+h":
		t.toggleHidden()
	case "ctrl+n":
		t.target = t.dirUnderCursor()
		t.openInput(modeNewFile, "")
	case "ctrl+shift+n":
		t.target = t.dirUnderCursor()
		t.openInput(modeNewDir, "")
	case "f2":
		n := rows[t.cursor].n
		if n == t.visRoot() {
			return Action{Kind: Error, Text: "cannot rename the root"}, nil
		}
		t.target = n.path
		t.openInput(modeRename, n.name)
	case "delete":
		n := rows[t.cursor].n
		if n == t.visRoot() {
			return Action{Kind: Error, Text: "cannot delete the root"}, nil
		}
		t.target = n.path
		t.mode = modeDelete
	}
	return Action{}, nil
}

// enter expands a directory or opens a file.
func (t *Tree) enter() Action {
	n := t.current()
	if !n.isDir {
		return Action{Kind: Open, Path: n.path}
	}
	if n.expanded {
		return Action{}
	}
	return t.expand(n)
}

// expand opens a directory node.
func (t *Tree) expand(n *node) Action {
	var err error
	if !n.loaded {
		err = t.load(n)
	}
	n.expanded = true
	t.dirty = true
	t.vis()
	if err != nil {
		return Action{Kind: Error, Text: errText(err)}
	}
	return Action{}
}

// left collapses a directory, or moves to the parent entry. On the root it moves the root up.
func (t *Tree) left() {
	n := t.current()
	if n == t.visRoot() {
		t.rootUp()
		return
	}
	if n.isDir && n.expanded {
		n.expanded = false
		t.dirty = true
		t.vis()
		return
	}
	if n.parent != nil {
		t.cursorPath = n.parent.path
		t.dirty = true
		t.vis()
	}
}

// rootUp makes the parent directory the root.
func (t *Tree) rootUp() {
	old := t.root
	parent := filepath.Dir(old.path)
	if parent == old.path {
		return
	}
	r := &node{name: filepath.Base(parent), path: parent, isDir: true, expanded: true}
	if parent == string(filepath.Separator) {
		r.name = parent
	}
	_ = t.load(r)
	for i, c := range r.children {
		if c.path == old.path {
			r.children[i] = old
			old.parent = r
		}
	}
	old.expanded = true
	t.root = r
	t.cursorPath = old.path
	t.rebuildFilter()
	t.dirty = true
	t.vis()
}

// toggleHidden shows or hides the hidden entries.
func (t *Tree) toggleHidden() {
	t.showHidden = !t.showHidden
	t.reload(t.root)
	t.rebuildFilter()
	t.dirty = true
	t.vis()
}

// dirUnderCursor returns the directory under the cursor, or the directory of the file under it.
func (t *Tree) dirUnderCursor() string {
	n := t.current()
	if n.isDir {
		return n.path
	}
	return filepath.Dir(n.path)
}

// filterKey handles the keys of the filter input.
func (t *Tree) filterKey(msg tea.KeyPressMsg) (Action, tea.Cmd) {
	switch msg.String() {
	case "esc":
		t.closeInput()
		t.clearFilter()
		return Action{}, nil
	case "enter":
		t.closeInput()
		return Action{}, nil
	case "up":
		t.moveTo(t.cursor - 1)
		return Action{}, nil
	case "down":
		t.moveTo(t.cursor + 1)
		return Action{}, nil
	}
	var cmd tea.Cmd
	t.input, cmd = t.input.Update(msg)
	if v := t.input.Value(); v == "" {
		t.clearFilter()
	} else {
		t.setFilter(v)
	}
	return Action{}, cmd
}

// validName reports whether name is a legal entry name.
func validName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.Contains(name, "/")
}

// errText makes a short text of a file system error.
func errText(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		err = pe.Err
	}
	var le *os.LinkError
	if errors.As(err, &le) {
		err = le.Err
	}
	return err.Error()
}

// promptKey handles the keys of the new and rename prompts.
func (t *Tree) promptKey(msg tea.KeyPressMsg) (Action, tea.Cmd) {
	switch msg.String() {
	case "esc":
		t.closeInput()
		return Action{}, nil
	case "enter":
		name := t.input.Value()
		m := t.mode
		if !validName(name) {
			return Action{Kind: Error, Text: "bad name: " + name}, nil
		}
		t.closeInput()
		return t.commit(m, name), nil
	}
	var cmd tea.Cmd
	t.input, cmd = t.input.Update(msg)
	return Action{}, cmd
}

// commit creates or renames the entry.
func (t *Tree) commit(m mode, name string) Action {
	var path string
	if m == modeRename {
		path = filepath.Join(filepath.Dir(t.target), name)
		if path == t.target {
			return Action{}
		}
	} else {
		path = filepath.Join(t.target, name)
	}
	if _, err := os.Lstat(path); err == nil {
		return Action{Kind: Error, Text: name + " exists"}
	}
	var err error
	switch m {
	case modeNewFile:
		var f *os.File
		if f, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644); err == nil {
			err = f.Close()
		}
	case modeNewDir:
		err = os.Mkdir(path, 0o755)
	case modeRename:
		err = os.Rename(t.target, path)
	}
	if err != nil {
		return Action{Kind: Error, Text: errText(err)}
	}
	t.filter, t.filterRoot = "", nil
	t.reload(t.root)
	t.reveal(path)
	t.cursorPath = path
	t.dirty = true
	t.vis()
	if m == modeNewFile {
		return Action{Kind: Open, Path: path}
	}
	return Action{}
}

// deleteKey handles the keys of the delete dialog. Only y deletes.
func (t *Tree) deleteKey(msg tea.KeyPressMsg) Action {
	t.mode = modeTree
	if msg.String() != "y" {
		return Action{}
	}
	idx := t.cursor
	var err error
	if st, e := os.Lstat(t.target); e == nil && st.IsDir() {
		err = os.RemoveAll(t.target)
	} else {
		err = os.Remove(t.target)
	}
	t.Refresh()
	t.moveTo(idx)
	if err != nil {
		return Action{Kind: Error, Text: errText(err)}
	}
	return Action{Kind: Status, Text: "deleted " + filepath.Base(t.target)}
}

// View draws h lines of w cells.
func (t *Tree) View(w, h int, focused bool) []string {
	rows := t.vis()
	t.height = h
	if t.follow {
		if t.cursor < t.top {
			t.top = t.cursor
		} else if t.cursor >= t.top+h {
			t.top = t.cursor - h + 1
		}
		t.follow = false
	}
	t.top = max(0, min(t.top, len(rows)-h))
	out := make([]string, h)
	for i := range out {
		idx := t.top + i
		if idx >= len(rows) {
			out[i] = strings.Repeat(" ", max(w, 0))
			continue
		}
		out[i] = t.line(rows[idx], w, idx == t.cursor, focused)
	}
	return out
}

// line draws one row.
func (t *Tree) line(r row, w int, selected, focused bool) string {
	base := lipgloss.NewStyle()
	if selected {
		bg := theme.SelBgDim
		if focused {
			bg = theme.SelBg
		}
		base = base.Background(theme.C(bg))
	}
	marker := "  "
	name := r.n.name
	nameSt := base.Foreground(theme.C(theme.Text))
	if r.n.isDir {
		marker = "▸ "
		if r.n.expanded {
			marker = "▾ "
		}
		name += "/"
		nameSt = nameSt.Bold(true)
	}
	switch {
	case r.n.path == t.openPath && !r.n.isDir:
		nameSt = base.Foreground(theme.C(theme.Accent))
	case r.n.hidden():
		nameSt = base.Foreground(theme.C(theme.Dim))
	}
	s := base.Render(strings.Repeat(" ", 2*r.depth)) +
		base.Foreground(theme.C(theme.Muted)).Render(marker) + nameSt.Render(name)
	return ui.Pad(s, w, base)
}
