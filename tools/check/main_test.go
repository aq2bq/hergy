package main

import (
	"strings"
	"testing"
)

// The -h text is written from the same table the run uses, so it cannot
// fall behind: every gate, its rule and every pinned tool are in it.
func TestUsageIsWrittenFromTheTable(t *testing.T) {
	text := usage()
	for _, g := range gates {
		line := "  " + g.name + " "
		if !strings.Contains(text, line) {
			t.Errorf("usage lacks the gate %q", g.name)
		}
		if !strings.Contains(text, string(g.rule)) {
			t.Errorf("usage lacks the rule %q of %s", g.rule, g.name)
		}
	}
	for _, tool := range tools {
		if !strings.Contains(text, tool.pkg+"@"+tool.version) {
			t.Errorf("usage lacks the tool %s@%s", tool.pkg, tool.version)
		}
	}
}

func TestEveryGateHasARuleAndAnAbout(t *testing.T) {
	seen := map[string]bool{}
	for _, g := range gates {
		if g.rule != Hard && g.rule != Ratchet {
			t.Errorf("gate %s: rule %q is neither Hard nor Ratchet", g.name, g.rule)
		}
		if g.about == "" || g.run == nil {
			t.Errorf("gate %s: lacks an about or a run", g.name)
		}
		if seen[g.name] {
			t.Errorf("gate %s is listed twice", g.name)
		}
		seen[g.name] = true
	}
}

func TestSelectGatesRefusesAnUnknownName(t *testing.T) {
	if _, err := selectGates([]string{"lint", "nope"}); err == nil || !strings.Contains(err.Error(), `"nope"`) {
		t.Fatalf("selectGates(lint nope) = %v, want an error naming nope", err)
	}
	got, err := selectGates([]string{"vuln", "fmt"})
	if err != nil || len(got) != 2 || got[0].name != "vuln" || got[1].name != "fmt" {
		t.Fatalf("selectGates(vuln fmt) = %v, %v", got, err)
	}
}

func TestGatesForPicksWhatAChangeCanFail(t *testing.T) {
	names := func(paths ...string) string {
		var out []string
		for _, g := range gatesFor(paths) {
			out = append(out, g.name)
		}
		return strings.Join(out, " ")
	}
	cases := []struct {
		paths []string
		want  string
	}{
		{nil, ""},
		{[]string{"README.md"}, "test"},
		{[]string{"internal/app/serve.go"}, "fmt tidy fix lint arch test e2e coverage deadcode"},
		{[]string{"internal/app/serve_test.go"}, "fmt fix lint arch test coverage deadcode"},
		{[]string{"tools/check/diff.go"}, "fmt fix lint arch test coverage deadcode"},
		{[]string{"internal/web/src/roles/main.js"}, "webbuild names types unit test e2e"},
		{[]string{"internal/web/static/app.js"}, "webbuild test e2e"},
		{[]string{"internal/web/static/app.css"}, "test e2e"},
		{[]string{"internal/web/templates/roles_v2.html"}, "webbuild test e2e"},
		{[]string{"internal/web/tsconfig.json"}, "webbuild types unit test"},
		{[]string{"tools/webvendor/build-site.js"}, "webbuild test deadcode"},
		{[]string{"tools/webtools/eslint.config.js"}, "names types apitypes unit test"},
		{[]string{"internal/apicontract/openapi.yaml"}, "apitypes test e2e"},
		{[]string{"internal/web/jstest/resource.test.js"}, "names unit test"},
		{[]string{"e2e/tests/home.spec.ts"}, "test reading e2e"},
		{[]string{"go.sum"}, "tidy lint test e2e deadcode vuln"},
		{[]string{".testcoverage.yml", "skills/hergy/SKILL.md"}, "test coverage"},
	}
	for _, c := range cases {
		if got := names(c.paths...); got != c.want {
			t.Errorf("gatesFor(%v) = %q, want %q", c.paths, got, c.want)
		}
	}
}
