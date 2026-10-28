package launcher

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/screenlog"
	"github.com/aq2bq/hergy/internal/status"
)

// TestMain loads the messages, so the printed lines a test reads are the ones a
// human sees.
func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// fakePort is a Herdr that reports the given live agents and tabs and records
// every pane it is asked to close. It also serves the calls a refresh makes:
// agent get, agent start, agent prompt and pane process-info.
type fakePort struct {
	live     []protocol.Agent
	tabs     []protocol.Tab
	panes    []protocol.Pane
	closed   []string
	verified []string
	calls    []string

	// Refresh state. agents is the live set by herdr name; gone marks the names
	// herdr has forgotten. process maps a pane to its {foreground group, shell}.
	// onAgentGet, when set, runs before each agent get, so a test can change the
	// live state between the first check and the one before the stop. splits and
	// renamed record the panes a refresh made and named for a member the
	// definition has and hergy has no pane for.
	agents     map[string]protocol.Agent
	gone       map[string]bool
	process    map[string][2]int
	onAgentGet func(target string)
	splits     []splitCall
	renamed    []string

	// screens is what `agent read` answers for a target, by herdr name. A target
	// that is not in it answers a stable stand-in, so a status test can check the
	// screen hash without pinning a real terminal.
	screens map[string]string

	// startupBlocked names the agents whose start herdr refuses with
	// agent_not_ready, "blocked during startup": the agent is live and asks a
	// human a question, so the start records it as blocked and answers the
	// error. startedBlocked is the set of agents the fake started that way.
	// onStartupGet runs before each reading of one while it is blocked, and
	// onSettleGet before each reading after it stopped asking, so a test can
	// answer its question (set idle) or end it (delete it) the way a person
	// would answer in the pane. startupTab is the tab those agents report.
	startupBlocked map[string]bool
	startedBlocked map[string]bool
	onStartupGet   func(name string)
	onSettleGet    func(name string)
	startupTab     string
}

func (p *fakePort) Verify(mode string) error {
	p.verified = append(p.verified, mode)
	return nil
}

func (p *fakePort) record(args ...string) {
	p.calls = append(p.calls, strings.Join(args, " "))
}

// splitCall is one pane split the fake made: the pane it split, the direction,
// and the pane it made.
type splitCall struct {
	from      string
	direction string
	pane      string
}

// tabOf is the tab one pane of the fake stands in, or "" when it is not a pane
// the fake was given.
func (p *fakePort) tabOf(pane string) string {
	for _, known := range p.panes {
		if known.ID == pane {
			return known.TabID
		}
	}
	return ""
}

// hasCall reports whether the fake was asked one command that starts with prefix.
func hasCall(p *fakePort, prefix string) bool {
	for _, call := range p.calls {
		if strings.HasPrefix(call, prefix) {
			return true
		}
	}
	return false
}

// findAgent looks a live agent up by herdr name or by pane id.
func (p *fakePort) findAgent(target string) (protocol.Agent, bool) {
	if agent, ok := p.agents[target]; ok {
		return agent, true
	}
	for _, agent := range p.agents {
		if agent.PaneID == target {
			return agent, true
		}
	}
	return protocol.Agent{}, false
}

// notFound is the error herdr answers agent get with for a name it does not know.
func notFound(args []string) error {
	_, err := protocol.Result(args, `{"error":{"code":"agent_not_found"}}`, "", 0)
	return err
}

func agentObject(agent protocol.Agent) map[string]any {
	return map[string]any{
		"name": agent.Name, "agent": agent.Kind,
		"agent_status": agent.Status, "pane_id": agent.PaneID,
		"tab_id": agent.TabID, "terminal_title_stripped": agent.Title,
	}
}

func argValue(args []string, flag string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}

func (p *fakePort) JSON(args ...string) (map[string]any, error) {
	p.record(args...)
	switch {
	case slices.Equal(args, []string{"agent", "list"}):
		agents := make([]any, 0, len(p.live))
		for _, agent := range p.live {
			agents = append(agents, agentObject(agent))
		}
		return map[string]any{"agents": agents}, nil
	case slices.Equal(args, []string{"tab", "list"}):
		tabs := make([]any, 0, len(p.tabs))
		for _, tab := range p.tabs {
			tabs = append(tabs, map[string]any{"tab_id": tab.ID, "label": tab.Label})
		}
		return map[string]any{"tabs": tabs}, nil
	case slices.Equal(args, []string{"pane", "list"}):
		panes := make([]any, 0, len(p.panes))
		for _, pane := range p.panes {
			panes = append(panes, map[string]any{"pane_id": pane.ID, "label": pane.Label, "tab_id": pane.TabID})
		}
		return map[string]any{"panes": panes}, nil
	case len(args) == 3 && args[0] == "pane" && args[1] == "close":
		p.closed = append(p.closed, args[2])
		return map[string]any{}, nil
	case len(args) == 4 && args[0] == "pane" && args[1] == "rename":
		p.renamed = append(p.renamed, args[2]+" "+args[3])
		return map[string]any{}, nil
	case len(args) >= 3 && args[0] == "pane" && args[1] == "split":
		// A split makes a pane of its own, in the tab the pane it splits is in:
		// a test can read where a refresh put a member it added.
		pane := fmt.Sprintf("n%d", len(p.splits)+1)
		p.splits = append(p.splits, splitCall{from: args[2], direction: argValue(args, "--direction"), pane: pane})
		p.panes = append(p.panes, protocol.Pane{ID: pane, TabID: p.tabOf(args[2])})
		return map[string]any{"pane": map[string]any{"pane_id": pane}}, nil
	case len(args) == 3 && args[0] == "agent" && args[1] == "get":
		target := args[2]
		if agent, ok := p.findAgent(target); ok && p.startedBlocked[agent.Name] {
			switch {
			case agent.Status == "blocked" && p.onStartupGet != nil:
				p.onStartupGet(agent.Name)
			case agent.Status != "blocked" && p.onSettleGet != nil:
				p.onSettleGet(agent.Name)
			}
		}
		if p.onAgentGet != nil {
			p.onAgentGet(target)
		}
		agent, ok := p.findAgent(target)
		if !ok || p.gone[agent.Name] {
			return nil, notFound(args)
		}
		return map[string]any{"agent": agentObject(agent)}, nil
	case len(args) == 4 && args[0] == "pane" && args[1] == "process-info" && args[2] == "--pane":
		pane := args[3]
		group, shell := 4242, 900
		if pair, ok := p.process[pane]; ok {
			group, shell = pair[0], pair[1]
		}
		if group == shell {
			// Only the shell is in the pane, so there is no agent for herdr to
			// know: it drops the name, as the real herdr would.
			for name, agent := range p.agents {
				if agent.PaneID == pane {
					p.gone[name] = true
				}
			}
		}
		return map[string]any{"process_info": map[string]any{
			"foreground_process_group_id": group,
			"shell_pid":                   shell,
			"pane_id":                     pane,
		}}, nil
	case len(args) >= 3 && args[0] == "agent" && args[1] == "start":
		name := args[2]
		if _, taken := p.agents[name]; taken && !p.gone[name] {
			_, err := protocol.Result(args, `{"error":{"code":"agent_name_taken"}}`, "", 0)
			return nil, err
		}
		if p.startupBlocked[name] {
			// The agent is live, and it asks a human a question while it starts:
			// herdr refuses the start as not ready, and keeps the agent.
			p.agents[name] = protocol.Agent{
				Name: name, Kind: argValue(args, "--kind"), Status: "blocked",
				PaneID: argValue(args, "--pane"), TabID: p.startupTab,
			}
			if p.startedBlocked == nil {
				p.startedBlocked = map[string]bool{}
			}
			p.startedBlocked[name] = true
			delete(p.gone, name)
			_, err := protocol.Result(args, `{"error":{"code":"agent_not_ready","message":"agent `+name+
				` is blocked during startup and is not ready for prompts"}}`, "", 0)
			return nil, err
		}
		p.agents[name] = protocol.Agent{
			Name: name, Kind: argValue(args, "--kind"),
			Status: "idle", PaneID: argValue(args, "--pane"),
		}
		delete(p.gone, name)
		return map[string]any{}, nil
	case len(args) >= 3 && args[0] == "agent" && args[1] == "prompt":
		return map[string]any{}, nil
	}
	return nil, fmt.Errorf("unexpected herdr command: %v", args)
}

// Text answers `agent read`, the plain-text command status hashes.
func (p *fakePort) Text(args ...string) (string, error) {
	p.record(args...)
	if len(args) >= 3 && args[0] == "agent" && args[1] == "read" {
		target := args[2]
		if screen, ok := p.screens[target]; ok {
			return screen, nil
		}
		return "screen of " + target, nil
	}
	return "", fmt.Errorf("unexpected herdr text command: %v", args)
}

// commandTrail is the recorded calls as "<group> <sub>" pairs, so a test can
// check the order of the calls a refresh makes.
func commandTrail(calls []string) []string {
	var out []string
	for _, call := range calls {
		fields := strings.Fields(call)
		if len(fields) >= 2 {
			out = append(out, fields[0]+" "+fields[1])
		}
	}
	return out
}

// downLauncher builds a team of the named members and a launcher over the fake
// Herdr. The returned buffer collects what Down prints.
func downLauncher(t *testing.T, port *fakePort, members ...string) (*Launcher, *bytes.Buffer) {
	t.Helper()
	return downLauncherFor(t, port, "team", members...)
}

// downLauncherFor is downLauncher with the team named, so a test can set the
// name the worker tabs are labelled with.
func downLauncherFor(t *testing.T, port *fakePort, team string, members ...string) (*Launcher, *bytes.Buffer) {
	t.Helper()
	list := make([]definition.Start, len(members))
	for i, name := range members {
		list[i] = definition.Start{Name: name, Short: name, Role: "role", Kind: "claude"}
	}
	out := &bytes.Buffer{}
	return New(list, team, herdr.New(port), 30000, out), out
}

func TestDownClosesItsOwnPaneLast(t *testing.T) {
	port := &fakePort{live: []protocol.Agent{
		{Name: "a", Kind: "claude", Status: "idle", PaneID: "p1"},
		{Name: "b", Kind: "claude", Status: "idle", PaneID: "own"},
		{Name: "c", Kind: "claude", Status: "idle", PaneID: "p3"},
	}}
	l, _ := downLauncher(t, port, "a", "b", "c")
	members := l.starts

	if _, err := l.Down(members, "own", false); err != nil {
		t.Fatalf("Down: %v", err)
	}
	want := []string{"p1", "p3", "own"}
	if !slices.Equal(port.closed, want) {
		t.Fatalf("closed %v, want %v", port.closed, want)
	}
	if !slices.Equal(port.verified, []string{protocol.ModeDown}) {
		t.Fatalf("Verify called with %v, want [%s]", port.verified, protocol.ModeDown)
	}
}

func TestDownSkipsAMemberThatIsNotLive(t *testing.T) {
	port := &fakePort{live: []protocol.Agent{
		{Name: "a", Kind: "claude", Status: "idle", PaneID: "p1"},
	}}
	l, out := downLauncher(t, port, "a", "b")
	members := l.starts

	if _, err := l.Down(members, "", false); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if !slices.Equal(port.closed, []string{"p1"}) {
		t.Fatalf("closed %v, want only [p1]", port.closed)
	}
	if !strings.Contains(out.String(), "b: not present") {
		t.Fatalf("output does not report b as absent:\n%s", out.String())
	}
}

func TestDownDryClosesNothing(t *testing.T) {
	port := &fakePort{live: []protocol.Agent{
		{Name: "a", Kind: "claude", Status: "idle", PaneID: "p1"},
		{Name: "b", Kind: "claude", Status: "idle", PaneID: "p2"},
	}}
	l, out := downLauncher(t, port, "a", "b")
	members := l.starts

	if _, err := l.Down(members, "p1", true); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if len(port.closed) != 0 {
		t.Fatalf("dry run closed %v, want nothing", port.closed)
	}
	if len(port.verified) != 0 {
		t.Fatalf("dry run verified %v, want no Herdr check", port.verified)
	}
	if !strings.Contains(out.String(), "would close") {
		t.Fatalf("output shows no plan:\n%s", out.String())
	}
}

func TestDownWithNobodyLiveClosesNothing(t *testing.T) {
	port := &fakePort{}
	l, out := downLauncher(t, port, "a", "b")
	members := l.starts

	if _, err := l.Down(members, "", false); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if len(port.closed) != 0 {
		t.Fatalf("closed %v, want nothing", port.closed)
	}
	if !strings.Contains(out.String(), "nothing to close") {
		t.Fatalf("output does not say nothing was closed:\n%s", out.String())
	}
}

// TestDownClosesTheTeamsHiredWorkers: down closes the team's members and the
// workers hired into tabs labelled "worker:<team>/...", and leaves another
// team's worker alone.
func TestDownClosesTheTeamsHiredWorkers(t *testing.T) {
	port := &fakePort{
		live: []protocol.Agent{
			{Name: "dev-lead", Kind: "claude", Status: "idle", PaneID: "p-lead", TabID: "t-team"},
			{Name: "dev-worker", Kind: "claude", Status: "idle", PaneID: "p-worker", TabID: "t-team"},
			{Name: "dev-w1", Kind: "claude", Status: "idle", PaneID: "p-w1", TabID: "t-w1"},
			{Name: "dev-w2", Kind: "claude", Status: "idle", PaneID: "p-w2", TabID: "t-w2"},
			{Name: "dev-x-w1", Kind: "claude", Status: "idle", PaneID: "p-x1", TabID: "t-x1"},
		},
		tabs: []protocol.Tab{
			{ID: "t-team", Label: "team:dev"},
			{ID: "t-w1", Label: "worker:dev/w1"},
			{ID: "t-w2", Label: "worker:dev/w2"},
			{ID: "t-x1", Label: "worker:dev-x/w1"},
		},
	}
	l, out := downLauncherFor(t, port, "dev", "dev-lead", "dev-worker")
	members := l.starts

	if _, err := l.Down(members, "", false); err != nil {
		t.Fatalf("Down: %v", err)
	}
	want := []string{"p-lead", "p-worker", "p-w1", "p-w2"}
	if !slices.Equal(port.closed, want) {
		t.Fatalf("closed %v, want %v", port.closed, want)
	}
	if strings.Contains(out.String(), "dev-x") {
		t.Fatalf("down dev reports dev-x's worker:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "dev-w1: closing (pane p-w1, worker)\n") {
		t.Fatalf("down does not mark dev-w1 as a worker:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "closed: 4\n") {
		t.Fatalf("down does not count the workers:\n%s", out.String())
	}
}

// TestDownAsClosesOnlyThatTeamsWorkers: with --as, the team name in the label
// decides; only "worker:<as>/..." tabs close, not the settings name's workers.
func TestDownAsClosesOnlyThatTeamsWorkers(t *testing.T) {
	port := &fakePort{
		live: []protocol.Agent{
			{Name: "dev-w1", Kind: "claude", Status: "idle", PaneID: "p-dev", TabID: "t-dev"},
			{Name: "dev2-w1", Kind: "claude", Status: "idle", PaneID: "p-dev2", TabID: "t-dev2"},
		},
		tabs: []protocol.Tab{
			{ID: "t-dev", Label: "worker:dev/w1"},
			{ID: "t-dev2", Label: "worker:dev2/w1"},
		},
	}
	l, out := downLauncherFor(t, port, "dev2", "dev2-lead", "dev2-worker")
	members := l.starts

	if _, err := l.Down(members, "", false); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if !slices.Equal(port.closed, []string{"p-dev2"}) {
		t.Fatalf("closed %v, want only dev2's worker", port.closed)
	}
	if !strings.Contains(out.String(), "closed: 1\n") {
		t.Fatalf("down does not count dev2's worker:\n%s", out.String())
	}
}

// TestDownDryPlansTheWorkers: --dry prints the members and the workers and
// closes nothing.
func TestDownDryPlansTheWorkers(t *testing.T) {
	port := &fakePort{
		live: []protocol.Agent{
			{Name: "dev-lead", Kind: "claude", Status: "idle", PaneID: "p-lead", TabID: "t-team"},
			{Name: "dev-w1", Kind: "claude", Status: "idle", PaneID: "p-w1", TabID: "t-w1"},
		},
		tabs: []protocol.Tab{
			{ID: "t-team", Label: "team:dev"},
			{ID: "t-w1", Label: "worker:dev/w1"},
		},
	}
	l, out := downLauncherFor(t, port, "dev", "dev-lead", "dev-worker")
	members := l.starts

	if _, err := l.Down(members, "", true); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if len(port.closed) != 0 {
		t.Fatalf("dry run closed %v, want nothing", port.closed)
	}
	if len(port.verified) != 0 {
		t.Fatalf("dry run verified %v, want no Herdr check", port.verified)
	}
	for _, want := range []string{
		"dev-lead: would close (pane p-lead)\n",
		"dev-w1: would close (pane p-w1, worker)\n",
		"would close: 2\n",
		"would skip: 1\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("dry plan is missing %q:\n%s", want, out.String())
		}
	}
}

// TestDownClosesAWorkersOwnPaneLast: a worker can be running in hergy's own
// pane; it is still closed, and closed last.
func TestDownClosesAWorkersOwnPaneLast(t *testing.T) {
	port := &fakePort{
		live: []protocol.Agent{
			{Name: "dev-lead", Kind: "claude", Status: "idle", PaneID: "p-lead", TabID: "t-team"},
			{Name: "dev-w1", Kind: "claude", Status: "idle", PaneID: "own", TabID: "t-w1"},
		},
		tabs: []protocol.Tab{
			{ID: "t-team", Label: "team:dev"},
			{ID: "t-w1", Label: "worker:dev/w1"},
		},
	}
	l, out := downLauncherFor(t, port, "dev", "dev-lead")
	members := l.starts

	if _, err := l.Down(members, "own", false); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if want := []string{"p-lead", "own"}; !slices.Equal(port.closed, want) {
		t.Fatalf("closed %v, want %v (own pane last)", port.closed, want)
	}
	if !strings.Contains(out.String(), "dev-w1: closing (pane own, worker, own pane; last)\n") {
		t.Fatalf("down does not report the worker's own pane last:\n%s", out.String())
	}
}

// TestStatusRowsMembersThenWorkers: status reports one row per member (live or
// not) and then the team's hired workers, marking the workers.
func TestStatusRowsMembersThenWorkers(t *testing.T) {
	port := &fakePort{
		live: []protocol.Agent{
			{Name: "dev-lead", Kind: "claude", Status: "working", PaneID: "p-lead", TabID: "t-team", Title: "#42 の報告を待っている"},
			{Name: "dev-w1", Kind: "claude", Status: "blocked", PaneID: "p-w1", TabID: "t-w1", Title: "依存の更新を入れてよいか聞いている"},
		},
		tabs: []protocol.Tab{
			{ID: "t-team", Label: "team:dev"},
			{ID: "t-w1", Label: "worker:dev/w1"},
		},
	}
	l, _ := downLauncherFor(t, port, "dev", "dev-lead", "dev-worker")
	members := l.starts

	rows, err := l.Status(members)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	want := []status.Row{
		{Name: "dev-lead", State: "working", Live: true, PaneID: "p-lead", Title: "#42 の報告を待っている", Role: "role", Kind: "claude", Screen: screenlog.Hash("screen of dev-lead")},
		{Name: "dev-worker", Live: false, Role: "role", Kind: "claude"},
		{Name: "dev-w1", State: "blocked", Live: true, Worker: true, PaneID: "p-w1", Title: "依存の更新を入れてよいか聞いている", Kind: "claude"},
	}
	if !slices.Equal(rows, want) {
		t.Fatalf("Status = %+v, want %+v", rows, want)
	}
	if !slices.Equal(port.verified, []string{protocol.ModeStatus}) {
		t.Fatalf("Verify called with %v, want [%s]", port.verified, protocol.ModeStatus)
	}
	if slices.Contains(commandTrail(port.calls), "pane close") {
		t.Fatalf("status closed a pane:\n%v", port.calls)
	}
}

// TestRefreshStartsAMemberThatIsNotLive: when a member has no live agent but the
// pane hergy named after it is still at its shell, refresh starts the member
// there again and submits the prompt. It stops nothing.
func TestRefreshStartsAMemberThatIsNotLive(t *testing.T) {
	port := &fakePort{
		agents:  map[string]protocol.Agent{},
		gone:    map[string]bool{},
		panes:   []protocol.Pane{{ID: "p1", Label: "team-lead"}},
		process: map[string][2]int{"p1": {900, 900}},
	}
	l, out := downLauncher(t, port, "team-lead")
	l.timeout = 3000
	l.kill = func(pgid int) error {
		t.Fatalf("refresh killed %d for a member that is not live", pgid)
		return nil
	}
	members := l.starts

	if err := l.Refresh(members, ""); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	want := []string{"agent get", "pane list", "pane process-info", "agent start", "agent prompt"}
	if got := commandTrail(port.calls); !slices.Equal(got, want) {
		t.Fatalf("call order %v, want %v", got, want)
	}
	if !slices.Equal(port.verified, []string{protocol.ModeRefresh}) {
		t.Fatalf("Verify called with %v, want [%s]", port.verified, protocol.ModeRefresh)
	}
	if !strings.Contains(out.String(), "restarted: team-lead") {
		t.Fatalf("refresh does not report the restart:\n%s", out.String())
	}
}

// addablePort is a running demo team whose tab holds demo-lead at p1, and a
// definition that also has demo-spec2, which hergy has no pane for.
func addablePort() *fakePort {
	return &fakePort{
		agents: map[string]protocol.Agent{
			"demo-lead": {Name: "demo-lead", Kind: "claude", Status: "idle", PaneID: "p1", TabID: "t1"},
		},
		gone:    map[string]bool{},
		process: map[string][2]int{"p1": {4242, 900}},
		panes: []protocol.Pane{
			{ID: "px", Label: "other-lead", TabID: "t9"},
			{ID: "p1", Label: "demo-lead", TabID: "t1"},
		},
		tabs: []protocol.Tab{
			{ID: "t9", Label: "other"},
			{ID: "t1", Label: "demo"},
		},
	}
}

// TestRefreshAddsAMemberTheDefinitionHasAndNoPaneFor: a member written in the
// definition that hergy made no pane for is given one in the team's tab, and
// started there with the bootstrap prompt and the timeout a boot uses. The
// members already running are left alone (d-8fcf, r-7e09).
func TestRefreshAddsAMemberTheDefinitionHasAndNoPaneFor(t *testing.T) {
	port := addablePort()
	l, out := downLauncherFor(t, port, "demo", "demo-lead", "demo-spec2")
	l.timeout = 3000
	l.kill = func(pgid int) error { t.Fatalf("refresh killed %d for a member it only adds", pgid); return nil }
	l.starts[1].Prompt = "you are spec2; read the rules and wait"
	spec2 := l.starts[1]

	if err := l.Refresh([]definition.Start{spec2}, ""); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	want := []splitCall{{from: "p1", direction: "right", pane: "n1"}}
	if !slices.Equal(port.splits, want) {
		t.Fatalf("splits %+v, want %+v (a pane in the team's tab, from the team's pane)", port.splits, want)
	}
	if !slices.Equal(port.renamed, []string{"n1 demo-spec2"}) {
		t.Fatalf("renamed %v, want the new pane named after the member", port.renamed)
	}
	if !hasCall(port, "agent start demo-spec2 --kind claude --pane n1 --timeout 3000") {
		t.Fatalf("refresh did not start the member in its new pane with the timeout:\n%v", port.calls)
	}
	if !hasCall(port, "agent prompt demo-spec2 "+spec2.Prompt) {
		t.Fatalf("refresh did not submit the bootstrap prompt to the new pane:\n%v", port.calls)
	}
	for _, call := range port.calls {
		if strings.Contains(call, "demo-lead") {
			t.Fatalf("refresh touched a member it was not asked about: %q\n%v", call, port.calls)
		}
	}
	if !strings.Contains(out.String(), "started: demo-spec2 (tab demo (t1))") {
		t.Fatalf("refresh does not report the member it started:\n%s", out.String())
	}
	if !strings.HasSuffix(out.String(), "next: hergy reprompt --all --team demo\n") {
		t.Fatalf("the last line does not name hergy reprompt --all:\n%s", out.String())
	}
}

// TestRefreshAllRestartsTheRunningAndAddsTheRest: with --all the live member is
// started again in its own pane as before, and the member with no pane is added;
// the one member's restart does not make a pane.
func TestRefreshAllRestartsTheRunningAndAddsTheRest(t *testing.T) {
	port := addablePort()
	l, out := downLauncherFor(t, port, "demo", "demo-lead", "demo-spec2")
	l.timeout = 3000
	var killed []int
	l.kill = func(pgid int) error {
		killed = append(killed, pgid)
		port.gone["demo-lead"] = true
		return nil
	}
	l.starts[1].Prompt = "you are spec2; read the rules and wait"
	spec2 := l.starts[1]

	if err := l.Refresh(l.starts, ""); err != nil {
		t.Fatalf("Refresh --all: %v", err)
	}
	if !slices.Equal(killed, []int{4242}) {
		t.Fatalf("killed %v, want only the live member's process group", killed)
	}
	if want := []splitCall{{from: "p1", direction: "right", pane: "n1"}}; !slices.Equal(port.splits, want) {
		t.Fatalf("splits %+v, want one pane for the member with none: %+v", port.splits, want)
	}
	if !hasCall(port, "agent start demo-lead --kind claude --pane p1 --timeout 3000") {
		t.Fatalf("refresh did not start the live member again in its own pane:\n%v", port.calls)
	}
	if !hasCall(port, "agent start demo-spec2 --kind claude --pane n1 --timeout 3000") {
		t.Fatalf("refresh did not start the added member in its new pane:\n%v", port.calls)
	}
	if !hasCall(port, "agent prompt demo-spec2 "+spec2.Prompt) {
		t.Fatalf("refresh did not submit the bootstrap prompt to the added member:\n%v", port.calls)
	}
	if !strings.Contains(out.String(), "restarted: demo-lead") {
		t.Fatalf("refresh does not report the restart:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "started: demo-spec2 (tab demo (t1))") {
		t.Fatalf("refresh does not report the added member:\n%s", out.String())
	}
}

// TestRefreshRefusesToAddWhenTheTeamHasNoTab: a team that is not running has no
// tab to add a member to, so refresh refuses and names hergy up, and nothing is
// split or started.
func TestRefreshRefusesToAddWhenTheTeamHasNoTab(t *testing.T) {
	port := addablePort()
	port.tabs = nil
	l, _ := downLauncherFor(t, port, "demo", "demo-lead", "demo-spec2")
	l.kill = func(pgid int) error { t.Fatalf("refresh killed %d", pgid); return nil }
	spec2 := l.starts[1]

	err := l.Refresh([]definition.Start{spec2}, "")
	if err == nil {
		t.Fatal("Refresh accepted a member with no pane in a team that is not running")
	}
	if !strings.Contains(err.Error(), "hergy up") {
		t.Fatalf("the refusal does not name hergy up: %v", err)
	}
	if len(port.splits) != 0 || slices.Contains(commandTrail(port.calls), "agent start") {
		t.Fatalf("refresh made or started something in a team that is not running:\n%v", port.calls)
	}
}

// TestRefreshDryShowsThePaneItWouldMake: --dry names the member it would give a
// pane to and the tab, and makes and starts nothing.
func TestRefreshDryShowsThePaneItWouldMake(t *testing.T) {
	port := addablePort()
	l, _ := downLauncherFor(t, port, "demo", "demo-lead", "demo-spec2")
	l.kill = func(pgid int) error { t.Fatalf("dry refresh killed %d", pgid); return nil }
	spec2 := l.starts[1]

	plan, err := l.RefreshDry([]definition.Start{spec2}, "")
	if err != nil {
		t.Fatalf("RefreshDry: %v", err)
	}
	want := NewPanes{Members: []string{"demo-spec2"}, Tab: "demo (t1)"}
	if !slices.Equal(plan.Members, want.Members) || plan.Tab != want.Tab {
		t.Fatalf("RefreshDry = %+v, want %+v", plan, want)
	}
	if len(port.splits) != 0 || len(port.renamed) != 0 {
		t.Fatalf("dry refresh made a pane: %v %v", port.splits, port.renamed)
	}
	if slices.Contains(commandTrail(port.calls), "agent start") {
		t.Fatalf("dry refresh started the member:\n%v", port.calls)
	}
}

// TestRefreshWaitsOnAnAddedMembersStartupQuestion: a member hergy adds asks a
// question while it starts, the way a boot's member does: hergy names the tab, the
// pane and the kind, waits, then submits the bootstrap prompt. The wait names the
// team's tab, which refresh made no tab of its own for.
func TestRefreshWaitsOnAnAddedMembersStartupQuestion(t *testing.T) {
	inLanguage(t, "en")
	port := addablePort()
	port.startupBlocked = map[string]bool{"demo-spec2": true}
	port.startupTab = "t1"
	l, out := downLauncherFor(t, port, "demo", "demo-lead", "demo-spec2")
	l.timeout = 3000
	l.kill = func(pgid int) error { t.Fatalf("refresh killed %d", pgid); return nil }
	readings := 0
	port.onStartupGet = func(name string) {
		readings++
		if readings > 1 {
			answered(port, name)
		}
	}
	l.starts[1].Prompt = "you are spec2; read the rules and wait"
	spec2 := l.starts[1]

	if err := l.Refresh([]definition.Start{spec2}, ""); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	for _, want := range []string{
		"waiting for permission: demo-spec2 is asking a question while it starts (tab demo (t1), pane n1)",
		"demo-spec2 answered and started; going on",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("the wait does not show %q:\n%s", want, out.String())
		}
	}
	if !hasCall(port, "agent prompt demo-spec2 "+spec2.Prompt) {
		t.Fatalf("hergy did not submit the bootstrap prompt after the answer:\n%v", port.calls)
	}
}

// TestRefreshRefusesAMemberWhosePaneIsBusy: a member that is not live, whose
// named pane runs something else, is refused, and nothing is started.
func TestRefreshRefusesAMemberWhosePaneIsBusy(t *testing.T) {
	port := &fakePort{
		agents:  map[string]protocol.Agent{},
		gone:    map[string]bool{},
		panes:   []protocol.Pane{{ID: "p1", Label: "team-lead"}},
		process: map[string][2]int{"p1": {4242, 900}},
	}
	l, _ := downLauncher(t, port, "team-lead")
	l.kill = func(pgid int) error { t.Fatalf("refresh killed %d", pgid); return nil }
	members := l.starts

	if err := l.Refresh(members, ""); err == nil {
		t.Fatal("Refresh accepted a member whose pane is busy")
	}
	if slices.Contains(commandTrail(port.calls), "agent start") {
		t.Fatalf("refresh started an agent in a busy pane:\n%v", port.calls)
	}
}

// TestRefreshRefusesItsOwnPaneForADeadMember: the self refusal holds when the
// member is not live and its named pane is hergy's own.
func TestRefreshRefusesItsOwnPaneForADeadMember(t *testing.T) {
	port := &fakePort{
		agents: map[string]protocol.Agent{},
		gone:   map[string]bool{},
		panes:  []protocol.Pane{{ID: "own", Label: "team-lead"}},
	}
	l, _ := downLauncher(t, port, "team-lead")
	members := l.starts

	if err := l.Refresh(members, "own"); err == nil {
		t.Fatal("Refresh accepted its own pane for a member that is not live")
	}
	if slices.Contains(commandTrail(port.calls), "pane process-info") {
		t.Fatalf("refresh read process info for its own pane:\n%v", port.calls)
	}
}

// TestRefreshAllChecksEveryoneBeforeStarting: --all checks every member before
// it starts any, so a working member stops the whole command before the dead one
// is started (or its pane is read).
func TestRefreshAllChecksEveryoneBeforeStarting(t *testing.T) {
	port := &fakePort{
		agents: map[string]protocol.Agent{
			"team-lead": {Name: "team-lead", Kind: "claude", Status: "working", PaneID: "p1"},
		},
		gone:    map[string]bool{},
		panes:   []protocol.Pane{{ID: "p2", Label: "team-worker"}},
		process: map[string][2]int{"p2": {900, 900}},
	}
	l, _ := downLauncher(t, port, "team-lead", "team-worker")
	members := l.starts

	if err := l.Refresh(members, ""); err == nil {
		t.Fatal("Refresh --all accepted a working member")
	}
	trail := commandTrail(port.calls)
	if slices.Contains(trail, "agent start") || slices.Contains(trail, "pane list") {
		t.Fatalf("refresh --all acted before checking every member:\n%v", port.calls)
	}
}

// TestRefreshDryAcceptsADeadMember: --dry checks the same dead member and stops
// and starts nothing.
func TestRefreshDryAcceptsADeadMember(t *testing.T) {
	port := &fakePort{
		agents:  map[string]protocol.Agent{},
		gone:    map[string]bool{},
		panes:   []protocol.Pane{{ID: "p1", Label: "team-lead"}},
		process: map[string][2]int{"p1": {900, 900}},
	}
	l, _ := downLauncher(t, port, "team-lead")
	l.kill = func(pgid int) error { t.Fatalf("dry refresh killed %d", pgid); return nil }
	members := l.starts

	if _, err := l.RefreshDry(members, ""); err != nil {
		t.Fatalf("RefreshDry: %v", err)
	}
	if len(port.verified) != 0 {
		t.Fatalf("RefreshDry checked the herdr commands: %v", port.verified)
	}
	if slices.Contains(commandTrail(port.calls), "agent start") {
		t.Fatalf("dry refresh started the member:\n%v", port.calls)
	}
}

// TestRefreshRestartsInPlaceInOrder: refresh stops the member's process group,
// waits for herdr to forget the name, then starts the member again in the same
// pane and submits the prompt, in that order.
func TestRefreshRestartsInPlaceInOrder(t *testing.T) {
	port := &fakePort{
		agents: map[string]protocol.Agent{
			"team-lead": {Name: "team-lead", Kind: "claude", Status: "idle", PaneID: "p1"},
		},
		gone:    map[string]bool{},
		process: map[string][2]int{"p1": {4242, 900}},
	}
	l, out := downLauncher(t, port, "team-lead")
	l.timeout = 3000
	var killed []int
	l.kill = func(pgid int) error {
		killed = append(killed, pgid)
		port.gone["team-lead"] = true
		return nil
	}
	members := l.starts

	if err := l.Refresh(members, ""); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if !slices.Equal(killed, []int{4242}) {
		t.Fatalf("killed %v, want [4242]", killed)
	}
	want := []string{"agent get", "agent get", "pane process-info", "agent get", "agent start", "agent prompt"}
	if got := commandTrail(port.calls); !slices.Equal(got, want) {
		t.Fatalf("call order %v, want %v", got, want)
	}
	if !slices.Equal(port.verified, []string{protocol.ModeRefresh}) {
		t.Fatalf("Verify called with %v, want [%s]", port.verified, protocol.ModeRefresh)
	}
	if !strings.Contains(out.String(), "restarted: team-lead") {
		t.Fatalf("refresh does not report the restart:\n%s", out.String())
	}
}

// TestRefreshStopsNothingWhenAMemberIsWorking: a member that is not idle or done
// stops the whole command before anything is stopped.
func TestRefreshStopsNothingWhenAMemberIsWorking(t *testing.T) {
	port := &fakePort{
		agents: map[string]protocol.Agent{
			"team-lead": {Name: "team-lead", Kind: "claude", Status: "working", PaneID: "p1"},
		},
		gone: map[string]bool{},
	}
	l, _ := downLauncher(t, port, "team-lead")
	var killed []int
	l.kill = func(pgid int) error { killed = append(killed, pgid); return nil }
	members := l.starts

	if err := l.Refresh(members, ""); err == nil {
		t.Fatal("Refresh accepted a working member")
	}
	if len(killed) != 0 {
		t.Fatalf("refresh stopped a working member's process group: %v", killed)
	}
	if slices.Contains(commandTrail(port.calls), "pane process-info") {
		t.Fatalf("refresh read process info for a working member:\n%v", port.calls)
	}
}

// TestRefreshAllStopsNothingWhenAnyIsWorking: --all checks every member before
// it stops any, so one working member leaves the whole team running.
func TestRefreshAllStopsNothingWhenAnyIsWorking(t *testing.T) {
	port := &fakePort{
		agents: map[string]protocol.Agent{
			"team-lead":   {Name: "team-lead", Kind: "claude", Status: "idle", PaneID: "p1"},
			"team-worker": {Name: "team-worker", Kind: "claude", Status: "working", PaneID: "p2"},
		},
		gone: map[string]bool{},
	}
	l, _ := downLauncher(t, port, "team-lead", "team-worker")
	var killed []int
	l.kill = func(pgid int) error { killed = append(killed, pgid); return nil }
	members := l.starts

	if err := l.Refresh(members, ""); err == nil {
		t.Fatal("Refresh --all accepted a working member")
	}
	if len(killed) != 0 {
		t.Fatalf("refresh --all stopped a member before checking them all: %v", killed)
	}
}

// TestRefreshTimesOutWhenTheAgentNeverGoes: when herdr keeps the name past
// --timeout, refresh errors and does not start a second agent.
func TestRefreshTimesOutWhenTheAgentNeverGoes(t *testing.T) {
	port := &fakePort{
		agents: map[string]protocol.Agent{
			"team-lead": {Name: "team-lead", Kind: "claude", Status: "idle", PaneID: "p1"},
		},
		gone:    map[string]bool{},
		process: map[string][2]int{"p1": {4242, 900}},
	}
	l, _ := downLauncher(t, port, "team-lead")
	l.timeout = 50
	var killed []int
	// The process group is stopped, but herdr never forgets the name.
	l.kill = func(pgid int) error { killed = append(killed, pgid); return nil }
	members := l.starts

	if err := l.Refresh(members, ""); err == nil {
		t.Fatal("Refresh did not time out")
	}
	if !slices.Equal(killed, []int{4242}) {
		t.Fatalf("killed %v, want [4242]", killed)
	}
	if slices.Contains(commandTrail(port.calls), "agent start") {
		t.Fatalf("refresh started a member herdr still knows:\n%v", port.calls)
	}
}

// TestRefreshDoesNotStopTheShell: when the foreground process group is the
// pane's own shell, there is no agent to stop.
func TestRefreshDoesNotStopTheShell(t *testing.T) {
	port := &fakePort{
		agents: map[string]protocol.Agent{
			"team-lead": {Name: "team-lead", Kind: "claude", Status: "idle", PaneID: "p1"},
		},
		gone:    map[string]bool{},
		process: map[string][2]int{"p1": {900, 900}},
	}
	l, _ := downLauncher(t, port, "team-lead")
	var killed []int
	l.kill = func(pgid int) error { killed = append(killed, pgid); return nil }
	members := l.starts

	if err := l.Refresh(members, ""); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if len(killed) != 0 {
		t.Fatalf("refresh stopped the shell's process group: %v", killed)
	}
	if !slices.Contains(commandTrail(port.calls), "agent start") {
		t.Fatalf("refresh did not start the member again:\n%v", port.calls)
	}
}

// TestRepromptOnlyPrompts: reprompt sends the prompt and touches no process:
// no process-info and no kill.
func TestRepromptOnlyPrompts(t *testing.T) {
	port := &fakePort{
		agents: map[string]protocol.Agent{
			"team-lead": {Name: "team-lead", Kind: "claude", Status: "idle", PaneID: "p1"},
		},
		gone: map[string]bool{},
	}
	l, out := downLauncher(t, port, "team-lead")
	l.kill = func(pgid int) error { t.Fatalf("reprompt called kill(%d)", pgid); return nil }
	members := l.starts

	if err := l.Reprompt(members); err != nil {
		t.Fatalf("Reprompt: %v", err)
	}
	want := []string{"agent get", "agent get", "agent prompt"}
	if got := commandTrail(port.calls); !slices.Equal(got, want) {
		t.Fatalf("call order %v, want %v", got, want)
	}
	if slices.Contains(commandTrail(port.calls), "pane process-info") {
		t.Fatalf("reprompt read process info:\n%v", port.calls)
	}
	if !slices.Equal(port.verified, []string{protocol.ModeReprompt}) {
		t.Fatalf("Verify called with %v, want [%s]", port.verified, protocol.ModeReprompt)
	}
	if !strings.Contains(out.String(), "reprompted: team-lead") {
		t.Fatalf("reprompt does not report the reprompt:\n%s", out.String())
	}
}

// TestRefreshRefusesItsOwnPane: a member in hergy's own pane cannot be
// restarted, because stopping it would stop hergy first.
func TestRefreshRefusesItsOwnPane(t *testing.T) {
	port := &fakePort{
		agents: map[string]protocol.Agent{
			"team-lead": {Name: "team-lead", Kind: "claude", Status: "idle", PaneID: "own"},
		},
		gone:    map[string]bool{},
		process: map[string][2]int{"own": {4242, 900}},
	}
	l, _ := downLauncher(t, port, "team-lead")
	var killed []int
	l.kill = func(pgid int) error { killed = append(killed, pgid); return nil }
	members := l.starts

	if err := l.Refresh(members, "own"); err == nil {
		t.Fatal("Refresh accepted its own pane")
	}
	if len(killed) != 0 {
		t.Fatalf("refresh stopped its own process group: %v", killed)
	}
	if slices.Contains(commandTrail(port.calls), "pane process-info") {
		t.Fatalf("refresh read process info for its own pane:\n%v", port.calls)
	}
}

// TestRefreshAllRefusesWhenSelfIsIncluded: with --all, a self member stops the
// whole command before any member is stopped.
func TestRefreshAllRefusesWhenSelfIsIncluded(t *testing.T) {
	port := &fakePort{
		agents: map[string]protocol.Agent{
			"team-lead":   {Name: "team-lead", Kind: "claude", Status: "idle", PaneID: "p1"},
			"team-worker": {Name: "team-worker", Kind: "claude", Status: "idle", PaneID: "own"},
		},
		gone: map[string]bool{},
	}
	l, _ := downLauncher(t, port, "team-lead", "team-worker")
	var killed []int
	l.kill = func(pgid int) error { killed = append(killed, pgid); return nil }
	members := l.starts

	if err := l.Refresh(members, "own"); err == nil {
		t.Fatal("Refresh --all accepted its own pane")
	}
	if len(killed) != 0 {
		t.Fatalf("refresh --all stopped a member before checking them all: %v", killed)
	}
	if slices.Contains(commandTrail(port.calls), "pane process-info") {
		t.Fatalf("refresh --all read process info before checking them all:\n%v", port.calls)
	}
}

// TestRefreshDryShowsTheSameRefusal: --dry reads the live agents and shows the
// self refusal, and still stops nothing.
func TestRefreshDryShowsTheSameRefusal(t *testing.T) {
	port := &fakePort{
		agents: map[string]protocol.Agent{
			"team-lead": {Name: "team-lead", Kind: "claude", Status: "idle", PaneID: "own"},
		},
		gone:    map[string]bool{},
		process: map[string][2]int{"own": {4242, 900}},
	}
	l, _ := downLauncher(t, port, "team-lead")
	l.kill = func(pgid int) error { t.Fatalf("dry refresh called kill(%d)", pgid); return nil }
	members := l.starts

	if _, err := l.RefreshDry(members, "own"); err == nil {
		t.Fatal("RefreshDry accepted its own pane")
	}
	if len(port.verified) != 0 {
		t.Fatalf("RefreshDry checked the herdr commands: %v", port.verified)
	}
	if slices.Contains(commandTrail(port.calls), "pane process-info") {
		t.Fatalf("dry refresh read process info:\n%v", port.calls)
	}
}

// TestRefreshRechecksBeforeStopping: a member that starts working between the
// first check and the stop is not stopped.
func TestRefreshRechecksBeforeStopping(t *testing.T) {
	port := &fakePort{
		agents: map[string]protocol.Agent{
			"team-lead": {Name: "team-lead", Kind: "claude", Status: "idle", PaneID: "p1"},
		},
		gone:    map[string]bool{},
		process: map[string][2]int{"p1": {4242, 900}},
	}
	// The first check (by name) sees idle; the check just before the stop (by
	// pane) sees the member start working.
	port.onAgentGet = func(target string) {
		if target != "p1" {
			return
		}
		agent := port.agents["team-lead"]
		agent.Status = "working"
		port.agents["team-lead"] = agent
	}
	l, _ := downLauncher(t, port, "team-lead")
	var killed []int
	l.kill = func(pgid int) error { killed = append(killed, pgid); return nil }
	members := l.starts

	if err := l.Refresh(members, ""); err == nil {
		t.Fatal("Refresh stopped a member that started working after the first check")
	}
	if len(killed) != 0 {
		t.Fatalf("refresh stopped a working member's process group: %v", killed)
	}
	if slices.Contains(commandTrail(port.calls), "pane process-info") {
		t.Fatalf("refresh read process info for a working member:\n%v", port.calls)
	}
}

// TestDownByNameClosesWhatTheTeamStarted: when the team definition behind a
// running team cannot be read, down closes the member agents the caller found and
// the workers in "worker:<team>/" tabs. A definition is what a prompt is built
// from, and closing needs no prompt (r-e494's follow-up).
func TestDownByNameClosesWhatTheTeamStarted(t *testing.T) {
	port := &fakePort{
		live: []protocol.Agent{
			{Name: "qw2-m", Kind: "codex", Status: "blocked", PaneID: "p-m", TabID: "t-team"},
			{Name: "qw2-w1", Kind: "codex", Status: "idle", PaneID: "p-w1", TabID: "t-w1"},
			{Name: "qw-other", Kind: "claude", Status: "idle", PaneID: "p-other", TabID: "t-other"},
			{Name: "notes-m", Kind: "claude", Status: "idle", PaneID: "p-notes", TabID: "t-notes"},
		},
		tabs: []protocol.Tab{
			{ID: "t-team", Label: "qw2"},
			{ID: "t-w1", Label: "worker:qw2/w1"},
			{ID: "t-other", Label: "team:qw@qw"},
			{ID: "t-notes", Label: "notes"},
		},
	}
	l, out := downLauncherFor(t, port, "qw2")

	if _, err := l.DownByName([]string{"qw2-m"}, "", false); err != nil {
		t.Fatalf("DownByName: %v", err)
	}
	want := []string{"p-m", "p-w1"}
	if !slices.Equal(port.closed, want) {
		t.Fatalf("closed %v, want %v", port.closed, want)
	}
	for _, name := range []string{"qw-other", "notes-m"} {
		if strings.Contains(out.String(), name) {
			t.Errorf("down by name reports %s, which is not this team's:\n%s", name, out.String())
		}
	}
	if !strings.Contains(out.String(), "qw2-m: closing (pane p-m)\n") {
		t.Errorf("down by name does not report the member:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "closed: 2\n") {
		t.Errorf("down by name does not count what it closed:\n%s", out.String())
	}
}

// TestDownByNameHoldsTheOwnPaneBack: hergy's own pane is closed last, however it
// was found.
func TestDownByNameHoldsTheOwnPaneBack(t *testing.T) {
	port := &fakePort{live: []protocol.Agent{
		{Name: "qw2-a", Kind: "codex", Status: "idle", PaneID: "p1"},
		{Name: "qw2-b", Kind: "codex", Status: "idle", PaneID: "own"},
	}}
	l, out := downLauncherFor(t, port, "qw2")
	if _, err := l.DownByName([]string{"qw2-a", "qw2-b"}, "own", false); err != nil {
		t.Fatalf("DownByName: %v", err)
	}
	if want := []string{"p1", "own"}; !slices.Equal(port.closed, want) {
		t.Fatalf("closed %v, want %v", port.closed, want)
	}
	if !strings.Contains(out.String(), "own pane; last") {
		t.Errorf("down by name does not say which pane went last:\n%s", out.String())
	}
}

// TestWaitGoneSaysWhichMemberToRefresh: an agent herdr still knows after the
// timeout is named whole — the agent, the wait, and the member a person refreshes
// — rather than with an unfilled placeholder in the command to run (r-bac8).
func TestWaitGoneSaysWhichMemberToRefresh(t *testing.T) {
	agent := protocol.Agent{Name: "core-lead", Kind: "claude", Status: "idle", PaneID: "p1"}
	port := &fakePort{
		live:   []protocol.Agent{agent},
		agents: map[string]protocol.Agent{"core-lead": agent},
	}
	l, _ := downLauncher(t, port, "core-lead")
	l.timeout = 1

	err := l.waitGone("core-lead")
	if err == nil {
		t.Fatal("waitGone answered no error while herdr still knew the agent")
	}
	text := err.Error()
	if strings.Contains(text, "%!") {
		t.Errorf("the sentence has an unfilled placeholder: %q", text)
	}
	want := i18n.Errorf("error.agent_not_gone", "core-lead", 1, "core-lead").Error()
	if text != want {
		t.Errorf("the sentence is %q, want %q", text, want)
	}
	if !strings.Contains(text, "hergy refresh core-lead") {
		t.Errorf("the sentence does not say which member to refresh: %q", text)
	}
}
