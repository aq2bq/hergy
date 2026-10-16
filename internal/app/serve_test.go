package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
)

// initLocales loads the locales for the tests that call the pieces of a run
// rather than Run itself: a team's built-in prompt template comes from the
// locale, so without it the members read no document and count no rule.
func initLocales(t *testing.T) {
	t.Helper()
	if err := i18n.Init(); err != nil {
		t.Fatal(err)
	}
}

// serveEnv is the environment the web UI reads, built the way Run builds it.
func serveEnv(t *testing.T) team.Env {
	t.Helper()
	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	return env
}

// writeDemoTeam writes one project-local team definition whose two members read
// the agreement and a role file each, with two of the four rules carrying a
// check.
func writeDemoTeam(t *testing.T, f *fixture, name string) {
	t.Helper()
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", name, "team.yaml"), `version: 1
docs:
  - protocol.yaml
roles: [roles]
members:
  - name: lead
    role: lead
    kind: claude
  - name: builder
    role: builder
    kind: claude
`)
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", name, "protocol.yaml"), `version: 1
sections:
  - rules:
      - id: C1
        text: one
        report: audit.scope
      - id: C2
        text: two
reports:
  audit:
    where: the report
    fields:
      scope: the scope
`)
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", name, "roles", "lead.yaml"), `version: 1
sections:
  - rules:
      - id: L1
        text: three
        tests: [T-1]
`)
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", name, "roles", "builder.yaml"), `version: 1
sections:
  - rules:
      - id: B1
        text: four
`)
}

// TestServePortTeams: the team list reads a layer's definitions the way the
// commands read them. Each row carries the members team.yaml writes, the rules
// those members read (a file two members read counts once), and how many of
// those rules have no check. A definition that cannot be read keeps its row and
// carries why, written the way hergy validate writes it.
func TestServePortTeams(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeDemoTeam(t, f, "demo")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "broken", "team.yaml"), "version: 2\n")
	writeFile(t, filepath.Join(f.cfg, "hergy", "teams", "shared", "team.yaml"), `version: 1
members:
  - name: lead
    role: lead
    kind: claude
`)
	writeFile(t, filepath.Join(f.cfg, "hergy", "teams", "shared", "roles", "lead.yaml"), `version: 1
sections:
  - rules:
      - id: S1
        text: one
`)
	p := servePort{env: serveEnv(t)}

	local := p.Teams(web.LayerLocal)
	if len(local) != 2 {
		t.Fatalf("the local layer has %d team definition(s), want 2: %+v", len(local), local)
	}
	broken, demo := local[0], local[1]
	if broken.Name != "broken" || demo.Name != "demo" {
		t.Fatalf("the rows are %q and %q, want them by name", broken.Name, demo.Name)
	}
	if broken.Broken == "" {
		t.Error("a definition that cannot be read has no reason on its row")
	}
	if !strings.Contains(broken.Broken, ".hergy/teams/broken/team.yaml") {
		t.Errorf("the reason does not name the file, as hergy validate does: %q", broken.Broken)
	}
	if strings.Contains(broken.Broken, "\n") {
		t.Errorf("the reason is not one line: %q", broken.Broken)
	}
	if broken.Members != 0 || broken.Rules != 0 {
		t.Errorf("a definition that cannot be read carries counts (%d, %d)", broken.Members, broken.Rules)
	}

	if demo.Broken != "" {
		t.Errorf("a definition that reads carries a reason: %q", demo.Broken)
	}
	if demo.Members != 2 {
		t.Errorf("demo has %d members, want 2", demo.Members)
	}
	if demo.Rules != 4 || demo.NoCheck != 2 {
		t.Errorf("demo reads %d rules with %d of them unchecked, want 4 and 2", demo.Rules, demo.NoCheck)
	}

	global := p.Teams(web.LayerGlobal)
	if len(global) != 1 || global[0].Name != "shared" {
		t.Fatalf("the global layer is %+v, want the one shared definition", global)
	}
	if global[0].Members != 1 || global[0].Rules != 1 || global[0].NoCheck != 1 {
		t.Errorf("shared reads %+v, want one member and the one rule of its role file", global[0])
	}
}

// TestServePortLayerRoot: the frame shows where the layer being displayed writes,
// as a person writes it.
func TestServePortLayerRoot(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	p := servePort{env: serveEnv(t)}
	if got := p.LayerRoot(web.LayerLocal); got != ".hergy" {
		t.Errorf("the local root is %q, want .hergy", got)
	}
	if got, want := p.LayerRoot(web.LayerGlobal), filepath.Join(f.cfg, "hergy"); got != want {
		t.Errorf("the global root is %q, want %q", got, want)
	}
}

// TestServePrintsTheURLAndStops: hergy serve prints the URL once the socket is
// open, in the language in use, and stops when its context is canceled (which is
// what Ctrl-C does) without reporting a failure.
func TestServePrintsTheURLAndStops(t *testing.T) {
	newFixture(t)
	initLocales(t)
	fakeBrowser(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var out, errOut strings.Builder
	if err := runServe(ctx, serveEnv(t), &options.Options{Port: 0}, &out, &errOut); err != nil {
		t.Fatalf("serve stopped with %v, want nil", err)
	}
	got := out.String()
	if !strings.HasSuffix(got, "\n") || strings.Count(got, "\n") != 1 {
		t.Fatalf("serve printed %q, want one line", got)
	}
	want := i18n.Tf("serve.url", "http://127.0.0.1:")
	if !strings.HasPrefix(got, want) {
		t.Errorf("serve printed %q, want a line starting with %q", got, want)
	}
	if errOut.String() != "" {
		t.Errorf("serve wrote %q to stderr, want nothing", errOut.String())
	}
}

// fakeBrowser stands in for the person's browser in a serve test: it records
// what runServe asked to open and answers with the error the test chose, so no
// browser opens on the machine the tests run on.
func fakeBrowser(t *testing.T, openErr error) *[]string {
	t.Helper()
	var opened []string
	prev := openBrowser
	openBrowser = func(url string) error {
		opened = append(opened, url)
		return openErr
	}
	t.Cleanup(func() { openBrowser = prev })
	return &opened
}

// serveOnce runs hergy serve with a context that is already canceled (which is
// what Ctrl-C does) and gives back what it printed on both streams.
func serveOnce(t *testing.T, opts *options.Options) (out, errOut string) {
	t.Helper()
	newFixture(t)
	initLocales(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr strings.Builder
	if err := runServe(ctx, serveEnv(t), opts, &stdout, &stderr); err != nil {
		t.Fatalf("serve stopped with %v, want nil", err)
	}
	return stdout.String(), stderr.String()
}

// TestServeHandsTheURLToTheBrowser: once the URL is printed, hergy serve opens
// it in the person's browser (d-fb89), and says nothing more.
func TestServeHandsTheURLToTheBrowser(t *testing.T) {
	opened := fakeBrowser(t, nil)
	out, errOut := serveOnce(t, &options.Options{Port: 0})

	if len(*opened) != 1 {
		t.Fatalf("serve opened %q, want the one URL it printed", *opened)
	}
	url := (*opened)[0]
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Errorf("serve opened %q, want this machine's own address", url)
	}
	if !strings.Contains(out, url) {
		t.Errorf("serve opened %q but printed %q", url, out)
	}
	if errOut != "" {
		t.Errorf("serve wrote %q to stderr, want nothing", errOut)
	}
}

// TestServeNoOpenPrintsTheURLOnly: with --no-open the URL is printed and nothing
// is opened.
func TestServeNoOpenPrintsTheURLOnly(t *testing.T) {
	opened := fakeBrowser(t, nil)
	out, errOut := serveOnce(t, &options.Options{Port: 0, NoOpen: true})

	if len(*opened) != 0 {
		t.Errorf("hergy serve --no-open opened %q, want nothing", *opened)
	}
	if want := i18n.Tf("serve.url", "http://127.0.0.1:"); !strings.HasPrefix(out, want) {
		t.Errorf("hergy serve --no-open printed %q, want a line starting with %q", out, want)
	}
	if errOut != "" {
		t.Errorf("hergy serve --no-open wrote %q to stderr, want nothing", errOut)
	}
}

// TestServeGoesOnWhenNoBrowserOpens: a browser that cannot be started is one
// line on stderr carrying the URL, and serve goes on answering (d-fb89).
func TestServeGoesOnWhenNoBrowserOpens(t *testing.T) {
	opened := fakeBrowser(t, errors.New("no xdg-open here"))
	out, errOut := serveOnce(t, &options.Options{Port: 0})

	if len(*opened) != 1 {
		t.Fatalf("serve opened %q, want it to try the one URL", *opened)
	}
	if want := i18n.Tf("serve.no_browser", (*opened)[0]); !strings.Contains(errOut, want) {
		t.Errorf("serve wrote %q to stderr, want %q", errOut, want)
	}
	if !strings.Contains(out, (*opened)[0]) {
		t.Errorf("serve printed %q, want the URL %q on it too", out, (*opened)[0])
	}
}
