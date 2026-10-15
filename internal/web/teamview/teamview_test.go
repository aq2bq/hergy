package teamview

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/status"
	"github.com/aq2bq/hergy/internal/web"
)

// TestMain loads the locales, so the screens render the words a person reads.
func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// fakePort answers the screens from tables a test fills in: no project on disk,
// no herdr to call, and the one thing these screens change (the focus) written
// down instead of moved.
type fakePort struct {
	teams    map[string][]web.Team
	roots    map[string]string
	defs     map[string]Definition
	herdr    map[string]Herdr
	herdrErr error
	focusErr error
	focused  []string
}

func (p *fakePort) Teams(layer string) []web.Team { return p.teams[layer] }

func (p *fakePort) LayerRoot(layer string) string { return p.roots[layer] }

func (p *fakePort) Definition(_, name string) (Definition, error) {
	def, ok := p.defs[name]
	if !ok {
		return Definition{}, errors.New("no such team definition")
	}
	return def, nil
}

func (p *fakePort) Running(layer string) (Herdr, error) {
	if p.herdrErr != nil {
		return Herdr{}, p.herdrErr
	}
	return p.herdr[layer], nil
}

func (p *fakePort) Focus(pane string) error {
	if p.focusErr != nil {
		return p.focusErr
	}
	p.focused = append(p.focused, pane)
	return nil
}

// The keys the fixture's files are read under. They are paths, as a doc read
// from disk is keyed by, and the screens carry them in a URL.
const (
	sharedKey  = "/p/.hergy/docs/team.md"
	devRoleKey = "/p/.hergy/roles/dev.md"
)

// demoDefinition is a small team: two members with different roles, one of them
// with a role file and a worker, and rules that write all three kinds of turn
// direction (both ends named, one end named, neither).
func demoDefinition() Definition {
	return Definition{
		Name:  "demo",
		Layer: web.LayerLocal,
		Path:  ".hergy/teams/demo/team.yaml",
		Docs: []Doc{{
			Key:   sharedKey,
			Label: "チームの約束",
			Path:  ".hergy/docs/team.md",
			Rules: []Rule{
				{ID: "C1", Text: "リードは、割り振りを決めます。"},
				{ID: "C2", Text: "リード ⇄ 実装"},
				{ID: "C3", Text: "依頼主 → リード"},
			},
		}},
		Members: []Member{
			{
				Name:    "lead",
				Herdr:   "demo-lead",
				Role:    "リード",
				Kind:    "claude",
				Profile: "heavy",
				Prompt:  "**役割**: リード。\n`herdr agent status` で様子を見ます。",
			},
			{
				Name:    "dev",
				Herdr:   "demo-dev",
				Role:    "実装",
				Kind:    "opencode",
				Workers: []Worker{{Purpose: "調べものを手伝う", Profile: "quick"}},
				RoleFile: &Doc{
					Key:   devRoleKey,
					Label: "実装の役割",
					Path:  ".hergy/roles/dev.md",
					Role:  "実装",
					Rules: []Rule{
						{ID: "D1", Text: "実装 → リード"},
						{ID: "D3", Text: "経理 → 総務"},
					},
				},
			},
		},
	}
}

// demoHerdr is Herdr saying the demo team is running: two members and one hired
// worker, with the states, the terminal titles and the panes at hand.
func demoHerdr() Herdr {
	return Herdr{
		At: time.Date(2026, 10, 15, 9, 30, 0, 0, time.UTC),
		Teams: []LiveTeam{{
			Team:       "demo",
			Definition: "demo",
			TabID:      "t-demo",
			Known:      true,
			Rows: []status.Row{
				{Name: "demo-lead", State: "working", Live: true, PaneID: "p1", Title: "設計を書いている", Role: "リード", Kind: "claude"},
				{Name: "demo-dev", State: "blocked", Live: true, PaneID: "p2", Role: "実装", Kind: "opencode"},
				{Name: "demo-helper", State: "done", Live: true, Worker: true, PaneID: "p3", Title: "調べものを終えた"},
			},
		}},
	}
}

func demoPort() *fakePort {
	return &fakePort{
		teams: map[string][]web.Team{
			web.LayerLocal: {
				{Name: "demo", Members: 2, Rules: 5, NoCheck: 1, Running: true},
				{Name: "other", Members: 1},
			},
		},
		roots: map[string]string{web.LayerLocal: ".hergy"},
		defs:  map[string]Definition{"demo": demoDefinition()},
		herdr: map[string]Herdr{web.LayerLocal: demoHerdr()},
	}
}

// client is one HTTP client for every test: it keeps no connection open, so a
// test leaves nothing running behind it.
var client = &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}

// start runs the three screens on a real, ephemeral port and returns the address
// a browser would use. The server is stopped, and waited for, when the test ends.
func start(t *testing.T, port interface {
	Port
	web.Port
}) string {
	t.Helper()
	screens, err := Screens(port)
	if err != nil {
		t.Fatalf("Screens: %v", err)
	}
	server, err := web.New(port, screens...)
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	urls := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, 0, func(url string) { urls <- url }) }()
	var base string
	select {
	case base = <-urls:
	case err := <-done:
		t.Fatalf("Serve stopped: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not report its URL")
	}
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Serve stopped with %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("Serve did not stop")
		}
	})
	return strings.TrimSuffix(base, "/")
}

func get(t *testing.T, base, path string) (int, string) {
	t.Helper()
	resp, err := client.Get(base + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp.StatusCode, string(body)
}

func post(t *testing.T, base, path, body string) int {
	t.Helper()
	resp, err := client.Post(base+path, "application/x-www-form-urlencoded", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp.StatusCode
}

func mustContain(t *testing.T, what, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("%s does not contain %q:\n%s", what, want, body)
		}
	}
}

func mustNotContain(t *testing.T, what, body string, unwanted ...string) {
	t.Helper()
	for _, bad := range unwanted {
		if strings.Contains(body, bad) {
			t.Errorf("%s contains %q:\n%s", what, bad, body)
		}
	}
}

// TestDefinitionScreenShowsWhatTheDefinitionHolds: the members and the workers
// they may call, each with the one line that says what it is, and the button that
// opens a member's prompt in the panel.
func TestDefinitionScreenShowsWhatTheDefinitionHolds(t *testing.T) {
	base := start(t, demoPort())
	code, body := get(t, base, "/team?layer=local")

	if code != http.StatusOK {
		t.Fatalf("GET /team = %d, want 200", code)
	}
	mustContain(t, "the definition screen", body,
		i18n.T("web.teamview.definition.title"),
		"チームの約束", "規則 3 件",
		"lead", "dev", "リード", "実装", "heavy", "調べものを手伝う", "quick",
		i18n.T("web.teamview.members.meaning"), i18n.T("web.teamview.workers.meaning"),
		"/panel/member?layer=local", "member=lead\"",
		`title="`+i18n.T("web.teams.running")+`"`,
	)
}

// TestDefinitionScreenSaysWhenTheLayerIsEmpty: a layer with no team definition
// says so and asks the app for nothing.
func TestDefinitionScreenSaysWhenTheLayerIsEmpty(t *testing.T) {
	base := start(t, demoPort())
	code, body := get(t, base, "/team?layer=global")

	if code != http.StatusOK {
		t.Fatalf("GET /team?layer=global = %d, want 200", code)
	}
	mustContain(t, "the empty definition screen", body, i18n.T("web.teamview.no_definition"))
	mustNotContain(t, "the empty definition screen", body, i18n.T("label.members"))
}

// TestMemberPanelShowsThePromptAndTheLiveState: the panel carries the settings,
// what Herdr says of the member right now, the workers it may call, and the
// prompt as it is sent, with only bold and code drawn (spec/10 の 4 節).
func TestMemberPanelShowsThePromptAndTheLiveState(t *testing.T) {
	base := start(t, demoPort())
	code, body := get(t, base, "/panel/member?layer=local&name=demo&member=lead")

	if code != http.StatusOK {
		t.Fatalf("GET /panel/member = %d, want 200", code)
	}
	mustContain(t, "the member panel", body,
		"lead", "demo-lead", "リード", "claude", "heavy",
		"working", i18n.T("web.teamview.state.working"), "設計を書いている", "ペイン p1",
		"<strong>役割</strong>", "<code>herdr agent status</code>",
		i18n.T("web.teamview.panel.no_role_file"),
	)
}

// TestMemberPanelShowsTheRoleFileAndTheWorkers: a member that reads a role file
// shows it, and a member that may hire a worker shows what for.
func TestMemberPanelShowsTheRoleFileAndTheWorkers(t *testing.T) {
	base := start(t, demoPort())
	_, body := get(t, base, "/panel/member?layer=local&name=demo&member=dev")

	mustContain(t, "the member panel", body,
		"実装の役割", "調べものを手伝う", "quick",
		"blocked", i18n.T("web.teamview.state.blocked"),
		i18n.Tf("status.human_waiting", "demo-dev"),
	)
}

// TestPanelSaysWhenALinkIsStale: a member or a rule that is not there any more
// answers with the reason in the panel, so a swap shows why instead of nothing.
func TestPanelSaysWhenALinkIsStale(t *testing.T) {
	base := start(t, demoPort())
	key := url.QueryEscape(sharedKey)
	cases := []struct {
		what string
		path string
		want string
	}{
		{"an unknown member", "/panel/member?layer=local&name=demo&member=nobody", i18n.Tf("web.teamview.panel.no_member", "nobody")},
		{"an unknown document", "/panel/doc?layer=local&name=demo&key=nothing", i18n.Tf("web.teamview.panel.no_doc", "nothing")},
		{"an unknown rule", "/panel/rule?layer=local&name=demo&doc=" + key + "&rule=X9", i18n.Tf("web.teamview.panel.no_rule", "X9")},
	}
	for _, tc := range cases {
		t.Run(tc.what, func(t *testing.T) {
			code, body := get(t, base, tc.path)
			if code != http.StatusOK {
				t.Errorf("GET %s = %d, want 200", tc.path, code)
			}
			mustContain(t, tc.what, body, tc.want)
			mustNotContain(t, tc.what, body, "<!doctype html>")
		})
	}
}

// TestDocPanelListsTheRulesAndTheRulePanelShowsOne: the document panel lists the
// rules of one file and a rule's id opens its text, which is where the figure's
// arrows point.
func TestDocPanelListsTheRulesAndTheRulePanelShowsOne(t *testing.T) {
	base := start(t, demoPort())
	key := url.QueryEscape(sharedKey)

	code, body := get(t, base, "/panel/doc?layer=local&name=demo&key="+key)
	if code != http.StatusOK {
		t.Fatalf("GET /panel/doc = %d, want 200", code)
	}
	mustContain(t, "the document panel", body,
		"チームの約束", ".hergy/docs/team.md", "C1", "C2", "C3", "リードは、割り振りを決めます。")

	_, body = get(t, base, "/panel/rule?layer=local&name=demo&doc="+key+"&rule=C2")
	mustContain(t, "the rule panel", body, "C2", "リード ⇄ 実装")
}

// TestRunningScreenShowsWhatHerdrSays: one row per member and worker, with the
// state hergy status prints, what the terminal says, the pane, and the judgement
// over the team.
func TestRunningScreenShowsWhatHerdrSays(t *testing.T) {
	base := start(t, demoPort())
	code, body := get(t, base, "/running?layer=local")

	if code != http.StatusOK {
		t.Fatalf("GET /running = %d, want 200:\n%s", code, body)
	}
	mustContain(t, "the running screen", body,
		i18n.T("web.teamview.running.title"),
		"demo", i18n.T("label.definition")+" demo", "t-demo",
		"demo-lead", "working", "demo-dev", "blocked", "demo-helper", i18n.T("web.teamview.worker"),
		"設計を書いている", "ペイン p2",
		i18n.Tf("status.human_waiting", "demo-dev"),
		i18n.T("web.teamview.running.focus"),
		i18n.T("web.teamview.state.working"), i18n.T("status.not_live"),
	)
	// Every state a row can write is explained under the list.
	for _, meaning := range []string{
		i18n.T("web.teamview.state.working"), i18n.T("web.teamview.state.idle"),
		i18n.T("web.teamview.state.blocked"), i18n.T("web.teamview.state.done"),
	} {
		if !strings.Contains(body, meaning) {
			t.Errorf("the legend does not explain %q", meaning)
		}
	}
}

// TestFocusMovesHerdrAndTheTokenGuardsIt: the one button of these screens posts
// the pane with the token of the run, and a post without it moves nothing.
func TestFocusMovesHerdrAndTheTokenGuardsIt(t *testing.T) {
	port := demoPort()
	base := start(t, port)
	_, body := get(t, base, "/running?layer=local")

	token := tokenOf(t, body)
	if code := post(t, base, "/running/focus?layer=local", "token="+url.QueryEscape(token)+"&pane=p3"); code != http.StatusOK {
		t.Fatalf("POST /running/focus = %d, want 200", code)
	}
	if len(port.focused) != 1 || port.focused[0] != "p3" {
		t.Fatalf("the focus moved to %v, want [p3]", port.focused)
	}

	if code := post(t, base, "/running/focus?layer=local", "pane=p2"); code != http.StatusForbidden {
		t.Errorf("a post without the token = %d, want 403", code)
	}
	if len(port.focused) != 1 {
		t.Errorf("a post without the token moved the focus to %v", port.focused)
	}
}

// TestRunningScreenShowsAFocusThatFailed: HTMX does not swap a failed answer, so
// a focus that does not move is one line at the top of the list, not an error
// page.
func TestRunningScreenShowsAFocusThatFailed(t *testing.T) {
	port := demoPort()
	port.focusErr = errors.New("herdr is not running")
	base := start(t, port)
	_, body := get(t, base, "/running?layer=local")
	token := tokenOf(t, body)

	code := post(t, base, "/running/focus?layer=local", "token="+url.QueryEscape(token)+"&pane=p3")
	if code != http.StatusOK {
		t.Fatalf("POST /running/focus = %d, want 200", code)
	}
}

// TestScreensSayWhenHerdrCannotBeRead: Herdr missing or unreadable is one line on
// the running screen and the other screens keep working.
func TestScreensSayWhenHerdrCannotBeRead(t *testing.T) {
	port := demoPort()
	port.herdrErr = errors.New("herdr is not running")
	base := start(t, port)

	code, body := get(t, base, "/running?layer=local")
	if code != http.StatusOK {
		t.Fatalf("GET /running = %d, want 200:\n%s", code, body)
	}
	mustContain(t, "the running screen", body,
		i18n.Tf("web.teamview.herdr_failed", "herdr is not running"),
		i18n.T("web.teamview.running.none"))
	mustNotContain(t, "the running screen", body, "demo-lead")

	code, body = get(t, base, "/team?layer=local")
	if code != http.StatusOK {
		t.Fatalf("GET /team = %d, want 200", code)
	}
	mustContain(t, "the definition screen", body, "lead")
}

// TestFigureDrawsTheTurnDirectionsItReads: the figure carries a box for every
// document, member and worker, an arrow for every turn direction with a name on
// it, and the states Herdr reports; a direction that matches no name is listed
// under the figure instead of drawn.
func TestFigureDrawsTheTurnDirectionsItReads(t *testing.T) {
	base := start(t, demoPort())
	code, body := get(t, base, "/figure?layer=local&name=demo")

	if code != http.StatusOK {
		t.Fatalf("GET /figure = %d, want 200", code)
	}
	mustContain(t, "the figure", body,
		"<svg", "チームの約束", "リード", "実装", "調べものを手伝う",
		i18n.T("web.teamview.figure.human"), i18n.T("web.teamview.figure.no_role_file"),
		// The three directions that name a member: two solid, and the one
		// whose far end is not in the team, which is dotted.
		"C2", "C3", "D1", "tp-edge any", "依頼主",
		// The two states Herdr reports, on the members.
		"st-working", "st-blocked", "working", "blocked",
		// A box opens a panel, and an arrow opens the rule it came from.
		"/panel/member?layer=local", "member=lead\"", "/panel/rule?layer=local", "rule=C2\"",
		// A ⇄ is drawn with a head at both ends.
		`marker-end="url(#tp-end)"`, `marker-start="url(#tp-start)"`,
	)
	mustContain(t, "the dropped directions", body,
		i18n.T("web.teamview.figure.unread"), "D3", "経理", "総務")
	mustNotContain(t, "the figure", body, i18n.T("web.teamview.figure.no_arrows"))
}

// TestFigureSaysWhenTheDefinitionIsNotRunning: a definition no team is booted
// from draws no state and says so.
func TestFigureSaysWhenTheDefinitionIsNotRunning(t *testing.T) {
	port := demoPort()
	port.herdr = map[string]Herdr{}
	base := start(t, port)
	_, body := get(t, base, "/figure?layer=local&name=demo")

	mustContain(t, "the figure", body, i18n.T("web.teamview.figure.not_running"), "C2")
	mustNotContain(t, "the figure", body, "st-working")
}

// TestOnlyTheFocusIsAWrite: these screens read. The one write is the button that
// moves Herdr's focus, and the figure's boxes are fragments of the same read.
func TestOnlyTheFocusIsAWrite(t *testing.T) {
	screens, err := Screens(demoPort())
	if err != nil {
		t.Fatalf("Screens: %v", err)
	}
	writes, assets := 0, 0
	for _, screen := range screens {
		if _, ok := screen.(web.ScreenAssets); ok {
			assets++
		}
		for _, route := range screen.Routes() {
			if !route.Write {
				continue
			}
			writes++
			if route.Path != "/running/focus" {
				t.Errorf("the write route is %q, want /running/focus", route.Path)
			}
		}
	}
	if writes != 1 {
		t.Errorf("%d write routes, want 1", writes)
	}
	if assets != 1 {
		t.Errorf("%d screens bring files, want 1 (the frame refuses two with one name)", assets)
	}
}

// tokenOf reads the token of the run out of a page, the way a browser does when
// it posts the form.
func tokenOf(t *testing.T, body string) string {
	t.Helper()
	match := tokenRE.FindStringSubmatch(body)
	if match == nil {
		t.Fatalf("the page carries no token:\n%s", body)
	}
	return match[1]
}

var tokenRE = regexp.MustCompile(`name="token" value="([^"]+)"`)
