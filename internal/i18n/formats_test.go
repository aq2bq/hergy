package i18n

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// This file checks the other half of a message: how many values a call passes to
// it. go vet looks at fmt's own calls, not at i18n.Tf, so a message whose values
// do not add up used to reach a person as "%!s(MISSING)" — hergy new --local said
// "Take --%!s(MISSING) out" (n-60fd) and refresh said
// "Run `hergy refresh %!s(MISSING)`" (r-bac8). The call and the text are read here
// once, so the next one is caught before a person reads it.

// formatVerb is what a message's percent sign stands for: a value, or a percent
// sign of its own ("%%", which takes none). index is the value it names when the
// message says which one ("%[2]s"), and 0 when it takes the next one.
type formatVerb struct {
	kind  formatVerbKind
	index int
}

// formatVerbKind is whether a percent sign takes a value.
type formatVerbKind int

const (
	verbValue formatVerbKind = iota
	verbPercent
)

// verbs is the percent standing for a value in a message, in the order it is
// written.
func verbs(text string) []formatVerb {
	var out []formatVerb
	for i := 0; i < len(text); i++ {
		if text[i] != '%' {
			continue
		}
		j := i + 1
		index := 0
		// An explicit index ("%[1]s") names which value it takes; the index is
		// read so the count knows the values the message names.
		if j < len(text) && text[j] == '[' {
			start := j + 1
			for j < len(text) && text[j] != ']' {
				j++
			}
			if j < len(text) && j > start {
				if n, err := strconv.Atoi(text[start:j]); err == nil {
					index = n
				}
			}
			j++
		}
		// The flags, the width and the precision come before the verb.
		for j < len(text) && strings.ContainsRune("+-# 0123456789.*", rune(text[j])) {
			j++
		}
		if j >= len(text) {
			break
		}
		if text[j] == '%' {
			out = append(out, formatVerb{kind: verbPercent})
			i = j
			continue
		}
		out = append(out, formatVerb{kind: verbValue, index: index})
		i = j
	}
	return out
}

// valueVerbs is how many values a message takes: the highest index it names
// ("%[2]s" names the second value, and a message may name one twice), or the
// number of percent signs that take a value in order.
func valueVerbs(text string) int {
	count, highest := 0, 0
	for _, one := range verbs(text) {
		if one.kind == verbPercent {
			continue
		}
		count++
		if one.index > highest {
			highest = one.index
		}
	}
	if highest > 0 {
		return highest
	}
	return count
}

// locales reads both languages' messages.
func locales(t *testing.T) map[string]map[string]string {
	t.Helper()
	out := map[string]map[string]string{}
	for _, lang := range Available {
		raw, err := os.ReadFile(filepath.Join("locales", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		messages := map[string]string{}
		if err := json.Unmarshal(raw, &messages); err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		out[lang] = messages
	}
	return out
}

// localeCall is one call that fills a message: the key, and how many values it
// passes.
type localeCall struct {
	file  string
	line  int
	key   string
	value int
}

// localeCalls is every call in the sources that formats a message by its key.
// A call whose key is not written out at the call site, or that passes a slice
// through, is left alone: what it passes is not read here.
func localeCalls(t *testing.T, root string) []localeCall {
	t.Helper()
	fset := token.NewFileSet()
	var out []localeCall
	skip := map[string]bool{"testdata": true, "node_modules": true, "vendor": true, ".git": true}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skip[d.Name()] || (d.Name() == "static" && strings.HasSuffix(path, filepath.Join("web", "static"))) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		pkg := file.Name.Name
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			key, ok := keyOf(call, pkg)
			if !ok || call.Ellipsis.IsValid() {
				return true
			}
			out = append(out, localeCall{
				file: path, line: fset.Position(call.Pos()).Line,
				key: key, value: len(call.Args) - 1,
			})
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// keyOf is the message one call fills: i18n.Tf, i18n.Textf and i18n.Errorf by
// their selector, and the WebUI's own wrapper (web's tf) by its name. The key has
// to be written out at the call: a key read from a variable is not checked.
func keyOf(call *ast.CallExpr, pkg string) (string, bool) {
	var name string
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		ident, ok := fun.X.(*ast.Ident)
		if !ok || ident.Name != "i18n" {
			return "", false
		}
		name = fun.Sel.Name
	case *ast.Ident:
		name = fun.Name
		if pkg != "web" || name != "tf" {
			return "", false
		}
	default:
		return "", false
	}
	switch name {
	case "Tf", "Textf", "Errorf", "tf":
	default:
		return "", false
	}
	if len(call.Args) == 0 {
		return "", false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	key, err := strconv.Unquote(lit.Value)
	if err != nil || !strings.Contains(key, ".") {
		return "", false
	}
	return key, true
}

// TestEveryCallFillsTheMessagesValues: every call that fills a message passes as
// many values as the message has room for, in both languages (n-60fd, r-bac8).
func TestEveryCallFillsTheMessagesValues(t *testing.T) {
	texts := locales(t)
	calls := localeCalls(t, filepath.Join("..", ".."))
	seen := map[string]bool{}
	for _, call := range calls {
		seen[call.key] = true
		if _, ok := texts[Fallback][call.key]; !ok {
			t.Errorf("%s:%d fills %q, which the %s locale does not hold", call.file, call.line, call.key, Fallback)
		}
	}
	// The scan reads the calls it is there for: a message this repository formats
	// in more than one place, and one whose values were short (n-60fd, r-bac8).
	for _, key := range []string{"error.layer_flag_gone", "error.agent_not_gone"} {
		if !seen[key] {
			t.Fatalf("the scan read no call that fills %s; it is not reading the sources", key)
		}
	}
	for _, call := range calls {
		for lang, messages := range texts {
			text, ok := messages[call.key]
			if !ok {
				continue
			}
			if want := valueVerbs(text); want != call.value {
				t.Errorf("%s:%d: %s passes %d values, the %s message has room for %d: %q",
					call.file, call.line, call.key, call.value, lang, want, text)
			}
		}
	}
}
