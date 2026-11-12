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

func commandTable(lang string) string {
	var b strings.Builder
	if lang == "ja" {
		b.WriteString("| コマンド | 内容 |\n")
	} else {
		b.WriteString("| Command | What it does |\n")
	}
	b.WriteString("|---|---|\n")
	cell := func(text string) string {
		text = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "|", "&#124;").Replace(text)
		return strings.ReplaceAll(text, "\n", "<br>")
	}
	for _, kind := range options.Kinds {
		fmt.Fprintf(&b, "| `%s` | %s |\n", strings.ReplaceAll(options.Synopsis(kind, lang), "|", "\\|"), cell(options.Description(kind, lang)))
	}
	b.WriteString("\n")
	for _, flag := range options.FlagSummaries() {
		fmt.Fprintf(&b, "- `%s`: %s\n", flag.Label, options.FlagDescriptions(flag.Name, lang))
	}
	return b.String()
}

func projectedDocuments(root string) (map[string][]byte, error) {
	outputs := map[string][]byte{}
	for _, doc := range []struct {
		path, lang string
		regions    []string
	}{{"README.md", "en", []string{"new", "rule-documents", "identity", "resolution", "placeholders", "reference", "prompt-preview"}}, {"README.ja.md", "ja", []string{"new", "rule-documents", "identity", "resolution", "placeholders", "reference", "prompt-preview"}}, {"skills/hergy/SKILL.md", "en", []string{"reference"}}} {
		raw, err := os.ReadFile(filepath.Join(root, doc.path))
		if err != nil {
			return nil, err
		}
		content, err := words.Region(string(raw), "commands", commandTable(doc.lang))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", doc.path, err)
		}
		regions := map[string]string{
			"new":            words.Describe(words.ForCommand("new"), doc.lang),
			"reference":      "```text\n" + words.Reference(doc.lang) + "```",
			"rule-documents": words.Find("reference.rule-documents").Explain(doc.lang),
			"identity":       words.Find("team.identity").Explain(doc.lang),
			"resolution":     words.Find("reference.resolution").Explain(doc.lang),
			"prompt-preview": words.Find("web.prompt-preview").Explain(doc.lang),
			"placeholders":   words.Find("reference.bootstrap-prompt-placeholders").Explain(doc.lang),
		}
		for _, name := range doc.regions {
			body := regions[name]
			content, err = words.Region(content, name, body)
			if err != nil {
				return nil, err
			}
		}
		outputs[doc.path] = []byte(content)
	}
	for _, lang := range []string{"en", "ja"} {
		path := "internal/i18n/locales/" + lang + ".json"
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return nil, err
		}
		generated := map[string]string{"cheat.people": cheat.BuildPeople(lang), "usage": options.BuildUsage(lang)}
		for _, rule := range words.Rules() {
			if rule.ErrorKey != "" {
				generated[rule.ErrorKey] = rule.Pattern(lang)
			}
		}
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
