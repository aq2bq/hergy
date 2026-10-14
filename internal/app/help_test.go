package app

import (
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
			[]string{"hergy new - チーム定義のひな形を作る", "--local", ".hergy/", "付けないと、グローバル", "config.yaml"},
			[]string{"--timeout", "--as", "--purpose"}},
		{[]string{"up", "--help"},
			[]string{"hergy up - Start a team", "--as <team>", "--timeout <ms>"},
			[]string{"--purpose", "--profile"}},
		{[]string{"hire", "-h", "--lang", "ja"},
			[]string{"hergy hire - ワーカーを追加する", "--purpose <目的>", "--task <依頼>", "--profile <プロファイル>", "--from <メンバー>"},
			[]string{"--as"}},
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
