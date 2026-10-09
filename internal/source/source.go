// Package source names the documents a prompt is built from and reads them.
//
// A Doc is a file or directory on disk. The layer records which layer supplied
// it (global or local), so the CLI can show where each value and document came
// from.
package source

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Layer names. Order is global -> local; local wins.
const (
	LayerGlobal = "global"
	LayerLocal  = "local"
)

// Doc is one document, or a directory of documents, a prompt may be built from.
type Doc struct {
	Layer string
	Path  string // an absolute disk path
}

// EntryDoc turns one docs/roles entry into a Doc. A relative entry is resolved
// against base.
func EntryDoc(entry, base, layer string) Doc {
	if !filepath.IsAbs(entry) {
		entry = filepath.Join(base, entry)
	}
	return Doc{Layer: layer, Path: entry}
}

// Display is how a Doc is shown to a person.
func (d Doc) Display() string { return d.Path }

// Reader reads documents named by Doc.
type Reader interface {
	Read(d Doc) (string, error)
	IsFile(d Doc) bool
	IsDir(d Doc) bool
	MarkdownIn(d Doc) ([]Doc, error)
}

// Disk reads documents from disk.
type Disk struct{}

// Read returns the whole document as UTF-8 text.
func (Disk) Read(d Doc) (string, error) {
	b, err := os.ReadFile(d.Path)
	return string(b), err
}

// IsFile reports whether the Doc names an existing regular file.
func (Disk) IsFile(d Doc) bool {
	info, err := os.Stat(d.Path)
	return err == nil && !info.IsDir()
}

// IsDir reports whether the Doc names an existing directory.
func (Disk) IsDir(d Doc) bool {
	info, err := os.Stat(d.Path)
	return err == nil && info.IsDir()
}

// MarkdownIn lists the *.md files directly inside a directory, by name.
func (Disk) MarkdownIn(d Doc) ([]Doc, error) {
	entries, err := os.ReadDir(d.Path)
	if err != nil {
		return nil, err
	}
	var out []Doc
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		out = append(out, Doc{Layer: d.Layer, Path: filepath.Join(d.Path, entry.Name())})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
