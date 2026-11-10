package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// confirmedSendRules names functions, rather than whole files: turns.ask is a
// single-attempt question, not an exception for every sender in turns.go.
// The YAML records both ownership boundaries and the question exception. The
// gate checks references too, so assigning PromptWait to a variable cannot
// introduce a second retry loop outside ConfirmPrompt.
type confirmedSendRules struct {
	Owner         string   `yaml:"owner"`
	WaitBuilder   string   `yaml:"wait_builder"`
	SingleAttempt []string `yaml:"single_attempt"`
}

func (r *runner) confirmedSendFindings(conf *confirmedSendRules) ([]string, error) {
	if conf == nil {
		return nil, nil
	}
	for _, location := range append([]string{conf.Owner, conf.WaitBuilder}, conf.SingleAttempt...) {
		path, function, ok := strings.Cut(location, "#")
		if !ok || path == "" || function == "" {
			return nil, fmt.Errorf("%s: confirmed_send は ファイル#型.関数 で書いてください: %q", ownersPath, location)
		}
	}
	files, err := r.sourceFiles()
	if err != nil {
		return nil, err
	}
	var findings []string
	for _, path := range files {
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			continue
		}
		rel, err := filepath.Rel(r.root, path)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		lines := token.NewFileSet()
		file, err := parser.ParseFile(lines, path, raw, 0)
		if err != nil {
			return nil, err
		}
		for _, declaration := range file.Decls {
			findings = append(findings, confirmedSendDeclaration(conf, rel, lines, declaration)...)
		}
	}
	sort.Strings(findings)
	return findings, nil
}

func confirmedSendDeclaration(conf *confirmedSendRules, path string, lines *token.FileSet, declaration ast.Decl) []string {
	function := ""
	if fn, ok := declaration.(*ast.FuncDecl); ok {
		function = receiptFunction(fn)
	}
	location := path + "#" + function
	allowedWait := location == conf.Owner
	for _, exception := range conf.SingleAttempt {
		allowedWait = allowedWait || location == exception
	}
	var findings []string
	report := func(pos token.Pos, description string) {
		findings = append(findings, fmt.Sprintf("%s:%d: 確認つきの送信の持ち主: %s（%s）", path, lines.Position(pos).Line, description, ownersPath))
	}
	if fn, ok := declaration.(*ast.FuncDecl); ok {
		switch fn.Name.Name {
		case "ConfirmPrompt":
			if location != conf.Owner {
				report(fn.Pos(), "ConfirmPrompt は "+conf.Owner+" だけで定義してください")
			}
		case "PromptWait":
			if location != conf.WaitBuilder {
				report(fn.Pos(), "PromptWait は "+conf.WaitBuilder+" だけで定義してください")
			}
		}
	}
	// Exact string tokens catch both JSON("agent", "prompt", ..., "--wait")
	// and an argv slice assembled in several statements; prose is not a token.
	tokens := map[string]bool{}
	ast.Inspect(declaration, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.SelectorExpr:
			if n.Sel.Name == "PromptWait" && !allowedWait {
				report(n.Pos(), "PromptWait の参照は "+conf.Owner+" と single_attempt の問いかけだけに置いてください")
			}
		case *ast.BasicLit:
			if n.Kind == token.STRING {
				if value, err := strconv.Unquote(n.Value); err == nil {
					tokens[value] = true
				}
			}
		}
		return true
	})
	if tokens["agent"] && tokens["prompt"] && tokens["--wait"] && location != conf.WaitBuilder {
		report(declaration.Pos(), "agent prompt --wait の組み立ては "+conf.WaitBuilder+" だけに置いてください")
	}
	return findings
}

func receiptFunction(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	receiver := fn.Recv.List[0].Type
	if pointer, ok := receiver.(*ast.StarExpr); ok {
		receiver = pointer.X
	}
	if name, ok := receiver.(*ast.Ident); ok {
		return name.Name + "." + fn.Name.Name
	}
	return fn.Name.Name
}
