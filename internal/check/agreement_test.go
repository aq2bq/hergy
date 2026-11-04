package check

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/source"
)

func (f *fixture) agreementPath() string {
	return filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.md")
}

func TestProseArrowsDoNotChangeTopologyChecks(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	f.roles()
	text := "# 合意\r\n不明な人 → 別の人。lead ⇄ unknown。\n"
	f.write(f.agreementPath(), text)
	report := Agreement(AgreementInput{Catalog: f.cat, Name: "demo", Place: source.LayerLocal, Text: text})
	if len(report.Problems) != 0 {
		t.Fatalf("prose warnings: %+v", report.Problems)
	}
	for _, finding := range f.findings() {
		if finding.Stops {
			t.Fatal(finding)
		}
	}
	f.team("  - human -> missing\n")
	found := false
	for _, finding := range f.findings() {
		found = found || finding.Stops
	}
	if !found {
		t.Fatal("invalid topology was accepted")
	}
}

func TestRoleProseBudgetAndExactPreview(t *testing.T) {
	f := newFixture(t)
	for _, n := range []int{0, 1200, 1201} {
		text := strings.Repeat("あ", n)
		got := Role(RoleInput{Catalog: f.cat, Place: source.LayerLocal, Name: "lead", Text: text})
		if got.Markdown.Chars != n || got.Limit != 1200 {
			t.Fatalf("size: %+v", got)
		}
		want := 0
		if n > 1200 {
			want = 1
		}
		if len(got.Problems) != want {
			t.Fatalf("%d: %+v", n, got.Problems)
		}
	}
	text := "# 題\r\n\t本文 %{role_doc} → someone  \n"
	got := Role(RoleInput{Catalog: f.cat, Place: source.LayerLocal, Name: "lead", Text: text})
	var lines []string
	for _, line := range got.Markdown.Lines {
		lines = append(lines, line.Text)
	}
	if strings.Join(lines, "\n") != text {
		t.Fatal("preview changed text")
	}
}
