package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/source"
)

func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// teamText is a team.yaml body whose prompt is only the documents, so the
// tests read the concatenation without any other prompt part in the way.
func teamText(docs string) string {
	return "version: 1\nteam: t\nprompt: \"%{docs}\"\ndocs: " + docs + "\n" +
		"members:\n  - name: a\n    role: r\n    kind: claude\n"
}

func parseTeam(t *testing.T, dir, text string) (*Config, error) {
	t.Helper()
	return Parse(Input{
		YAML:         text,
		TeamName:     "t",
		Layer:        source.LayerLocal,
		TeamYAMLPath: filepath.Join(dir, "team.yaml"),
		TeamDir:      dir,
		Base:         dir,
		Reader:       source.Disk{},
		Executable:   "hergy",
	})
}

func TestParseConcatenatesDocsInOrder(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "z.md"), "Z doc\n")
	write(t, filepath.Join(dir, "a.md"), "A doc\n")
	write(t, filepath.Join(dir, "guide", "m.md"), "M doc\n")
	write(t, filepath.Join(dir, "guide", "n.md"), "N doc\n")
	write(t, filepath.Join(dir, "guide", "skip.txt"), "not markdown\n")
	write(t, filepath.Join(dir, "guide", "deeper", "x.md"), "not directly inside\n")

	cfg, err := parseTeam(t, dir, teamText(`["z.md", "a.md", "guide"]`))
	if err != nil {
		t.Fatal(err)
	}
	want := "\nZ doc\n\nA doc\n\nM doc\n\nN doc\n"
	if got := cfg.Members[0].Prompt; got != want {
		t.Errorf("prompt = %q, want %q", got, want)
	}

	var read []string
	for _, doc := range cfg.Docs {
		read = append(read, filepath.Base(doc.Path))
	}
	if wantDocs := []string{"z.md", "a.md", "m.md", "n.md"}; strings.Join(read, ",") != strings.Join(wantDocs, ",") {
		t.Errorf("Docs = %v, want %v", read, wantDocs)
	}
}

func TestParseStripsHTMLCommentsFromDocs(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "note.md"), "Keep this\n<!-- a note for humans -->\nAnd this\n")

	cfg, err := parseTeam(t, dir, teamText(`["note.md"]`))
	if err != nil {
		t.Fatal(err)
	}
	prompt := cfg.Members[0].Prompt
	for _, unwanted := range []string{"<!--", "a note for humans"} {
		if strings.Contains(prompt, unwanted) {
			t.Errorf("prompt %q still contains %q", prompt, unwanted)
		}
	}
	for _, wanted := range []string{"Keep this", "And this"} {
		if !strings.Contains(prompt, wanted) {
			t.Errorf("prompt %q is missing %q", prompt, wanted)
		}
	}
}

func TestParseErrorsOnMissingDocs(t *testing.T) {
	dir := t.TempDir()
	_, err := parseTeam(t, dir, teamText(`["nope.md"]`))
	if err == nil {
		t.Fatal("a docs entry that does not exist did not stop the parse")
	}
	if !strings.Contains(err.Error(), "nope.md") {
		t.Errorf("error %q does not name the missing entry", err)
	}
}

func TestParseStopsOnBrokenYAML(t *testing.T) {
	cases := []struct {
		name  string
		parse func() error
	}{
		{
			name: "team.yaml without a version",
			parse: func() error {
				_, err := parseTeam(t, t.TempDir(), "team: t\nmembers:\n  - name: a\n    role: r\n    kind: claude\n")
				return err
			},
		},
		{
			name: "team.yaml with an unknown key",
			parse: func() error {
				_, err := parseTeam(t, t.TempDir(), "version: 1\nteam: t\nunknown_key: 1\nmembers:\n  - name: a\n    role: r\n    kind: claude\n")
				return err
			},
		},
		{
			name: "config.yaml without a version",
			parse: func() error {
				_, err := ParseConfig("lang: ja\n", "config.yaml")
				return err
			},
		},
		{
			name: "config.yaml with an unknown key",
			parse: func() error {
				_, err := ParseConfig("version: 1\nlang: ja\nunknown_key: 1\n", "config.yaml")
				return err
			},
		},
		{
			name: "profiles.yaml without a version",
			parse: func() error {
				_, err := ParseProfiles("profiles:\n  p:\n    kind: claude\n", "profiles.yaml")
				return err
			},
		},
		{
			name: "profiles.yaml with an unknown key",
			parse: func() error {
				_, err := ParseProfiles("version: 1\nprofiles:\n  p:\n    kind: claude\n    unknown_key: 1\n", "profiles.yaml")
				return err
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.parse(); err == nil {
				t.Error("broken YAML did not stop the parse")
			}
		})
	}
}

func TestParseReadsProfilesIntoMembers(t *testing.T) {
	dir := t.TempDir()
	profiles := map[string]Profile{
		"lead": {Name: "lead", Kind: "claude", Args: []string{"--model", "opus"}, Path: "profiles.yaml"},
	}
	cfg, err := Parse(Input{
		YAML:         "version: 1\nteam: t\nprompt: \"%{name}\"\nmembers:\n  - name: a\n    role: r\n    profile: lead\n",
		TeamName:     "t",
		Layer:        source.LayerLocal,
		TeamYAMLPath: filepath.Join(dir, "team.yaml"),
		TeamDir:      dir,
		Base:         dir,
		Profiles:     profiles,
		ProfilesYAML: "profiles.yaml",
		Reader:       source.Disk{},
		Executable:   "hergy",
	})
	if err != nil {
		t.Fatal(err)
	}
	member := cfg.Members[0]
	if member.Kind != "claude" || member.Profile != "lead" {
		t.Errorf("member kind/profile = %q/%q, want claude/lead", member.Kind, member.Profile)
	}
	if strings.Join(member.Args, " ") != "--model opus" {
		t.Errorf("member args = %v", member.Args)
	}
	if !cfg.UsesProfiles() {
		t.Error("UsesProfiles() = false, want true")
	}
}

func TestParseUnknownProfileStops(t *testing.T) {
	_, err := Parse(Input{
		YAML:         "version: 1\nteam: t\nmembers:\n  - name: a\n    role: r\n    profile: absent\n",
		TeamName:     "t",
		Layer:        source.LayerLocal,
		TeamYAMLPath: "team.yaml",
		Reader:       source.Disk{},
	})
	if err == nil {
		t.Fatal("a profile that does not exist did not stop the parse")
	}
	if !strings.Contains(err.Error(), "absent") {
		t.Errorf("error %q does not name the missing profile", err)
	}
}

// TestConfigYAMLRoundTrips: the config.yaml hergy writes is one ParseConfig
// reads back to the same language.
func TestConfigYAMLRoundTrips(t *testing.T) {
	for _, lang := range []string{"en", "ja"} {
		text := ConfigYAML(lang)
		got, err := ParseConfig(text, "config.yaml")
		if err != nil {
			t.Fatalf("ParseConfig(%q) = %v", text, err)
		}
		if got != lang {
			t.Errorf("ParseConfig(%q) lang = %q, want %q", text, got, lang)
		}
	}
}
