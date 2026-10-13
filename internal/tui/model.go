// Package tui is the bubbletea user interface of mded: the layout, the document pane in
// view mode and edit mode, the sidebar, the status line, the prompts, and the mouse.
package tui

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/biomassa/mded/internal/config"
	"github.com/biomassa/mded/internal/keymap"
	"github.com/biomassa/mded/internal/keymap/conv"
	"github.com/biomassa/mded/internal/kitty"
	"github.com/biomassa/mded/internal/markdown"
	"github.com/biomassa/mded/internal/theme"
	"github.com/biomassa/mded/internal/tree"
)

// Options is what the command gives to the UI.
type Options struct {
	Config config.Config
	Path   string // FILE or DIR from the command line, "" for none
	Vim    bool   // --vim: the edit flavor starts as vim
	// NewVim makes the vim keymap. It is nil if vim is not available.
	NewVim func(tabWidth int) keymap.Keymap
	// Images is nil if images are off.
	Images *kitty.Manager
	// The next three are for tests. A nil value gives the real action.
	SaveTheme        func(name string) error
	SaveSidebarWidth func(w int) error
	OpenURL          func(url string) error
}

// focusPane is the pane that gets the keys.
type focusPane int

const (
	focusDoc focusPane = iota
	focusTree
)

// mode is the mode of the document pane.
type mode int

const (
	modeView mode = iota
	modeEdit
)

// statusTime is the time that a message stays in the status line.
const statusTime = 3 * time.Second

// Messages that the UI sends to itself.
type (
	msgExpiredMsg struct{ seq int }
	themeSavedMsg struct {
		name string
		err  error
	}
	widthSavedMsg struct{ err error }
)

// Model is the whole UI state. Its methods change the state in place and put commands in
// m.cmds. Update returns them in one batch.
type Model struct {
	opts  Options
	cfg   config.Config
	cache *markdown.Cache
	tree  *tree.Tree

	width, height int
	showTree      bool
	sidebarW      int
	focus         focusPane
	termDark      bool
	themeGen      int // counts theme changes, for the layout key

	doc  *doc
	hist []*doc // link history: the documents that a back key returns to
	mode mode

	vimFlavor bool
	convKm    keymap.Keymap
	vimKm     keymap.Keymap
	km        keymap.Keymap // the keymap of edit mode
	find      *conv.FindBox
	findLn    []string // the drawn lines of the find box
	follow    bool     // scroll to show the cursor at the next layout

	// View mode state.
	items    []markdown.Item
	itemsDoc *doc
	itemsRev int
	itemsOK  bool
	itemSel  int  // index of the selected item, -1 for none
	srchPos  bool // the buffer cursor holds the position of the last search jump
	reveal   revealReq

	// Prompts, dialogs, and overlays.
	search     *searchPrompt
	searchHist []string
	dlg        *dialog
	help       *helpState
	themes     *themePicker
	ctrlW      bool

	// The status line.
	msg    string
	msgErr bool
	msgSeq int

	// The layout, built at the end of each Update.
	rows   []markdown.Row
	lk     layoutKey
	laid   bool
	placed map[int]placedImage

	imgTried map[string]bool // image files that a load started for
	imgGen   int             // counts finished image loads, for the layout key

	// Mouse state.
	lastClick  clickInfo
	dragBorder bool
	dragSel    bool
	lastAct    clickAct // what the last click in the view did, for a double-click
	dragWidth  int

	cmds     []tea.Cmd
	quitting bool
}

// revealReq asks the layout to scroll to a source line.
type revealReq struct {
	line   int
	top    bool // put the line at the top of the pane, else only make it visible
	active bool
}

// New makes the Model for the command line options.
func New(o Options) (*Model, error) {
	cfg := o.Config
	m := &Model{
		opts:      o,
		cfg:       cfg,
		cache:     markdown.NewCache(),
		termDark:  true,
		itemSel:   -1,
		sidebarW:  cfg.SidebarWidth,
		vimFlavor: (o.Vim || cfg.DefaultMode == "vim") && o.NewVim != nil,
		imgTried:  map[string]bool{},
	}
	root, file := ".", ""
	if o.Path != "" {
		st, err := os.Stat(o.Path)
		switch {
		case err == nil && st.IsDir():
			root = o.Path
		case err == nil || errors.Is(err, fs.ErrNotExist):
			file, root = o.Path, filepath.Dir(o.Path)
		default:
			return nil, errors.New(errText(err))
		}
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if m.tree, err = tree.New(root, cfg.ShowHidden); err != nil {
		return nil, err
	}
	if file == "" {
		m.doc = blankDoc()
		m.showTree, m.focus = true, focusTree
		return m, nil
	}
	d, err := loadDoc(file)
	if err != nil {
		return nil, err
	}
	m.setDoc(d)
	return m, nil
}

// Cleanup returns the escape sequences that delete the images of this run from the
// terminal. The command writes them after the program ends. It is empty without images.
func (m *Model) Cleanup() string {
	if m.opts.Images == nil {
		return ""
	}
	return m.opts.Images.DeleteAll()
}

// Init asks the terminal for its background color.
func (m *Model) Init() tea.Cmd { return tea.RequestBackgroundColor }

// cmd queues a command for the end of the Update.
func (m *Model) cmd(c tea.Cmd) {
	if c != nil {
		m.cmds = append(m.cmds, c)
	}
}

// Update runs one message, then builds the layout, then returns the queued commands.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.update(msg)
	m.relayout()
	cmds := m.cmds
	m.cmds = nil
	return m, tea.Batch(cmds...)
}

func (m *Model) update(msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.BackgroundColorMsg:
		m.termDark = msg.IsDark()
		if theme.Current() != "default" {
			return // a palette theme paints its own background
		}
		theme.Apply("default", msg.IsDark())
		r, g, b, _ := msg.RGBA()
		theme.BaseBg = fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8)
		m.themeChanged()
	case msgExpiredMsg:
		if msg.seq == m.msgSeq {
			m.msg = ""
		}
	case themeSavedMsg:
		if msg.err != nil {
			m.status("theme "+msg.name+" is in use, but it was not saved: "+msg.err.Error(), true)
			return
		}
		m.status("theme "+msg.name+" · saved for the next start", false)
	case widthSavedMsg:
		if msg.err != nil {
			m.status("the sidebar width was not saved: "+msg.err.Error(), true)
		}
	case kitty.LoadedMsg:
		m.imgGen++
	case tea.PasteMsg:
		m.paste(msg.Content)
	case tea.ClipboardMsg:
		if m.mode == modeEdit && m.focus == focusDoc && m.find == nil {
			m.paste(msg.Content)
		}
	case tea.MouseClickMsg:
		m.mouseClick(msg.Mouse())
	case tea.MouseReleaseMsg:
		m.mouseRelease(msg.Mouse())
	case tea.MouseMotionMsg:
		m.mouseMotion(msg.Mouse())
	case tea.MouseWheelMsg:
		m.mouseWheel(msg.Mouse())
	case tea.KeyPressMsg:
		m.key(msg)
	}
}

// themeChanged clears what holds colored text, after a change of the theme.
func (m *Model) themeChanged() {
	m.cache.Reset()
	m.themeGen++
}

// status shows a message in the status line for three seconds.
func (m *Model) status(text string, isErr bool) {
	m.msg, m.msgErr = text, isErr
	m.msgSeq++
	seq := m.msgSeq
	m.cmd(tea.Tick(statusTime, func(time.Time) tea.Msg { return msgExpiredMsg{seq} }))
}

// ---- keys ----

// sidebarKeys are the key names of the sidebar toggle. A legacy terminal sends NUL for
// ctrl+backtick, and bubbletea names it ctrl+space or ctrl+@.
var sidebarKeys = map[string]bool{"ctrl+`": true, "ctrl+space": true, "ctrl+@": true}

// normal reports whether the vim keymap is in normal mode with nothing pending.
func (m *Model) normal() bool {
	if m.km == nil {
		return false
	}
	n, ok := m.km.(interface{ Normal() bool })
	return ok && n.Normal()
}

// promptOpen reports whether a prompt takes the typed keys.
func (m *Model) promptOpen() bool {
	if m.search != nil || m.find != nil {
		return true
	}
	if _, on := m.tree.Prompt(); on {
		return true
	}
	if m.mode == modeEdit && m.focus == focusDoc && m.km != nil {
		_, _, on := m.km.StatusLine()
		return on
	}
	return false
}

// key routes a key press. The order: dialog, quit, overlays, prompts, global keys, the pane.
func (m *Model) key(msg tea.KeyPressMsg) {
	k := msg.String()
	if m.dlg != nil {
		m.dialogKey(k)
		return
	}
	if k == "ctrl+q" {
		m.themes, m.help = nil, nil
		m.askQuit()
		return
	}
	if m.themes != nil {
		m.themeKey(k)
		return
	}
	if m.help != nil {
		m.helpKey(k)
		return
	}
	if m.search != nil {
		m.searchKey(msg)
		return
	}
	if k == "f1" {
		m.openHelp()
		return
	}
	prompt := m.promptOpen()
	if !prompt {
		switch {
		case sidebarKeys[k]:
			m.toggleTree()
			return
		case k == "alt+h":
			m.focusPane(focusTree)
			return
		case k == "alt+l":
			m.focusPane(focusDoc)
			return
		case k == "ctrl+g":
			m.switchFlavor()
			return
		}
		if m.ctrlW {
			m.ctrlW = false
			if k == "w" {
				if m.focus == focusTree {
					m.focusPane(focusDoc)
				} else {
					m.focusPane(focusTree)
				}
				return
			}
		}
		if k == "ctrl+w" && (m.focus == focusTree || m.mode == modeView || m.vimFlavor && m.normal()) {
			m.ctrlW = true
			return
		}
	}
	m.ctrlW = false
	switch {
	case m.focus == focusTree:
		m.treeKey(msg)
	case m.mode == modeView:
		m.viewKey(msg)
	default:
		m.editKey(msg)
	}
}

// paste handles pasted text and a clipboard answer.
func (m *Model) paste(text string) {
	switch {
	case m.dlg != nil || m.themes != nil || m.help != nil:
	case m.search != nil:
		m.search.insert(text)
		m.searchUpdate()
	case m.focus == focusTree:
		if _, on := m.tree.Prompt(); on {
			for _, r := range oneLine(text) { // the tree has no paste: send the runes as keys
				m.treeKey(tea.KeyPressMsg{Code: r, Text: string(r)})
			}
		}
	case m.mode == modeEdit && m.find != nil:
		for _, r := range oneLine(text) {
			m.editKey(tea.KeyPressMsg{Code: r, Text: string(r)})
		}
	case m.mode == modeEdit:
		m.apply(m.km.Paste(m.doc.buf, text))
		m.follow = true
	}
}

// oneLine makes pasted text fit a one-line field: line breaks and tabs become spaces.
func oneLine(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' {
			r = ' '
		}
		out = append(out, r)
	}
	return string(out)
}

// toggleTree shows or hides the sidebar. Showing it does not move the focus. Hiding it
// while it has the focus moves the focus to the document.
func (m *Model) toggleTree() {
	m.showTree = !m.showTree
	if !m.showTree && m.focus == focusTree {
		m.focus = focusDoc
	}
}

// focusPane moves the focus. A hidden sidebar shows up when it gets the focus.
func (m *Model) focusPane(p focusPane) {
	if p == focusTree {
		m.showTree = true
	}
	m.focus = p
}

// treeKey sends a key to the sidebar and runs its action.
func (m *Model) treeKey(msg tea.KeyPressMsg) {
	act, cmd := m.tree.HandleKey(msg)
	m.cmd(cmd)
	m.treeAction(act)
}

// treeAction runs the result of a key or a click in the sidebar.
func (m *Model) treeAction(a tree.Action) {
	switch a.Kind {
	case tree.Open:
		m.openFromTree(a.Path)
	case tree.Status:
		m.status(a.Text, false)
	case tree.Error:
		m.status(a.Text, true)
	}
}

// openFromTree opens a file of the sidebar. The focus moves to the document if it opens.
func (m *Model) openFromTree(path string) {
	if d, ok := m.load(path); ok {
		m.guard(func() {
			m.setDoc(d)
			m.focus = focusDoc
		})
	}
}

// ---- documents ----

// load reads a file and shows the error in the status line.
func (m *Model) load(path string) (*doc, bool) {
	d, err := loadDoc(path)
	if err != nil {
		m.status(err.Error(), true)
		return nil, false
	}
	return d, true
}

// setDoc shows d in view mode.
func (m *Model) setDoc(d *doc) {
	m.doc = d
	m.mode = modeView
	m.find, m.search = nil, nil
	m.itemSel, m.srchPos, m.laid = -1, false, false
	m.follow = false
	if d.abs != "" {
		m.tree.SetOpen(d.abs)
	}
}

// guard runs then now, or after the user decided about unsaved changes.
func (m *Model) guard(then func()) {
	if m.doc.buf.Dirty() {
		m.dlg = &dialog{then: then}
		return
	}
	then()
}

// askQuit quits, or asks about unsaved changes first.
func (m *Model) askQuit() { m.guard(m.quit) }

func (m *Model) quit() {
	m.quitting = true
	m.cmd(tea.Quit)
}

// pushHistory remembers the open document for the back key.
func (m *Model) pushHistory() {
	if m.doc.path == "" && !m.doc.buf.Dirty() && m.doc.buf.Text() == "" {
		return
	}
	m.hist = append(m.hist, m.doc)
	if len(m.hist) > 100 {
		m.hist = m.hist[1:]
	}
}

// back returns to the previous document of the link history.
func (m *Model) back() {
	if len(m.hist) == 0 {
		m.status("no previous document", false)
		return
	}
	prev := m.hist[len(m.hist)-1]
	m.guard(func() {
		m.hist = m.hist[:len(m.hist)-1]
		m.setDoc(prev)
	})
}

// save writes the document to its file. It reports whether it wrote.
func (m *Model) save() bool {
	d := m.doc
	if d.path == "" {
		m.status("no file name", true)
		return false
	}
	return m.write(d.path)
}

// saveAs writes the document to path. The document then belongs to that path.
func (m *Model) saveAs(path string) bool {
	if path == "" {
		return m.save()
	}
	return m.write(path)
}

func (m *Model) write(path string) bool {
	d := m.doc
	if d.image {
		return false
	}
	if err := writeFile(path, d.encode()); err != nil {
		m.status("could not save "+path+": "+errText(err), true)
		return false
	}
	if abs, err := filepath.Abs(path); err == nil {
		d.abs = abs
	}
	d.path, d.exists = path, true
	d.plain = !isMarkdown(path)
	d.buf.MarkSaved()
	m.tree.Refresh()
	m.tree.SetOpen(d.abs)
	m.laid = false
	m.status("saved "+path, false)
	return true
}

// ---- links ----

// followLink opens the target of a link.
func (m *Model) followLink(url string) {
	if hasScheme(url) {
		open := m.opts.OpenURL
		if open == nil {
			open = openURL
		}
		if err := open(url); err != nil {
			m.status("could not open "+url+": "+errText(err), true)
			return
		}
		m.status("opened "+url, false)
		return
	}
	file, anchor := splitAnchor(url)
	d := m.doc
	if file == "" { // #anchor alone: the current document
		if line := markdown.FindHeading(d.buf.Lines, anchor); line >= 0 {
			m.reveal = revealReq{line: line, top: true, active: true}
		} else {
			m.status("no heading: #"+anchor, true)
		}
		return
	}
	path := unescape(file)
	if !filepath.IsAbs(path) {
		dir := "."
		if d.abs != "" {
			dir = filepath.Dir(d.abs)
		}
		path = filepath.Join(dir, path)
	}
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		m.status("no such file: "+file, true)
		return
	}
	nd, ok := m.load(path)
	if !ok {
		return
	}
	m.guard(func() {
		m.pushHistory()
		m.setDoc(nd)
		if anchor != "" {
			if line := markdown.FindHeading(nd.buf.Lines, anchor); line >= 0 {
				m.reveal = revealReq{line: line, top: true, active: true}
			} else {
				m.status("no heading: #"+anchor, true)
			}
		}
	})
}

// ---- keymap ----

// currentKm returns the keymap of the edit flavor, and makes it if needed.
func (m *Model) currentKm() keymap.Keymap {
	if m.vimFlavor && m.opts.NewVim != nil {
		if m.vimKm == nil {
			m.vimKm = m.opts.NewVim(m.cfg.TabWidth)
		}
		return m.vimKm
	}
	if m.convKm == nil {
		m.convKm = conv.New(m.cfg.TabWidth)
	}
	return m.convKm
}

// switchFlavor changes the edit flavor between conventional and vim. In edit mode the
// old keymap is reset first, so that an open insert session ends as one undo step.
func (m *Model) switchFlavor() {
	if !m.vimFlavor && m.opts.NewVim == nil {
		m.status("vim mode is not available", true)
		return
	}
	if m.mode == modeEdit && m.km != nil {
		m.km.Reset()
	}
	m.vimFlavor = !m.vimFlavor
	if m.mode == modeEdit {
		m.doc.buf.ClearSel()
		m.km = m.currentKm()
		m.km.Reset()
	}
	if m.vimFlavor {
		m.status("edit mode: vim", false)
	} else {
		m.status("edit mode: conventional", false)
	}
}

// enterEdit opens edit mode. at is the cursor for a double-click, nil for the last cursor
// of the document.
func (m *Model) enterEdit(at *[2]int) {
	d := m.doc
	if d.path == "" { // a document without a file has no edit mode
		return
	}
	if d.image {
		m.status("image files open in view mode only", true)
		return
	}
	m.mode = modeEdit
	m.km = m.currentKm()
	m.km.Reset()
	b := d.buf
	b.ClearSel()
	b.BreakTyping()
	if at != nil {
		b.MoveTo(at[0], at[1])
	} else {
		b.MoveTo(d.editRow, d.editCol)
	}
	m.itemSel = -1
	m.follow = true
}

// leaveEdit returns to view mode. The view keeps the line at its top.
func (m *Model) leaveEdit() {
	if m.mode != modeEdit {
		return
	}
	d := m.doc
	top := m.topLine()
	d.editRow, d.editCol = d.buf.Row, d.buf.Col
	d.buf.ClearSel()
	m.km.Reset()
	m.find = nil
	m.mode = modeView
	m.itemSel, m.srchPos = -1, false
	m.reveal = revealReq{line: top, top: true, active: true}
}

// apply runs the effects of a keymap.
func (m *Model) apply(effects []keymap.Effect) {
	for _, e := range effects {
		switch e.Kind {
		case keymap.KindStatus:
			m.status(e.Text, e.Err)
		case keymap.KindSetClipboard:
			m.cmd(tea.SetClipboard(e.Text))
		case keymap.KindReadClipboard:
			m.cmd(tea.ReadClipboard)
		case keymap.KindSave:
			m.save()
		case keymap.KindSaveAs:
			m.saveAs(e.Path)
		case keymap.KindQuit:
			if m.doc.buf.Dirty() {
				m.status("unsaved changes · :q! discards · :wq saves", true)
			} else {
				m.quit()
			}
		case keymap.KindForceQuit:
			m.quit()
		case keymap.KindSaveQuit:
			if m.save() {
				m.quit()
			}
		case keymap.KindExitToView:
			m.leaveEdit()
		case keymap.KindEditFile:
			m.editFile(e.Path)
		case keymap.KindSwitchFlavor:
			m.switchFlavor()
		case keymap.KindOpenFindBox:
			m.openFind()
		case keymap.KindChanged:
			m.itemSel = -1
			if m.find != nil {
				m.find.Refresh(m.doc.buf)
			}
		}
	}
}

// editFile opens a file for the vim :e command. Edit mode stays on.
func (m *Model) editFile(path string) {
	if path == "" {
		return
	}
	d, ok := m.load(path)
	if !ok {
		return
	}
	m.guard(func() {
		m.setDoc(d)
		m.enterEdit(nil)
	})
}

// openFind opens the find and replace box of the conventional edit mode.
func (m *Model) openFind() {
	if m.find == nil {
		m.find = conv.NewFindBox()
	}
	m.cmd(m.find.Open(m.doc.buf, m.docRect().w-2))
}

// editKey sends a key to the find box or to the keymap.
func (m *Model) editKey(msg tea.KeyPressMsg) {
	b := m.doc.buf
	m.follow = true
	if m.find != nil {
		cmd, effects, closed := m.find.HandleKey(b, msg)
		m.cmd(cmd)
		if closed {
			m.find = nil
		}
		m.apply(effects)
		return
	}
	k := msg.String()
	if k == "ctrl+s" && m.vimFlavor && m.normal() {
		m.save()
		return
	}
	m.apply(m.km.Handle(b, keymap.Key{Name: k, Text: msg.Text}, m.textW(), m.textH()))
}

// topLine returns the source line at the top of the pane.
func (m *Model) topLine() int {
	for i := m.doc.off; i < len(m.rows); i++ {
		if m.rows[i].Img == nil {
			return m.rows[i].Line
		}
	}
	if len(m.rows) > 0 {
		return m.rows[min(m.doc.off, len(m.rows)-1)].Line
	}
	return 0
}
