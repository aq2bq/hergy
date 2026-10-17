package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aq2bq/hergy/internal/activity"
	"github.com/aq2bq/hergy/internal/browser"
	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/effect"
	"github.com/aq2bq/hergy/internal/gycli"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
)

// servePort is the live half of what the screens read: herdr's running teams,
// their agents and pane tails, gy's open questions, the launch record, and where
// herdr's focus stands. The screens reach the top layer only through it, so
// internal/web imports none of it.
type servePort struct {
	env team.Env
	cat *catalog.Catalog
	// questions is the last read of gy's open questions, kept for a short while
	// so a redraw every five seconds does not call gy every time (設計 5.2).
	questions questionCache
}

// questionTTL is how long one read of gy's questions stands before the next
// redraw reads them again (設計 5.2: thirty seconds).
const questionTTL = 30 * time.Second

// questionCache is the last read of gy's questions, shared between the redraws.
type questionCache struct {
	mu     sync.Mutex
	at     time.Time
	loaded bool
	list   []activity.Question
	err    error
}

// Running lists the teams running on this machine with the definition each was
// booted from, the way hergy status tells them apart.
func (p *servePort) Running() ([]effect.Running, error) {
	entries, err := runningTeams()
	if err != nil {
		return nil, err
	}
	out := make([]effect.Running, 0, len(entries))
	for _, entry := range entries {
		out = append(out, effect.Running{Team: entry.Team, Definition: entry.Definition})
	}
	return out, nil
}

// Agents lists one team's live agents: the names herdr knows them by, the graph
// node each is (empty for a worker a member hired), herdr's state and its
// state_change_seq.
func (p *servePort) Agents(team string) ([]activity.Agent, error) {
	agents, err := herdrAgents()
	if err != nil {
		return nil, err
	}
	members := p.memberNames(team)
	prefix := team + "-"
	var out []activity.Agent
	for _, agent := range agents {
		if !strings.HasPrefix(agent.Name, prefix) {
			continue
		}
		short := strings.TrimPrefix(agent.Name, prefix)
		node := ""
		if members[short] {
			node = short
		}
		out = append(out, activity.Agent{
			Herdr:  agent.Name,
			Node:   node,
			Status: agent.Status,
			Seq:    agent.Seq,
		})
	}
	return out, nil
}

// memberNames is the members of the definition a running team was booted from,
// which tells a member's agent from a worker a member hired.
func (p *servePort) memberNames(team string) map[string]bool {
	entries, err := runningTeams()
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if entry.Team != team {
			continue
		}
		def, err := p.cat.Read(entry.Definition)
		if err != nil {
			return nil
		}
		names := make(map[string]bool, len(def.Graph.Members))
		for _, member := range def.Graph.Members {
			names[member.Name] = true
		}
		return names
	}
	return nil
}

// Tail is the last lines of one agent's pane, for the node that waits.
func (p *servePort) Tail(herdrName string, lines int) (string, error) {
	port := herdrcli.New(os.Getenv("HERGY_HERDR"))
	result, err := port.JSON("agent", "read", herdrName, "--source", "recent", "--lines", strconv.Itoa(lines))
	if err != nil {
		return "", err
	}
	for _, key := range []string{"text", "content", "output"} {
		if text, ok := result[key].(string); ok {
			return text, nil
		}
	}
	return "", nil
}

// Focus moves herdr's focus to one agent's pane.
func (p *servePort) Focus(herdrName string) error {
	if herdrName == "" {
		return i18n.Errorf("error.focus_no_pane")
	}
	port := herdrcli.New(os.Getenv("HERGY_HERDR"))
	return herdr.New(port).Focus(herdrName)
}

// Questions lists gy's open questions for this project, reading them from gy at
// most once every thirty seconds and keeping the last read in between, so the
// five-second redraw does not call gy every time (設計 5.2、C10). A project with
// no gy, or a machine with no gy on PATH, answers no questions and no error.
func (p *servePort) Questions() ([]activity.Question, error) {
	p.questions.mu.Lock()
	defer p.questions.mu.Unlock()
	now := time.Now()
	if p.questions.loaded && now.Sub(p.questions.at) < questionTTL {
		return p.questions.list, p.questions.err
	}
	list, err := gycli.OpenQuestions(p.env.ProjectRoot)
	p.questions.at, p.questions.loaded, p.questions.list, p.questions.err = now, true, list, err
	return list, err
}

// Launched is one running team's launch record: when it was launched, and the
// hash of each member's prompt at that time, by the member's short name (C9).
// The newest record of that team is the one it is running from: two teams of one
// name cannot run at once, so the newest is this team's.
func (p *servePort) Launched(team string) (web.Launch, bool) {
	rec, ok := p.launchOf(team)
	if !ok {
		return web.Launch{}, false
	}
	at, err := time.Parse(time.RFC3339, rec.Time)
	if err != nil {
		at = time.Time{}
	}
	prompts := make(map[string]string, len(rec.Members))
	for _, member := range rec.Members {
		if member.Short != "" {
			prompts[member.Short] = member.Prompt
		}
	}
	return web.Launch{At: at, Prompts: prompts}, true
}

// LaunchedDefinition reads the definition one team was booted from, from the
// project its launch record names: a team started from another project is judged
// like any other (W16). It reads that project's files and writes none. A team
// this machine has no record of answers an error, and the screen keeps its one
// line.
func (p *servePort) LaunchedDefinition(name string) (*definition.Definition, error) {
	rec, ok := p.launchOf(name)
	if !ok {
		return nil, i18n.Errorf("error.no_launch_record", name)
	}
	other := team.Env{
		ProjectRoot: rec.Project,
		GlobalDir:   p.env.GlobalDir,
		LocalDir:    filepath.Join(rec.Project, ".hergy"),
	}
	return other.Catalog(source.Disk{}).Read(rec.Definition)
}

// launchOf is the newest launch record of one team, or false when this machine
// holds none.
func (p *servePort) launchOf(team string) (launchlog.Record, bool) {
	log, err := launchlog.Open()
	if err != nil {
		return launchlog.Record{}, false
	}
	records, err := log.Read()
	if err != nil {
		return launchlog.Record{}, false
	}
	for i := len(records) - 1; i >= 0; i-- {
		if records[i].Team == team {
			return records[i], true
		}
	}
	return launchlog.Record{}, false
}

// breakLine writes one broken settings file as one line: where it is, as a
// person writes it, and why, with the whole reason on one line.
func breakLine(env team.Env, path string, err error) string {
	return check.BreakLine(env.Display, path, err)
}

// listingPath is the team.yaml of one layer's row of the listing.
func listingPath(row catalog.Listing, layer string) string {
	if layer == source.LayerGlobal {
		return row.GlobalPath
	}
	return row.LocalPath
}

// herdrTabList lists the tabs herdr reports, with the labels a team is found by.
func herdrTabList() ([]protocol.Tab, error) {
	port := herdrcli.New(os.Getenv("HERGY_HERDR"))
	return herdr.New(port).Tabs()
}

// runServe serves the screens on this machine and prints the URL once the socket
// is open, then opens that URL in the person's default browser. It runs in the
// foreground until Ctrl-C, then waits for the requests in flight.
//
// A browser that cannot be opened is one line on stderr and nothing else: the
// screens are already answering, so the command goes on and the line carries the
// URL to open by hand (d-fb89).
func runServe(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	cat := env.Catalog(source.Disk{})
	server, err := web.New(cat, &servePort{env: env, cat: cat})
	if err != nil {
		return err
	}
	return server.Serve(ctx, opts.Port, func(url string) {
		fmt.Fprintln(stdout, i18n.Tf("serve.url", url))
		if opts.NoOpen {
			return
		}
		if err := openBrowser(url); err != nil {
			fmt.Fprintln(stderr, i18n.Tf("serve.no_browser", url))
		}
	})
}

// openBrowser opens a URL in the person's browser. It is internal/browser's
// Open, held as a variable so a test can watch what runServe asks for without a
// browser opening on the machine the tests run on.
var openBrowser = browser.Open
