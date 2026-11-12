package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/aq2bq/hergy/internal/words"
)

// wordsGate uses the same pure renderer for comparisons and -write. Unmarked
// document text is input to projection, never overwritten by the generator.
func (r *runner) wordsGate() error {
	outputs, err := words.Generated()
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(outputs))
	for path := range outputs {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if err := r.wordsOutput(path, outputs[path]); err != nil {
			return err
		}
	}
	return nil
}

func (r *runner) wordsOutput(path string, made []byte) error {
	target := filepath.Join(r.root, path)
	if r.write {
		return os.WriteFile(target, made, 0644)
	}
	have, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	if !bytes.Equal(have, made) {
		return fmt.Errorf("%s differs from internal/words (go run ./tools/check words -write)", path)
	}
	return nil
}
