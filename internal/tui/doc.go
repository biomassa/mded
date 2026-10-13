package tui

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/biomassa/mded/internal/buffer"
)

// binaryProbe is the number of bytes that the binary check reads. A NUL byte in them
// makes the file a binary file.
const binaryProbe = 8000

// doc is one open document: its text buffer and the facts about its file.
type doc struct {
	path    string         // the path as the user opened it, "" for a document without a file
	abs     string         // the absolute path, "" for a document without a file
	buf     *buffer.Buffer // the text, with LF line ends and without the final line end
	plain   bool           // not a markdown file: no preview
	image   bool           // an image file: the pane shows the picture
	crlf    bool           // the file had CRLF line ends
	finalNL bool           // the file ended with a line end
	exists  bool           // the file exists on disk
	// editRow and editCol are the cursor of the last edit session. View mode moves the
	// buffer cursor for the search, so the edit position needs its own place.
	editRow, editCol int
	off              int // first visible screen row
}

// markdownExt lists the file extensions that open as markdown.
var markdownExt = map[string]bool{".md": true, ".markdown": true, ".mdown": true, ".mkd": true}

// imageExt lists the file extensions that open in the image viewer.
var imageExt = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true}

func isMarkdown(path string) bool { return markdownExt[strings.ToLower(filepath.Ext(path))] }
func isImage(path string) bool    { return imageExt[strings.ToLower(filepath.Ext(path))] }

// title returns the text for the title of the document pane.
func (d *doc) title() string {
	if d.path == "" {
		return "[new]"
	}
	return filepath.Base(d.path)
}

// blank returns the empty document of a start without a file.
func blankDoc() *doc { return &doc{buf: buffer.New(""), finalNL: true} }

// loadDoc reads a file. A missing file gives an empty document with that path. A binary
// file gives an error.
func loadDoc(path string) (*doc, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	d := &doc{path: path, abs: abs, plain: !isMarkdown(path)}
	st, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		d.buf, d.finalNL = buffer.New(""), true
		return d, nil
	case err != nil:
		return nil, errors.New(errText(err))
	case st.IsDir():
		return nil, fmt.Errorf("%s is a directory", filepath.Base(path))
	}
	d.exists = true
	if isImage(path) { // images hold NUL bytes, so this check comes first
		d.image, d.buf = true, buffer.New("")
		return d, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New(errText(err))
	}
	if bytes.IndexByte(data[:min(len(data), binaryProbe)], 0) >= 0 {
		return nil, fmt.Errorf("binary file: %s", filepath.Base(path))
	}
	text := string(data)
	if i := strings.IndexByte(text, '\n'); i > 0 && text[i-1] == '\r' {
		d.crlf = true
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if strings.HasSuffix(text, "\n") {
		d.finalNL = true
		text = text[:len(text)-1]
	}
	d.buf = buffer.New(text)
	return d, nil
}

// errText makes a short text of a file system error.
func errText(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}

// encode returns the bytes of the file: the text with the line ends of the original.
func (d *doc) encode() []byte {
	text := d.buf.Text()
	if d.finalNL && (text != "" || d.exists) {
		text += "\n"
	}
	if d.crlf {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	return []byte(text)
}

// writeFile writes data to path in one atomic step: a temporary file in the same
// directory, then a rename. The file keeps its mode. A new file gets 0644. A symbolic
// link stays a link: the write goes to its target.
func writeFile(path string, data []byte) error {
	target := path
	if r, err := filepath.EvalSymlinks(path); err == nil {
		target = r
	}
	perm := fs.FileMode(0o644)
	if st, err := os.Stat(target); err == nil {
		perm = st.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".mded-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	fail := func(err error) error {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return fail(err)
	}
	if err := tmp.Chmod(perm); err != nil {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, target); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
