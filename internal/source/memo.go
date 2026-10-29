package source

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Memo reads documents from disk and keeps what it read, under the version of
// the file it read: its size and the moment it was last written. A request asks
// for the same file twice, or the next request asks for it again, and the file is
// not opened unless its version moved. A file that changed is read again on the
// next answer: nothing is held for a time (d-e00b).
//
// A Memo is safe for concurrent use: hergy serve answers several requests at
// once. One Memo is one file system's reading; a serve builds one and hands it
// to every project's catalog, so a shared profiles.yaml is read once.
type Memo struct {
	mu    sync.Mutex
	files map[string]memoFile
	dirs  map[string]memoDir
	// reads and lists are the times a file was opened and a directory listed,
	// and hits the times a held version answered instead. A test counts them.
	reads, lists, hits int
}

type memoFile struct {
	size  int64
	mod   int64
	text  string
	exist bool
}

type memoDir struct {
	mod   int64
	names []string
}

// NewMemo is an empty reading.
func NewMemo() *Memo {
	return &Memo{files: map[string]memoFile{}, dirs: map[string]memoDir{}}
}

// version is one file's version: its size and modification time. A file that
// is not there is told apart from one that is.
func version(path string) (size, mod int64, exist bool) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, 0, false
	}
	return info.Size(), info.ModTime().UnixNano(), true
}

// Read returns a document's text, from the held copy when the file's version is
// the one it was read at.
func (m *Memo) Read(d Doc) (string, error) {
	size, mod, exist := version(d.Path)
	m.mu.Lock()
	cached, ok := m.files[d.Path]
	if ok && cached.exist == exist && cached.size == size && cached.mod == mod {
		m.hits++
		m.mu.Unlock()
		if !exist {
			return "", &os.PathError{Op: "open", Path: d.Path, Err: os.ErrNotExist}
		}
		return cached.text, nil
	}
	m.mu.Unlock()
	if !exist {
		return "", &os.PathError{Op: "open", Path: d.Path, Err: os.ErrNotExist}
	}
	// The reading happens outside the lock: two requests may open the same file
	// at once, and both write the same answer.
	m.mu.Lock()
	m.reads++
	m.mu.Unlock()
	b, err := os.ReadFile(d.Path)
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	m.files[d.Path] = memoFile{size: size, mod: mod, text: string(b), exist: true}
	m.mu.Unlock()
	return string(b), nil
}

// IsFile reports whether the document names a regular file, asking the disk
// rather than a held copy: a file made or taken away is there on this answer.
func (m *Memo) IsFile(d Doc) bool {
	info, err := os.Stat(d.Path)
	return err == nil && !info.IsDir()
}

// IsDir reports whether the document names a directory.
func (m *Memo) IsDir(d Doc) bool {
	info, err := os.Stat(d.Path)
	return err == nil && info.IsDir()
}

// YAMLIn lists the *.yaml files directly inside a directory, by name.
func (m *Memo) YAMLIn(d Doc) ([]Doc, error) {
	return m.filesIn(d, ".yaml")
}

// MarkdownIn lists the *.md files directly inside a directory, by name.
func (m *Memo) MarkdownIn(d Doc) ([]Doc, error) {
	return m.filesIn(d, ".md")
}

// filesIn lists one directory's files of an extension, under the directory's
// own version: a file added or taken away moves it, and a file whose text
// changed does not (its name did not move).
func (m *Memo) filesIn(d Doc, ext string) ([]Doc, error) {
	info, err := os.Stat(d.Path)
	if err != nil {
		return nil, err
	}
	mod := info.ModTime().UnixNano()
	key := d.Path + "|" + ext
	m.mu.Lock()
	cached, ok := m.dirs[key]
	m.mu.Unlock()
	if ok && cached.mod == mod {
		return docsOf(cached.names, d), nil
	}
	m.mu.Lock()
	m.lists++
	m.mu.Unlock()
	entries, err := os.ReadDir(d.Path)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ext) {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	m.mu.Lock()
	m.dirs[key] = memoDir{mod: mod, names: names}
	m.mu.Unlock()
	return docsOf(names, d), nil
}

// docsOf is one directory's names as documents.
func docsOf(names []string, d Doc) []Doc {
	out := make([]Doc, 0, len(names))
	for _, name := range names {
		out = append(out, Doc{Layer: d.Layer, Path: filepath.Join(d.Path, name)})
	}
	return out
}

// Stats is what a test counts: files opened, directories listed, and versions
// answered from the held copy.
func (m *Memo) Stats() (reads, lists, hits int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reads, m.lists, m.hits
}

// ResetStats forgets the counts, not the held versions.
func (m *Memo) ResetStats() {
	m.mu.Lock()
	m.reads, m.lists, m.hits = 0, 0, 0
	m.mu.Unlock()
}
