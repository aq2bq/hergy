package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const legacy = `version: 1
title: Sample
sections:
  - heading: First
    intro: Introduction
    rules:
      - id: ABCD
        text: |
          First paragraph.
          Second line.
        note: private memo
      - id: EFGH
        text: Excluded text
        in_prompt: false
        off_since: 2026-10-01
  - heading: Nested
    level: 3
    rules:
      - text: Last paragraph.
`

func TestConvert(t *testing.T) {
	doc, err := Convert(legacy, "roles/sample.yaml")
	if err != nil {
		t.Fatal(err)
	}
	want := "# Sample\n\n## First\n\nIntroduction\n\n- First paragraph.\n   Second line.\n\n### Nested\n\n- Last paragraph.\n"
	if doc.Text != want {
		t.Fatalf("got %q, want %q", doc.Text, want)
	}
	for _, text := range []string{"", "version: [", "version: 1\nsections: []\nunknown: true"} {
		if _, err := Convert(text, "bad.yaml"); err == nil {
			t.Errorf("accepted %q", text)
		}
	}
}

func put(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
func contents(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_STATE_HOME", dir)
	path := filepath.Join(dir, "sample.yaml")
	put(t, path, legacy)
	return path
}

func TestPrepareAndApplyPreserveBackup(t *testing.T) {
	path := fixture(t)
	f, err := Prepare(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(f.Backup); !os.IsNotExist(err) {
		t.Fatal("prepare wrote backup")
	}
	if err := Apply(f); err != nil {
		t.Fatal(err)
	}
	if contents(t, f.Backup) != legacy {
		t.Fatal("backup changed")
	}
	if strings.Contains(contents(t, f.Destination), "ABCD") {
		t.Fatal("ID retained")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("old YAML remains")
	}
	again, err := Prepare(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(again); err != nil {
		t.Fatal(err)
	}
}

func TestResumeAndRefuseConflicts(t *testing.T) {
	for _, phase := range []string{"backup", "markdown", "source-changed", "backup-changed", "destination-changed", "unowned-destination"} {
		t.Run(phase, func(t *testing.T) {
			path := fixture(t)
			f, err := Prepare(path)
			if err != nil {
				t.Fatal(err)
			}
			switch phase {
			case "backup":
				put(t, f.Backup, legacy)
			case "markdown":
				put(t, f.Backup, legacy)
				put(t, f.Destination, f.markdown)
			case "source-changed":
				put(t, path, legacy+"# concurrent edit\n")
			case "backup-changed":
				put(t, f.Backup, "keep this backup")
			case "destination-changed":
				put(t, f.Destination, "# Edited prose\n")
			case "unowned-destination":
				put(t, f.Destination, f.markdown)
			}
			err = Apply(f)
			if phase == "backup" || phase == "markdown" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil {
					t.Fatal("conflict accepted")
				}
				if _, err := os.Stat(path); err != nil {
					t.Fatal("source removed")
				}
			}
		})
	}
}

func TestRefuseInvalidPathsAndMissingSource(t *testing.T) {
	path := fixture(t)
	for _, p := range []string{path + ".txt", path + "missing.yaml", filepath.Dir(path) + ".yaml"} {
		if _, err := Prepare(p); err == nil {
			t.Fatalf("accepted %s", p)
		}
	}
	link := filepath.Join(filepath.Dir(path), "link.yaml")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(link); err == nil {
		t.Fatal("accepted symlink")
	}
	f, err := Prepare(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := Apply(f); err == nil {
		t.Fatal("missing source accepted")
	}
	if err := Apply(File{}); err == nil {
		t.Fatal("unprepared accepted")
	}
}

func FuzzConvert(f *testing.F) {
	f.Add("version: 1\nsections: []\n")
	f.Add("version: 1\nsections:\n  - rules:\n      - text: hello\n")
	f.Fuzz(func(t *testing.T, text string) {
		first, err := Convert(text, "legacy.yaml")
		next, again := Convert(text, "legacy.yaml")
		if (err == nil) != (again == nil) || first != next {
			t.Fatal("conversion is not deterministic")
		}
	})
}
