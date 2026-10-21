//go:build e2e

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/chromedp/chromedp"
)

// TestE2EScreenPaths holds the team, role and profile screens to the way a
// problem's sentence writes a path (lead 2026-10-21): a file inside the project
// reads .hergy/…, a file of the layer every team shares reads under the home
// directory, and the machine's own path reads nowhere. The fixture keeps the
// shared layer at ~/.config/hergy, the way a person's machine has it, so both
// forms are the ones a person reads.
//
// A sentence that names the file it stands in leaves that name to the head the
// screen draws above it — the panel's own path, the YAML tab's file — and keeps
// the location it gives at its end and the names of the other files it is about.
// Two small files stand in a state hergy cannot open, the way a person's broken
// file stands, so each screen has a sentence that names a file.
func TestE2EScreenPaths(t *testing.T) {
	chrome := chromeBinary(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	f := newFixture(t)
	initLocales(t)
	// The layer every team shares stands under the home directory: with HOME and
	// the XDG settings both inside the fixture, a file of that layer reads
	// ~/.config/hergy/…, and a screen that wrote the machine's path would say so.
	root := filepath.Dir(f.proj)
	home := filepath.Join(root, "home")
	cfg := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	src := filepath.Join(cwd, "..", "..", "testdata", "teams-v2")
	copyDir(t, filepath.Join(src, "project", ".hergy"), filepath.Join(f.proj, ".hergy"))
	copyDir(t, filepath.Join(src, "config", "hergy"), filepath.Join(cfg, "hergy"))
	writeFile(t, filepath.Join(cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	scout := filepath.Join(f.proj, ".hergy", "roles", "scout.yaml")
	if err := i18n.Use("ja"); err != nil {
		t.Fatal(err)
	}
	f.runningHerdr(t, "", "")
	base, stop := startServe(t, f)
	defer stop()

	// The run's one browser, and this test's tab in it (r-a107).
	browser := e2eChrome(t, chrome)
	ctx, stopTab := e2eTab(t, browser, 4*time.Minute)
	defer stopTab()
	e2eRun(t, ctx, chromedp.EmulateViewport(2000, 1100))
	e2eThrottle(t, ctx)

	// The machine's own paths: no sentence a screen draws carries one.
	machine := []string{f.proj, cfg, home, f.state, root}
	noMachinePath := func(t *testing.T, screen, what, text string) {
		t.Helper()
		if strings.TrimSpace(text) == "" {
			t.Errorf("%s: %s draws no problem sentence, so the walk proves nothing", screen, what)
			return
		}
		for _, path := range machine {
			if strings.Contains(text, path) {
				t.Errorf("%s: %s names the machine's own path %s: %s", screen, what, path, text)
			}
		}
	}
	// everyProblem is every problem sentence one place draws: the check column of
	// a figure or a file, and the lists the shared parts fill.
	everyProblem := func(where string) string {
		return fmt.Sprintf(`[...document.querySelectorAll(%s + ' .vlist .vitem .msg, ' + %s + ' .vm li')].map(function(e){return e.textContent}).join('\n')`,
			strconv.Quote(where), strconv.Quote(where))
	}
	// open waits until the team row one name stands on is open with its check
	// column drawn, which the server fills a round trip after the row opens.
	open := func(t *testing.T, name string) string {
		t.Helper()
		e2eRun(t, ctx, chromedp.Navigate(base+"/teams?open="+name))
		panel := `.tdef[data-name="` + name + `"] .panel.team-p`
		e2eWait(t, ctx, `document.querySelectorAll(`+strconv.Quote(panel+` .vlist .vitem`)+`).length>0`)
		return panel
	}

	t.Run("team", func(t *testing.T) {
		e2eRun(t, ctx, chromedp.Navigate(base+"/teams"), chromedp.WaitReady("#view .tdef"))
		panel := open(t, "core")
		got := e2eTeamsValue(t, ctx, everyProblem(panel))
		noMachinePath(t, "the team screen", "the core panel", got)
		// The problem that stops a member's prompt names the role document a
		// person has to fix, in the form the panel heads a file with (X1).
		if !strings.Contains(got, ".hergy/roles/builder.yaml") {
			t.Errorf("the core panel does not name the role document a person fixes: %s", got)
		}
		// The panel's head names the definition's own team.yaml, so no sentence
		// opens with that name again.
		if strings.Contains(got, ".hergy/teams/core/team.yaml: ") {
			t.Errorf("the core panel names its own file in a sentence as well as in its head: %s", got)
		}

		// The agreement's own check column, on the ルール tab beside it.
		e2eRun(t, ctx, chromedp.Navigate(base+"/teams?open=core&agree=core"))
		agree := `.tdef[data-name="core"] .panel.agree-p`
		e2eWait(t, ctx, `document.querySelectorAll(`+strconv.Quote(agree+` .vm li`)+`).length>0`)
		noMachinePath(t, "the team screen", "the agreement panel", e2eTeamsValue(t, ctx, everyProblem(agree)))
	})

	t.Run("role", func(t *testing.T) {
		// A role document hergy cannot open, so the screen has a sentence that
		// names a file and the panel's head has one to spare.
		writeFile(t, scout, "version: 2\ntitle: 調査\nsections:\n  - heading: 調べ方\n    rules:\n      - id: SCOUT-1\n        text: 調べたことは、出典と一緒に 1 つの報告にまとめる。\n")
		e2eRun(t, ctx, chromedp.Navigate(base+"/roles/yaml?file=project.scout"), chromedp.WaitReady("#view .vlist"))
		e2eWait(t, ctx, `[...document.querySelectorAll('#view .vlist .vitem')].some(function(b){return b.textContent.indexOf('scout.yaml')>=0})`)
		got := e2eTeamsValue(t, ctx, everyProblem("#view"))
		noMachinePath(t, "the roles screen", "the scout file", got)
		if !strings.Contains(got, ".hergy/roles/scout.yaml") {
			t.Errorf("the roles screen does not name the file it reads: %s", got)
		}
		// The file's own name stands above the sentence, so the sentence does not
		// open with it again.
		if strings.Contains(got, "scout.yaml: ") {
			t.Errorf("the roles screen names its own file in a sentence as well as in its head: %s", got)
		}
	})

	t.Run("profiles", func(t *testing.T) {
		// The project's profiles.yaml emptied: a definition stops on it, and
		// both screens have a sentence about the file.
		writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"), "")

		panel := open(t, "core")
		got := e2eTeamsValue(t, ctx, everyProblem(panel))
		noMachinePath(t, "the team screen", "the core panel", got)
		if !strings.Contains(got, ".hergy/profiles.yaml") {
			t.Errorf("the core panel does not name the profiles file that stops it: %s", got)
		}
		// A sentence about a file other than the definition's own keeps that
		// file's name, and the panel's head keeps the definition's.
		if strings.Contains(got, ".hergy/teams/core/team.yaml: ") {
			t.Errorf("the core panel names its own file in a sentence as well as in its head: %s", got)
		}

		e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/yaml"), chromedp.WaitReady(".yed"))
		section := `.yed[data-file="project"]`
		e2eWait(t, ctx, `[...document.querySelectorAll(`+strconv.Quote(section+` .vlist .vitem`)+`)].length>0`)
		gotFiles := e2eTeamsValue(t, ctx, everyProblem(section))
		noMachinePath(t, "the profiles screen", "the project profiles.yaml", gotFiles)
		if !strings.Contains(gotFiles, ".hergy/profiles.yaml") {
			t.Errorf("the profiles screen does not name its own file the way a person reads it: %s", gotFiles)
		}
	})
}
