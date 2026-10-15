package config

import (
	"path/filepath"
	"testing"

	"github.com/aq2bq/hergy/internal/source"
)

// The YAML readers must never panic, whatever the file holds. The seeds run
// with go test; the nightly workflow (go run ./tools/check nightly) runs them
// with -fuzz.

func FuzzParseTeam(f *testing.F) {
	f.Add("version: 1\nteam: t\nprompt: \"%{name}\"\nmembers:\n  - name: a\n    role: r\n    kind: claude\n")
	f.Add("version: 1\nteam: t\nmembers:\n  - name: lead\n    role: r\n    kind: claude\n    may_spawn:\n      - purpose: 調査\n        profile: seeker\n")
	f.Add("version: 2\n")
	f.Add("members: [")
	f.Add("")
	f.Fuzz(func(t *testing.T, text string) {
		dir := t.TempDir()
		cfg, err := Parse(Input{
			YAML:         text,
			TeamName:     "t",
			Layer:        source.LayerLocal,
			TeamYAMLPath: filepath.Join(dir, "team.yaml"),
			TeamDir:      dir,
			Base:         dir,
			Reader:       source.Disk{},
			Executable:   "hergy",
		})
		if (cfg == nil) == (err == nil) {
			t.Fatalf("Parse = %v, %v: exactly one of the two must be set", cfg, err)
		}
	})
}

func FuzzParseProfiles(f *testing.F) {
	f.Add("version: 1\nprofiles:\n  seeker:\n    kind: claude\n")
	f.Add("profiles: {a: {kind: x}}")
	f.Add("- 1")
	f.Add("")
	f.Fuzz(func(_ *testing.T, text string) {
		_, _ = ParseProfiles(text, "profiles.yaml")
	})
}

func FuzzParseConfig(f *testing.F) {
	f.Add("version: 1\nlang: ja\n")
	f.Add("lang: [1]")
	f.Add("")
	f.Fuzz(func(_ *testing.T, text string) {
		_, _ = ParseConfig(text, "config.yaml")
	})
}
