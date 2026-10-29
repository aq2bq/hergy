package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/options"
)

// TestCommandHelpNeedsNoHerdrAndPrintsThatCommandsFlags: hergy <command> --help
// prints that command's own page, in the language asked for, before anything
// touches herdr, and exits 0.
func TestCommandHelpNeedsNoHerdrAndPrintsThatCommandsFlags(t *testing.T) {
	newFixture(t)
	tests := []struct {
		argv []string
		has  []string
		not  []string
	}{
		{[]string{"new", "--help", "--lang", "ja"},
			[]string{"hergy new - チーム定義のひな形を作る", ".hergy/", "グローバル", "config.yaml"},
			[]string{"--timeout", "--as", "--purpose", "--local", "--global"}},
		{[]string{"up", "--help"},
			[]string{"hergy up - Start a team", "--as <team>", "--timeout <ms>"},
			[]string{"--purpose", "--profile"}},
		{[]string{"hire", "-h", "--lang", "ja"},
			[]string{"hergy hire - ワーカーを追加する", "--purpose <目的>", "--task <依頼>", "--from <メンバー>"},
			[]string{"--as", "--profile"}},
		{[]string{"cheat", "--help"},
			[]string{"hergy cheat - Show the cheatsheet", "--ai"},
			[]string{"--dry"}},
	}
	for _, tt := range tests {
		stdout, stderr, code := run(tt.argv...)
		if code != 0 || stderr != "" {
			t.Errorf("%v: exit %d, stderr %q", tt.argv, code, stderr)
			continue
		}
		for _, want := range tt.has {
			if !strings.Contains(stdout, want) {
				t.Errorf("%v does not print %q:\n%s", tt.argv, want, stdout)
			}
		}
		for _, no := range tt.not {
			if strings.Contains(stdout, no) {
				t.Errorf("%v prints %q, a flag the command does not accept:\n%s", tt.argv, no, stdout)
			}
		}
	}
}

// TestHelpOfNoCommandIsTheFullListAndPointsAtCommandHelp: hergy and hergy --help
// stay the list of every command, and its last line points at hergy <command>
// --help, which is how a person who only knows hergy gets to a command's flags.
func TestHelpOfNoCommandIsTheFullListAndPointsAtCommandHelp(t *testing.T) {
	newFixture(t)
	for _, lang := range []string{"en", "ja"} {
		flagged := mustRun(t, "--help", "--lang", lang)
		if word := mustRun(t, "help", "--lang", lang); word != flagged {
			t.Errorf("[%s] hergy help and hergy --help differ", lang)
		}
		if lang == "en" {
			if bare := mustRun(t); bare != flagged {
				t.Errorf("hergy and hergy --help differ")
			}
		}
		bare := flagged
		for _, command := range options.Commands() {
			if !strings.Contains(bare, "  "+command) {
				t.Errorf("[%s] the full list does not name %q", lang, command)
			}
		}
		last := strings.TrimRight(bare, "\n")
		last = last[strings.LastIndex(last, "\n")+1:]
		want := map[string]string{
			"en": "Details of one command: hergy <command> --help",
			"ja": "コマンドごとの説明は hergy <コマンド> --help",
		}[lang]
		if last != want {
			t.Errorf("[%s] the last line of the full list is %q, want %q", lang, last, want)
		}
	}
}

// TestOptionErrorsPointAtTheCommandsHelpOnStderr: the errors that name a flag
// the command does not have are still errors: exit 1, nothing on stdout, and one
// line on stderr that points at the page of the command typed.
func TestOptionErrorsPointAtTheCommandsHelpOnStderr(t *testing.T) {
	newFixture(t)
	tests := []struct {
		argv []string
		want string
	}{
		{[]string{"new", "--foo", "--lang", "en"}, "unknown option \"--foo\" (see `hergy new --help`)"},
		{[]string{"new", "--foo", "--lang", "ja"}, "不明なオプションです: \"--foo\"（hergy new --help を見てください）"},
		{[]string{"ls", "--as", "x", "--lang", "en"}, "--as cannot be used with hergy ls (see `hergy ls --help`)"},
		{[]string{"ls", "--as", "x", "--lang", "ja"}, "--as は hergy ls では使えません（hergy ls --help を見てください）"},
		{[]string{"frobnicate", "--lang", "en"}, "unknown command \"frobnicate\" (see `hergy help`)"},
	}
	for _, tt := range tests {
		stdout, stderr, code := run(tt.argv...)
		if code != 1 || stdout != "" || strings.Count(stderr, "\n") != 1 || !strings.Contains(stderr, tt.want) {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want exit 1, no stdout, one line with %q", tt.argv, code, stdout, stderr, tt.want)
		}
	}
}

// TestUsagePrintsWithABrokenConfig: the global config.yaml cannot be read, and
// the usage — hergy, hergy --help, hergy <command> --help, cheat and version —
// still prints and exits 0, with one line at the end of a help that says the
// file cannot be read (r-f60a, d-dfdd). The language is --lang, else English.
func TestUsagePrintsWithABrokenConfig(t *testing.T) {
	f := newFixture(t)
	configPath := filepath.Join(f.cfg, "hergy", "config.yaml")
	writeFile(t, configPath, "version: 1\n")
	tests := []struct {
		argv []string
		note bool
	}{
		{[]string{}, true},
		{[]string{"--help"}, true},
		{[]string{"help"}, true},
		{[]string{"new", "--help"}, true},
		{[]string{"up", "-h"}, true},
		{[]string{"version"}, false},
		{[]string{"cheat"}, false},
		{[]string{"cheat", "--ai"}, false},
	}
	sound := make([]string, len(tests))
	for i, tt := range tests {
		sound[i] = mustRun(t, tt.argv...)
	}

	// The file names a version hergy does not read: the usage prints what it
	// printed before, plus one line that says why the file is broken.
	writeFile(t, configPath, "version: 9\n")
	note := "the settings cannot be read: config.yaml: cannot read version 9 (version: 1 only): " + configPath + "; rewrite it as version: 1\n"
	for i, tt := range tests {
		stdout, stderr, code := run(tt.argv...)
		if code != 0 {
			t.Errorf("hergy %v: exit %d\nstderr:\n%s", tt.argv, code, stderr)
			continue
		}
		if stderr != "" {
			t.Errorf("hergy %v prints on stderr:\n%s", tt.argv, stderr)
		}
		want := sound[i]
		if tt.note {
			if !strings.HasSuffix(stdout, "\n"+note) {
				t.Errorf("hergy %v does not end with the broken-file line %q:\n%s", tt.argv, note, stdout)
			}
			want += "\n" + note
		}
		if stdout != want {
			t.Errorf("hergy %v with a broken config.yaml:\n got:\n%s\nwant:\n%s", tt.argv, stdout, want)
		}
	}

	// --lang chooses the language of that line and of the usage: the whole line
	// speaks the language of the run.
	stdout, _, code := run("--help", "--lang", "ja")
	if code != 0 || !strings.Contains(stdout, "設定が読めない: config.yaml: version 9 は読めません") {
		t.Errorf("hergy --help --lang ja does not say the file is broken in Japanese (exit %d):\n%s", code, stdout)
	}
}

// TestABrokenConfigStopsTheCommandsThatUseIt: the usage prints without the
// settings, but a command that stands on them still stops on the file, as it
// always has (d-dfdd).
func TestABrokenConfigStopsTheCommandsThatUseIt(t *testing.T) {
	f := newFixture(t)
	configPath := filepath.Join(f.cfg, "hergy", "config.yaml")
	writeFile(t, configPath, "version: 9\n")
	for _, argv := range [][]string{{"up"}, {"status"}, {"ls"}, {"serve", "--no-open"}} {
		stdout, stderr, code := run(argv...)
		if code != 1 {
			t.Errorf("hergy %v with a broken config.yaml: exit %d, want 1\nstdout:\n%s", argv, code, stdout)
		}
		if !strings.Contains(stderr, "cannot read version 9") || !strings.Contains(stderr, configPath) {
			t.Errorf("hergy %v does not name the broken file and why:\n%s", argv, stderr)
		}
	}
}
