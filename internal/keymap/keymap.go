// Package keymap holds the types that the keymaps share: the key, the effects, and the
// Keymap interface. The conventional keymap (package conv) and the vim engine implement it.
//
// A keymap changes a buffer and returns effects. The UI runs the effects: it shows
// messages, talks to the clipboard, saves the file, and so on. A keymap never does I/O.
package keymap

import "github.com/biomassa/mded/internal/buffer"

// Key is one key press. The UI makes it from a tea.KeyPressMsg.
type Key struct {
	Name string // msg.String(), for example "ctrl+s", "a", "shift+left", "space", "enter"
	Text string // msg.Text: the text that the key types, empty for keys such as "left"
}

// IsTyped reports whether the key types text: it has text, and ctrl, alt, or super are
// not down. Name starts with the modifier names then.
func IsTyped(k Key) bool {
	if k.Text == "" {
		return false
	}
	for _, p := range []string{"ctrl+", "alt+", "super+", "meta+", "hyper+"} {
		if len(k.Name) >= len(p) && k.Name[:len(p)] == p {
			return false
		}
	}
	return true
}

// Kind is the type of an effect.
type Kind int

// The effects. Each one uses the fields of Effect that its comment names.
const (
	KindStatus        Kind = iota // show Text in the status line (an error if Err)
	KindSetClipboard              // write Text to the system clipboard
	KindReadClipboard             // read the system clipboard. The text comes back through Keymap.Paste.
	KindSave                      // save the file
	KindSaveAs                    // save the file as Path
	KindQuit                      // quit. The UI refuses or asks if the text is unsaved.
	KindForceQuit                 // quit and discard unsaved changes
	KindSaveQuit                  // save, then quit
	KindExitToView                // leave edit mode
	KindEditFile                  // open the file Path
	KindSwitchFlavor              // switch between the conventional and the vim keymap
	KindOpenFindBox               // open the find and replace box
	KindChanged                   // the text of the document changed
)

// Effect is something that the UI must do after a key.
type Effect struct {
	Kind Kind
	Text string // KindStatus, KindSetClipboard
	Err  bool   // KindStatus: show the text in the error color
	Path string // KindSaveAs, KindEditFile
}

// Status returns an effect that shows a message in the status line.
func Status(text string, isErr bool) Effect { return Effect{Kind: KindStatus, Text: text, Err: isErr} }

// SetClipboard returns an effect that writes text to the system clipboard.
func SetClipboard(text string) Effect { return Effect{Kind: KindSetClipboard, Text: text} }

// ReadClipboard returns an effect that asks for the system clipboard.
func ReadClipboard() Effect { return Effect{Kind: KindReadClipboard} }

// Save returns an effect that saves the file.
func Save() Effect { return Effect{Kind: KindSave} }

// SaveAs returns an effect that saves the file as path.
func SaveAs(path string) Effect { return Effect{Kind: KindSaveAs, Path: path} }

// Quit returns an effect that quits.
func Quit() Effect { return Effect{Kind: KindQuit} }

// ForceQuit returns an effect that quits and discards changes.
func ForceQuit() Effect { return Effect{Kind: KindForceQuit} }

// SaveQuit returns an effect that saves and then quits.
func SaveQuit() Effect { return Effect{Kind: KindSaveQuit} }

// ExitToView returns an effect that leaves edit mode.
func ExitToView() Effect { return Effect{Kind: KindExitToView} }

// EditFile returns an effect that opens a file.
func EditFile(path string) Effect { return Effect{Kind: KindEditFile, Path: path} }

// SwitchFlavor returns an effect that switches the edit flavor.
func SwitchFlavor() Effect { return Effect{Kind: KindSwitchFlavor} }

// OpenFindBox returns an effect that opens the find and replace box.
func OpenFindBox() Effect { return Effect{Kind: KindOpenFindBox} }

// Changed returns an effect that tells the UI that the text changed.
func Changed() Effect { return Effect{Kind: KindChanged} }

// Has reports whether effects holds an effect of kind k.
func Has(effects []Effect, k Kind) bool {
	for _, e := range effects {
		if e.Kind == k {
			return true
		}
	}
	return false
}

// Keymap turns keys into changes of a buffer. A keymap can keep state between keys. The
// vim engine keeps its mode, the pending operator, the count, the registers, the marks, the
// macros, and the text of the : and / prompts. The buffer holds the document state: text,
// cursor, selection, undo, and search.
type Keymap interface {
	// Handle processes one key. width is the text width for screen-line moves, and pageRows
	// is the number of visible text rows. A keymap appends KindChanged to the effects if the
	// key changed the text, and it keeps the search matches of the buffer up to date.
	Handle(b *buffer.Buffer, k Key, width, pageRows int) []Effect

	// Paste inserts text that arrives from the terminal. It is a bracketed paste, or the
	// answer to a KindReadClipboard effect. The vim engine decides what the text means.
	Paste(b *buffer.Buffer, text string) []Effect

	// ModeLabel is the label for the status line: "EDIT", "NORMAL", "INSERT", and so on.
	ModeLabel() string

	// Hints is the key hint text for the status line.
	Hints() string

	// StatusLine returns the text of a prompt that the keymap shows in the status line, such
	// as ":w" or "/text" or the question of :s with the flag c. cursorCol is the cursor
	// position in runes in text. active is false if there is no prompt. The UI shows the
	// prompt in place of the hints and the messages.
	StatusLine() (text string, cursorCol int, active bool)

	// Reset returns the keymap to its start state: no pending key, no prompt. The UI calls
	// it when it enters edit mode and when it switches the flavor. The vim engine keeps its
	// registers, marks, and macros. Reset has no buffer argument. An engine that must close
	// an open undo group (an insert session) keeps the buffer of its last Handle call.
	Reset()
}
