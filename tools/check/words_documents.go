package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aq2bq/hergy/internal/cheat"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/words"
)

func projectedDocuments(root string) (map[string][]byte, error) {
	outputs := map[string][]byte{}
	for _, lang := range []string{"en", "ja"} {
		path := "internal/i18n/locales/" + lang + ".json"
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return nil, err
		}
		generated := words.LocaleMessages(lang)
		generated["cheat.people"] = cheat.BuildPeople(lang)
		generated["usage"] = options.BuildUsage(lang)
		made, err := projectLocale(raw, generated)
		if err != nil {
			return nil, err
		}
		outputs[path] = made
	}
	return outputs, nil
}

// Locale keys stay in their original order. Only owned values are projected;
// unrelated messages are preserved byte for byte.
func projectLocale(raw []byte, generated map[string]string) ([]byte, error) {
	var messages map[string]string
	if err := json.Unmarshal(raw, &messages); err != nil {
		return nil, err
	}
	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		colon := strings.Index(line, "\": ")
		if colon < 0 {
			continue
		}
		var key string
		if err := json.Unmarshal([]byte(strings.TrimSpace(line[:colon+1])), &key); err != nil {
			return nil, err
		}
		value, ok := generated[key]
		if !ok {
			continue
		}
		var b strings.Builder
		encoder := json.NewEncoder(&b)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(value); err != nil {
			return nil, err
		}
		suffix := ""
		if strings.HasSuffix(line, ",") {
			suffix = ","
		}
		lines[i] = line[:colon+3] + strings.TrimSuffix(b.String(), "\n") + suffix
		delete(generated, key)
	}
	if len(generated) > 0 {
		return nil, fmt.Errorf("generated locale keys have no destination: %v", generated)
	}
	return []byte(strings.Join(lines, "\n")), nil
}
