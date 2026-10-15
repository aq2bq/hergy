package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/web"
)

// These are the write promises of spec/10 の 4 節 as the app keeps them, over a
// working directory with both layers (K2). The screen's own tests use a fake
// Port; these run the real one, so a promise cannot rot without a red test.

// TestWriteRuleKeepsThePromises: only a rule file a member of the layer reads is
// written, the file has to be the one that was read, and the write leaves no
// temp file beside it.
func TestWriteRuleKeepsThePromises(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeDemoTeam(t, f, "demo")
	p := servePort{env: serveEnv(t)}
	dir := filepath.Join(f.proj, ".hergy", "teams", "demo")
	doc := filepath.Join(dir, "protocol.yaml")
	before := readRule(t, doc)
	hash := hashText(before)

	t.Run("a file no member reads is refused", func(t *testing.T) {
		if err := p.WriteRule(filepath.Join(dir, "team.yaml"), "x", hash); err == nil {
			t.Error("a team.yaml was written through the rules screen")
		}
	})
	t.Run("a path outside the layers is refused", func(t *testing.T) {
		if err := p.WriteRule(filepath.Join(f.proj, "notes.txt"), "x", hash); err == nil {
			t.Error("a file outside the layers was written")
		}
	})
	t.Run("a hash that is not the file's is refused", func(t *testing.T) {
		if err := p.WriteRule(doc, before+"\n", "0000000000000000"); err == nil {
			t.Error("a write with another hash went through")
		}
		if got := readRule(t, doc); got != before {
			t.Errorf("a refused write changed the file:\n%s", got)
		}
	})
	t.Run("the write goes through and leaves no temp file", func(t *testing.T) {
		next := before + "\n# written by the rules screen\n"
		if err := p.WriteRule(doc, next, hash); err != nil {
			t.Fatalf("WriteRule: %v", err)
		}
		if got := readRule(t, doc); got != next {
			t.Errorf("the file is\n%s\nwant\n%s", got, next)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".") {
				t.Errorf("a temp file is left behind: %s", entry.Name())
			}
		}
	})
}

// TestWriteRuleRefusesAGlobalFileOnlyALocalTeamReads: the layer is read from
// where the file stands, so a file of the global layer belongs to the global
// layer even when a local team definition borrowed it by an absolute path. The
// person showing the local layer cannot write it.
func TestWriteRuleRefusesAGlobalFileOnlyALocalTeamReads(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	borrowed := filepath.Join(f.cfg, "hergy", "borrowed", "shared.yaml")
	writeFile(t, borrowed, "version: 1\nsections:\n  - rules:\n      - id: S1\n        text: one\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "borrow", "team.yaml"), `version: 1
docs:
  - `+borrowed+`
roles: [roles]
members:
  - name: lead
    role: lead
    kind: claude
`)
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "borrow", "roles", "lead.yaml"), "version: 1\nsections:\n  - rules:\n      - id: L1\n        text: two\n")
	p := servePort{env: serveEnv(t)}

	text := readRule(t, borrowed)
	if !p.reads(web.LayerLocal, borrowed) {
		t.Fatal("the test team does not read the borrowed file")
	}
	err := p.WriteRule(borrowed, text+"\n", hashText(text))
	if err == nil {
		t.Error("a global file that only a local team definition reads was written")
	}
	if got := readRule(t, borrowed); got != text {
		t.Error("the refused write changed the borrowed file")
	}
}

// TestWriteRuleKeepsALinkALink: a settings file kept as a link (a dotfiles
// setup) is written where it points, and stays a link (K3).
func TestWriteRuleKeepsALinkALink(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeDemoTeam(t, f, "demo")
	target := filepath.Join(f.proj, "src", "protocol.yaml")
	writeFile(t, target, readRule(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "protocol.yaml")))
	link := filepath.Join(f.proj, ".hergy", "teams", "demo", "protocol.yaml")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0o600); err != nil {
		t.Fatal(err)
	}
	p := servePort{env: serveEnv(t)}
	before := readRule(t, link)
	next := before + "\n# through the link\n"
	if err := p.WriteRule(link, next, hashText(before)); err != nil {
		t.Fatalf("WriteRule: %v", err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("the link was replaced by a plain file")
	}
	if got := readRule(t, target); got != next {
		t.Errorf("the file the link points at is\n%s\nwant\n%s", got, next)
	}
	if got, want := fileMode(t, target), os.FileMode(0o600); got != want {
		t.Errorf("the mode is %v, want the file's own %v", got, want)
	}
}

// TestPreviewNamesABreakOfThisEditAndNoOther: the preview runs hergy up's own
// reading over the edited file, and speaks only about that edit (K2).
func TestPreviewSpeaksAboutThisEditOnly(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeDemoTeam(t, f, "demo")
	doc := filepath.Join(f.proj, ".hergy", "teams", "demo", "protocol.yaml")
	p := servePort{env: serveEnv(t)}

	t.Run("an id that stands twice across the member's files is a break", func(t *testing.T) {
		text := strings.Replace(readRule(t, doc), "id: C2", "id: L1", 1)
		got, err := p.Preview(web.LayerLocal, doc, 0, text)
		if err != nil {
			t.Fatalf("Preview: %v", err)
		}
		if got.Break == "" {
			t.Fatal("a duplicate id is not a break")
		}
		if !strings.Contains(got.Break, "L1") {
			t.Errorf("the break does not name the id: %s", got.Break)
		}
	})

	t.Run("an edit YAML cannot read is a break", func(t *testing.T) {
		got, err := p.Preview(web.LayerLocal, doc, 0, "version: 1\nsections: [\n")
		if err != nil {
			t.Fatalf("Preview: %v", err)
		}
		if got.Break == "" {
			t.Error("an edit that does not read is not a break")
		}
	})

	t.Run("a sound edit is no break", func(t *testing.T) {
		text := strings.Replace(readRule(t, doc), "text: two", "text: two, edited", 1)
		got, err := p.Preview(web.LayerLocal, doc, 0, text)
		if err != nil {
			t.Fatalf("Preview: %v", err)
		}
		if got.Break != "" {
			t.Errorf("a sound edit is a break: %s", got.Break)
		}
		if !strings.Contains(got.Section, "two, edited") {
			t.Errorf("the section does not show the edit:\n%s", got.Section)
		}
	})

	t.Run("a broken definition that does not read the file is not a break", func(t *testing.T) {
		writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "torn", "team.yaml"), `version: 1
roles: [roles]
members:
  - name: lead
    role: lead
    kind: claude
`)
		writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "torn", "roles", "lead.yaml"), "version: 1\nsections:\n  - rules:\n      - text: no id here\n")
		got, err := p.Preview(web.LayerLocal, doc, 0, readRule(t, doc))
		if err != nil {
			t.Fatalf("Preview: %v", err)
		}
		if got.Break != "" {
			t.Errorf("an unrelated broken definition is a break: %s", got.Break)
		}
	})
}

// TestRuleTeamListsWhatEachMemberReads: the screen's file list and its checks
// come from here, so the role file and the documents are named, the role file
// first, the way a prompt is built.
func TestRuleTeamListsWhatEachMemberReads(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeDemoTeam(t, f, "demo")
	writeDemoTeam(t, f, "other")
	p := servePort{env: serveEnv(t)}
	teams := p.RuleTeams(web.LayerLocal)
	if len(teams) != 2 || teams[0].Name != "demo" || teams[1].Name != "other" {
		t.Fatalf("RuleTeams = %+v, want the two definitions by name", teams)
	}
	if len(teams[0].Members) != 2 {
		t.Fatalf("demo has %d members, want 2", len(teams[0].Members))
	}
	lead := teams[0].Members[0]
	if !strings.HasSuffix(lead.Name, "lead") || len(lead.Files) != 2 {
		t.Fatalf("the first member = %+v", lead)
	}
	if !lead.Files[0].Role || !strings.HasSuffix(lead.Files[0].Path, filepath.Join("roles", "lead.yaml")) {
		t.Errorf("the role file does not come first: %+v", lead.Files)
	}
	if lead.Files[1].Role || !strings.HasSuffix(lead.Files[1].Path, "protocol.yaml") {
		t.Errorf("the document does not come second: %+v", lead.Files)
	}
}

func readRule(t *testing.T, path string) string {
	t.Helper()
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(text)
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}
