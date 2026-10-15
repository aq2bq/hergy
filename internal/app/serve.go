package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aq2bq/hergy/internal/config"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
	"github.com/aq2bq/hergy/internal/web/homeview"
	"github.com/aq2bq/hergy/internal/web/rulesview"
	"github.com/aq2bq/hergy/internal/web/teamview"
)

// servePort is what the app hands the screens: the same layers, the same team
// definitions and the same rule files the commands read. The screens ask here
// and never reach around it, which is how internal/web stays out of the CLI's
// edge and out of hergy's top layer.
type servePort struct {
	env team.Env
	// timeout is how long hergy waits for an agent to come up, the same value
	// the commands take --timeout for. The running-team screen hands it to the
	// launcher that reads the live state.
	timeout int
}

// Teams lists one layer's team definitions with what the team list shows: how
// many members team.yaml writes, how many rules those members read, how many of
// those rules have no check, when the definition was last launched here with the
// hash that launch wrote, and whether a team runs from it now. A definition that
// cannot be read keeps its row and carries why, written the way hergy validate
// writes a broken file.
//
// A definition Herdr reports a running team for is marked, so the list says
// which definitions are in use. A Herdr that cannot be read marks none of them
// and the list is the same list it was.
func (p servePort) Teams(layer string) []web.Team {
	records := readRecords()
	running := p.runningDefinitions()
	var out []web.Team
	for _, row := range p.env.List() {
		if !inLayer(row, layer) {
			continue
		}
		cfg, err := validateTeam(p.env, row.Name, layer)
		if err != nil {
			out = append(out, web.Team{
				Name:    row.Name,
				Href:    teamHref(layer, row.Name),
				Running: running[row.Name],
				Broken:  breakLine(p.env, listingPath(row, layer), err),
			})
			continue
		}
		tally := ruleTally(cfg)
		team := p.launchNote(records, cfg, row.Name, layer)
		team.Name = row.Name
		team.Href = teamHref(layer, row.Name)
		team.Members = len(cfg.Members)
		team.Rules = tally.Total
		team.NoCheck = tally.Total - tally.WithCheck
		team.Running = running[row.Name]
		out = append(out, team)
	}
	return out
}

// runningDefinitions is the team definition names a team runs from right now.
// Herdr missing or unreadable leaves it empty: the team list is a list of files
// first, and a team that is not running is not an error.
func (p servePort) runningDefinitions() map[string]bool {
	tabs, err := herdrTabList()
	if err != nil {
		return nil
	}
	out := map[string]bool{}
	for _, entry := range team.RunningTeams(tabLabels(tabs)) {
		out[entry.Definition] = true
	}
	return out
}

// Definition reads one team definition of one layer in full, the way hergy show
// reads it: the documents every member reads, and the members with the role file
// they read and the prompt hergy hands them. A definition that cannot be read
// comes back with Broken set and nothing else, so the screen shows the same one
// line hergy validate writes.
func (p servePort) Definition(layer, name string) (teamview.Definition, error) {
	resolved, err := p.env.Resolve(name, layer)
	if err != nil {
		return teamview.Definition{}, err
	}
	out := teamview.Definition{
		Name:  name,
		Layer: layer,
		Path:  p.env.Display(resolved.TeamYAMLPath),
	}
	cfg, err := p.env.BuildConfig(resolved, source.Disk{}, "")
	if err != nil {
		out.Broken = breakLine(p.env, resolved.TeamYAMLPath, err)
		return out, nil
	}
	for _, doc := range cfg.Docs {
		out.Docs = append(out.Docs, p.readDoc(doc, ""))
	}
	for i := range cfg.Members {
		member := &cfg.Members[i]
		entry := teamview.Member{
			Name:    member.ShortName,
			Herdr:   member.Name,
			Role:    member.Role,
			Kind:    member.Kind,
			Profile: member.Profile,
			Prompt:  member.Prompt,
		}
		if member.MaySpawn != nil {
			for _, worker := range member.MaySpawn.Workers {
				entry.Workers = append(entry.Workers, teamview.Worker{
					Purpose: worker.Purpose,
					Profile: worker.Profile,
				})
			}
		}
		if member.HasRoleSource {
			doc := p.readDoc(member.RoleSource, member.Role)
			entry.RoleFile = &doc
		}
		out.Members = append(out.Members, entry)
	}
	return out, nil
}

// Running reads Herdr the way hergy status reads it: the teams it reports, and,
// for each one this layer can build, one row per member and worker with the
// state Herdr reports and what the terminal says. A team booted from another
// project's definition is listed without its rows, as hergy status lists it.
func (p servePort) Running(layer string) (teamview.Herdr, error) {
	tabs, err := herdrTabList()
	if err != nil {
		return teamview.Herdr{}, err
	}
	out := teamview.Herdr{At: time.Now()}
	for _, entry := range team.RunningTeams(tabLabels(tabs)) {
		live := teamview.LiveTeam{
			Team:       entry.Team,
			Definition: entry.Definition,
			TabID:      tabIDOf(tabs, entry),
		}
		resolved, err := p.env.Resolve(entry.Definition, layer)
		if err != nil {
			out.Teams = append(out.Teams, live)
			continue
		}
		cfg, err := p.env.BuildConfig(resolved, source.Disk{}, entry.Team)
		if err != nil {
			live.Known = true
			live.Broken = breakLine(p.env, resolved.TeamYAMLPath, err)
			out.Teams = append(out.Teams, live)
			continue
		}
		members, err := cfg.Select("")
		if err != nil {
			return teamview.Herdr{}, err
		}
		rows, err := newLauncher(cfg, p.timeout, io.Discard).Status(members)
		if err != nil {
			return teamview.Herdr{}, err
		}
		live.Known = true
		live.Rows = rows
		out.Teams = append(out.Teams, live)
	}
	return out, nil
}

// Focus moves Herdr's focus to one pane. It is the only thing the screens
// change, and the only reason hergy serve calls Herdr for anything but reading.
func (p servePort) Focus(pane string) error {
	if pane == "" {
		return i18n.Errorf("error.focus_no_pane")
	}
	port := herdrcli.New(os.Getenv("HERGY_HERDR"))
	return herdr.New(port).Focus(pane)
}

// LayerRoot is where one layer's files live, as a person writes it.
func (p servePort) LayerRoot(layer string) string {
	if layer == web.LayerGlobal {
		return p.env.Display(p.env.GlobalDir)
	}
	return p.env.Display(p.env.LocalDir)
}

// readDoc reads one rule file for the screens: what it calls itself, which layer
// it lives in, the rules in it, and why it cannot be read when it cannot. A file
// that does not read is still a file the screens show; the reason travels with
// it.
func (p servePort) readDoc(doc source.Doc, role string) teamview.Doc {
	out := teamview.Doc{
		Key:   doc.Path,
		Layer: doc.Layer,
		Label: filepath.Base(doc.Path),
		Path:  p.env.Display(doc.Path),
		Role:  role,
	}
	text, err := os.ReadFile(doc.Path)
	if err != nil {
		out.Broken = breakLine(p.env, doc.Path, err)
		return out
	}
	parsed, err := rules.Parse(string(text), doc.Path)
	if err != nil {
		out.Broken = breakLine(p.env, doc.Path, err)
		return out
	}
	if parsed.Title != "" {
		out.Label = parsed.Title
	}
	for _, rule := range parsed.Rules() {
		out.Rules = append(out.Rules, teamview.Rule{ID: rule.ID, Text: rule.Text})
	}
	return out
}

// herdrTabList lists the tabs Herdr reports, with the ids the WebUI shows for a
// running team and the labels the team is found by.
func herdrTabList() ([]protocol.Tab, error) {
	port := herdrcli.New(os.Getenv("HERGY_HERDR"))
	return herdr.New(port).Tabs()
}

// tabLabels is the tab labels, in Herdr's order.
func tabLabels(tabs []protocol.Tab) []string {
	out := make([]string, len(tabs))
	for i, tab := range tabs {
		out[i] = tab.Label
	}
	return out
}

// tabIDOf finds the tab one running team sits in, by the label hergy gave it.
func tabIDOf(tabs []protocol.Tab, entry team.Running) string {
	for _, tab := range tabs {
		if tab.Label == "team:"+entry.Team || tab.Label == "team:"+entry.Team+"@"+entry.Definition {
			return tab.ID
		}
	}
	return ""
}

// listingPath is the team.yaml of one layer's row of the listing.
func listingPath(row team.Listing, layer string) string {
	if layer == web.LayerGlobal {
		return row.GlobalPath
	}
	return row.LocalPath
}

// breakLine writes one broken settings file as one line: where it is, as a
// person writes it, and why, with the whole reason on one line. hergy validate
// and the web UI both show a broken file this way.
func breakLine(env team.Env, path string, err error) string {
	msg := strings.ReplaceAll(err.Error(), path, env.Display(path))
	// Some team-file errors do not name the file; the file-level parser errors
	// already do. Show the location once either way.
	if !strings.Contains(msg, env.Display(path)) {
		msg = fmt.Sprintf("%s: %s", env.Display(path), msg)
	}
	return strings.Join(strings.Fields(msg), " ")
}

// runServe serves the screens on this machine and prints the URL once the socket
// is open. It runs in the foreground until Ctrl-C, then waits for the requests
// in flight. It reads the settings the way the other commands read them and
// changes no file the screens do not write.
//
// The order here is the order of the navigation, and it follows the daily flow:
// the home first, because opening hergy serve lands on it (d-ff49), then the team
// list, then the rules screen, then the record's other screens: the prompt diff,
// then the version screen. The screens that read a running team come after those.
// A screen of another part is added on this line, in its place.
func runServe(ctx context.Context, env team.Env, opts *options.Options, stdout io.Writer) error {
	port := servePort{env: env, timeout: opts.Timeout}
	home, err := homeview.New(port)
	if err != nil {
		return err
	}
	teams, err := web.NewTeamList(port)
	if err != nil {
		return err
	}
	rulesScreen, err := rulesview.New(rulesPort{port})
	if err != nil {
		return err
	}
	diff, err := homeview.NewDiff(port)
	if err != nil {
		return err
	}
	bump, err := homeview.NewBump(port)
	if err != nil {
		return err
	}
	teamScreens, err := teamview.Screens(port)
	if err != nil {
		return err
	}
	screens := append([]web.Screen{home, teams, rulesScreen, diff, bump}, teamScreens...)
	server, err := web.New(port, orderScreens(screens)...)
	if err != nil {
		return err
	}
	return server.Serve(ctx, opts.Port, func(url string) {
		fmt.Fprintln(stdout, i18n.Tf("serve.url", url))
	})
}

// orderScreens puts the screens in the order the navigation shows them (d-207f):
// the home first, then the running teams, the team list, the figure, the rules,
// the prompt diff and the version. The team definition screen is last: it is not
// a tab (web.NavScreen), and it is opened from the left list. The order lives
// here and not in the callers, so a screen added by another part lands in its
// place in one edit.
func orderScreens(screens []web.Screen) []web.Screen {
	out := append([]web.Screen(nil), screens...)
	sort.SliceStable(out, func(i, j int) bool { return navRank(out[i]) < navRank(out[j]) })
	return out
}

// navRank is where a screen stands in the navigation, told from the path it
// answers on. A path this list does not know comes last, so an unknown screen
// never pushes a known one around.
func navRank(screen web.Screen) int {
	routes := screen.Routes()
	if len(routes) == 0 {
		return len(navOrder)
	}
	if rank, ok := navOrder[hrefOf(routes[0].Path)]; ok {
		return rank
	}
	return len(navOrder)
}

// navOrder is the navigation's order, by the path of each screen's first route
// (d-207f: the order the prototype shows, without the map and the triage).
var navOrder = map[string]int{
	"/": 0, "/running": 1, "/teams": 2, "/figure": 3, "/rules": 4, "/diff": 5, "/bump": 6, "/team": 7,
}

// hrefOf writes a route's pattern the way a browser asks for it: the method
// dropped, and the exact root written as "/".
func hrefOf(pattern string) string {
	if i := strings.IndexByte(pattern, ' '); i >= 0 {
		pattern = pattern[i+1:] // a "GET /x" pattern
	}
	return strings.Replace(pattern, "/{$}", "/", 1)
}

// ruleTally counts what one team's members read, the way hergy validate counts
// the whole machine.
func ruleTally(cfg *config.Config) *rules.Tally {
	tally := &rules.Tally{}
	collectRules(tally, map[string]bool{}, cfg)
	return tally
}
