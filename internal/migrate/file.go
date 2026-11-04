package migrate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// File is a checked conversion of one YAML file. Prepare does not write.
// The caller prepares all files before applying any of them.
type File struct {
	Path        string
	Destination string
	Backup      string
	original    string
	markdown    string
}

// Prepare checks a source and its adjacent destination and backup. A matching
// backup permits resuming an interrupted conversion; an unrelated .md does not.
func Prepare(path string) (File, error) {
	f := File{Path: path, Destination: strings.TrimSuffix(path, ".yaml") + ".md", Backup: path + ".bak"}
	if filepath.Ext(path) != ".yaml" {
		return File{}, fmt.Errorf("%s: expected a .yaml definition", path)
	}
	original, err := readRegular(path)
	if errors.Is(err, fs.ErrNotExist) {
		original, err = readRegular(f.Backup)
	}
	if err != nil {
		return File{}, err
	}
	doc, err := Convert(original, path)
	if err != nil {
		return File{}, err
	}
	f.original, f.markdown = original, doc.Text
	if err := f.check(); err != nil {
		return File{}, err
	}
	return f, nil
}

func readRegular(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s: migration requires a regular file", path)
	}
	data, err := os.ReadFile(path)
	return string(data), err
}

func matches(path, text string) (bool, error) {
	got, err := readRegular(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if got != text {
		return false, fmt.Errorf("%s: content differs; left unchanged", path)
	}
	return true, nil
}

func (f File) check() error {
	backup, err := matches(f.Backup, f.original)
	if err != nil {
		return err
	}
	exists, err := matches(f.Destination, f.markdown)
	if err != nil {
		return err
	}
	if exists && !backup {
		return fmt.Errorf("%s: destination already exists without a matching backup", f.Destination)
	}
	return nil
}

// Apply keeps the original bytes in .yaml.bak and installs Markdown without
// replacing an existing file. It can be called again after partial completion.
// Symlinks are refused rather than moving a link or modifying its target.
func (f File) Apply() error { return f.apply(os.Link) }

func (f File) apply(link func(string, string) error) error {
	if f.Path == "" {
		return errors.New("migration was not prepared")
	}
	exists, err := matches(f.Path, f.original)
	if err != nil {
		return err
	}
	if err := f.check(); err != nil {
		return err
	}
	if !exists {
		backup, err := matches(f.Backup, f.original)
		if err != nil {
			return err
		}
		if !backup {
			return fmt.Errorf("%s: source and backup are missing", f.Path)
		}
	}
	if err := install(f.Backup, f.original, link); err != nil {
		return err
	}
	if err := install(f.Destination, f.markdown, link); err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if _, err := matches(f.Path, f.original); err != nil {
		return err
	}
	return os.Remove(f.Path)
}

// install publishes a complete temporary file with an exclusive hard link.
// Unlike rename, the link cannot overwrite a destination created concurrently.
func install(path, text string, link func(string, string) error) error {
	exists, err := matches(path, text)
	if err != nil || exists {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".migrate-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temp.Name()) }()
	if _, err := temp.WriteString(text); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := link(temp.Name(), path); err != nil {
		return fmt.Errorf("%s: cannot install migration file with an exclusive hard link: %w", path, err)
	}
	return nil
}
