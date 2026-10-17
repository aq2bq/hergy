package web

import (
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/aq2bq/hergy/internal/activity"
	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/effect"
)

// activityList is the activity screen (設計 3.1): one section per running team,
// waiting teams first, with what each is doing and how to reach the pane that
// waits.
func (s *Server) activityList(_ *http.Request) (view, error) {
	data := activityData{Crumb: []crumb{{Name: t("web.band.activity")}}, Token: s.token}
	if s.focused != "" {
		data.Focused = tf("web.activity.focused", s.focused)
		s.focused = ""
	}
	running, err := s.port.Running()
	if err != nil {
		data.ErrLine = tf("web.activity.herdr_error", err)
	} else {
		known, byName, unknown := s.sections(running, time.Now())
		activity.Sort(known)
		sections := make([]teamSection, 0, len(running))
		for _, verdict := range known {
			sections = append(sections, byName[verdict.Name])
		}
		data.Count = ""
		if len(known) > 0 {
			data.Count = countsLine(len(known), waitingTeams(sections))
		}
		// A team whose definition is not here is listed after the judged ones
		// and is in no count (L4): it has no state to count.
		sections = append(sections, unknown...)
		data.Teams = sections
		data.Empty = len(data.Teams) == 0
		data.UpCommand = s.upCommand()
	}
	return s.pageBody(view{
		title: t("web.activity.title"),
		band:  bandActivity,
		crumb: data.Crumb,
	}, "activity.html", data)
}

// teamActivity is the detail of one running team (設計 3.2): the same figure
// with each node's state overlaid, and what a member hired.
func (s *Server) teamActivity(r *http.Request) (view, error) {
	name := r.PathValue("team")
	running, err := s.port.Running()
	if err != nil {
		return view{}, err
	}
	var entry effect.Running
	found := false
	for _, one := range running {
		if one.Team == name {
			entry, found = one, true
			break
		}
	}
	data := teamData{Name: name, Href: teamPath(name)}
	if s.focused != "" {
		data.Focused = tf("web.activity.focused", s.focused)
		s.focused = ""
	}
	crumb := []crumb{{Name: t("web.band.activity"), Href: "/activity"}, {Name: tf("web.activity.team_crumb", name)}}
	if !found {
		data.NotRunning = true
		return s.pageBody(view{title: tf("web.activity.team_title", name), band: bandActivity, crumb: crumb}, "team.html", data)
	}
	if entry.Definition != name {
		data.From = tf("web.activity.from", entry.Definition)
		data.DefinitionHref = definitionPath(entry.Definition)
	}
	def, err := s.readDefinition(entry.Definition)
	if err != nil {
		data.Broken = s.noDefinition(err, entry)
		return s.pageBody(view{title: tf("web.activity.team_title", name), band: bandActivity, crumb: crumb}, "team.html", data)
	}
	verdict, err := s.judge(name, def, time.Now())
	if err != nil {
		return view{}, err
	}
	data.Line = verdict.Line
	data.Tail = verdict.Tail
	data.States = stateClasses(verdict.Nodes)
	data.Figure = s.figure(def, data.States, "")
	data.Hired = hiredLines(verdict.Hired)
	return s.pageBody(view{title: tf("web.activity.team_title", name), band: bandActivity, crumb: crumb}, "team.html", data)
}

// focus is the one thing the activity writes: where herdr's focus stands. It
// changes no file (P6).
func (s *Server) focus(r *http.Request) (view, error) {
	pane := r.PostFormValue("pane")
	if err := s.port.Focus(pane); err != nil {
		return view{}, err
	}
	s.focused = pane
	return s.activityList(r)
}

// sections judges each running team and turns it into a section, keeping the
// judgement so the list can be ordered by it (5.2). A team that cannot be judged
// (its definition is not here, or does not read) is returned apart: it is listed
// after the judged ones and is in no count (L4).
func (s *Server) sections(running []effect.Running, now time.Time) ([]activity.Team, map[string]teamSection, []teamSection) {
	known := make([]activity.Team, 0, len(running))
	byName := make(map[string]teamSection, len(running))
	var unknown []teamSection
	for _, entry := range running {
		section, verdict, ok := s.section(entry, now)
		if !ok {
			unknown = append(unknown, section)
			continue
		}
		known = append(known, verdict)
		byName[entry.Team] = section
	}
	sort.Slice(unknown, func(i, j int) bool { return unknown[i].Name < unknown[j].Name })
	return known, byName, unknown
}

// section is one team's section in the activity list, with the judgement it was
// built from. The judgement is absent (ok false) when the team cannot be judged.
func (s *Server) section(entry effect.Running, now time.Time) (teamSection, activity.Team, bool) {
	section := teamSection{Name: entry.Team, Href: teamPath(entry.Team)}
	verdict := activity.Team{Name: entry.Team}
	if entry.Definition != entry.Team {
		section.From = tf("web.activity.from", entry.Definition)
	}
	def, err := s.readDefinition(entry.Definition)
	if err != nil {
		section.Unjudged = true
		section.Line = s.noDefinition(err, entry)
		return section, verdict, false
	}
	judged, err := s.judge(entry.Team, def, now)
	if err != nil {
		section.Unjudged = true
		section.Line = tf("web.activity.herdr_error", err)
		return section, verdict, false
	}
	section.Line = judged.Line
	section.Tail = judged.Tail
	section.State = judged.State
	section.Questions = judged.Questions
	section.Panes = focusPanes(entry.Team, judged.Nodes)
	return section, judged, true
}

// noDefinition is the one line a team whose definition this place does not hold
// gets: the team, the definition it was started from, and that it is not here.
// The find-the-file sentence (the paths it looked in, and the next command) is
// for the one person who typed a name, not for the activity list (L4, E2).
func (s *Server) noDefinition(err error, entry effect.Running) string {
	if errors.Is(err, catalog.ErrNotFound) {
		if entry.Definition == entry.Team {
			return tf("web.activity.no_definition_same", entry.Team, entry.Definition)
		}
		return tf("web.activity.no_definition", entry.Team, entry.Definition, entry.Definition)
	}
	return s.breakLine(readErrorPath(err, s.teamYAMLOf(entry.Definition)), err)
}

// judge is one team's judgement: the agents as the Port reports them, the pane
// tail of the node that waits, and the watcher's memory of how long it has been
// there (設計 5.2).
func (s *Server) judge(team string, def *definition.Definition, now time.Time) (activity.Team, error) {
	agents, err := s.port.Agents(team)
	if err != nil {
		return activity.Team{}, err
	}
	seen := s.watcher.Observe(agents, now)
	verdict := activity.Of(activity.Input{
		Team:   team,
		Graph:  def.Graph,
		Agents: agents,
		Seen:   seen,
		Now:    now,
	})
	if verdict.Last != "" {
		if pane := herdrNameOf(agents, verdict.Last); pane != "" {
			if tail, err := s.port.Tail(pane, partCount); err == nil {
				verdict.Tail = tail
			}
		}
	}
	return verdict, nil
}

// herdrNameOf is the herdr name of the agent that is one node.
func herdrNameOf(agents []activity.Agent, node string) string {
	for _, agent := range agents {
		if agent.Node == node {
			return agent.Herdr
		}
	}
	return ""
}

// stateClasses is each node's live state as the figure's class.
func stateClasses(nodes []activity.Node) map[string]string {
	out := make(map[string]string, len(nodes))
	for _, node := range nodes {
		out[node.Name] = stateClass(node.State)
	}
	return out
}

// stateClass is one node state as the figure's class.
func stateClass(state activity.NodeState) string {
	switch state {
	case activity.Active:
		return "active"
	case activity.Blocked:
		return "blocked"
	case activity.HandedOver:
		return "handed"
	case activity.Idle:
		return "idle"
	default:
		return ""
	}
}

// hiredLines is the workers a member hired, as one line each: the herdr name and
// the state.
func hiredLines(hired []activity.Node) []string {
	out := make([]string, 0, len(hired))
	for _, node := range hired {
		out = append(out, dot(node.Name, stateWord(node.State)))
	}
	return out
}

// stateWord is one node state as a person reads it.
func stateWord(state activity.NodeState) string {
	switch state {
	case activity.Active:
		return t("activity.working")
	case activity.Blocked, activity.HandedOver:
		return t("web.activity.waiting")
	case activity.Idle:
		return t("activity.stopped")
	default:
		return ""
	}
}

// focusPanes is the "move to the pane" button of each node that waits on the
// human.
func focusPanes(team string, nodes []activity.Node) []focusPane {
	var out []focusPane
	for _, node := range nodes {
		if node.Herdr == "" {
			continue
		}
		if node.State != activity.Blocked && node.State != activity.HandedOver {
			continue
		}
		out = append(out, focusPane{Label: tf("web.activity.focus", node.Name), Pane: node.Herdr, Team: team})
	}
	return out
}

// countsLine is the one line of numbers under the title: how many teams wait on
// the human, and how many run. Zero waiting teams are not said (設計 3.1).
func countsLine(running, waiting int) string {
	if waiting == 0 {
		return tf("web.activity.count_running", running)
	}
	return tf("web.activity.count", waiting, running)
}

// waitingTeams is how many sections wait on the human.
func waitingTeams(teams []teamSection) int {
	n := 0
	for _, section := range teams {
		if section.State == activity.WaitingAnswer || section.State == activity.WaitingTurn {
			n++
		}
	}
	return n
}

// upCommand is the command the empty activity shows: the one team definition's
// name, or a placeholder when there are several.
func (s *Server) upCommand() string {
	entries := s.entries()
	if len(entries) == 1 {
		return tf("web.activity.up", entries[0].Name)
	}
	return tf("web.activity.up_many")
}

// activityData is what the activity list template is given.
type activityData struct {
	Crumb     []crumb
	Count     string
	Teams     []teamSection
	Empty     bool
	ErrLine   string
	UpCommand string
	Focused   string
	Token     string
}

// teamSection is one running team's section.
type teamSection struct {
	Name      string
	Unjudged  bool
	Href      string
	From      string
	Line      string
	Tail      string
	State     activity.State
	Questions []activity.Question
	Panes     []focusPane
}

// focusPane is one "move to the pane" button.
type focusPane struct {
	Label string
	Pane  string
	Team  string
}

// teamData is what the team detail template is given.
type teamData struct {
	Name           string
	Href           string
	From           string
	DefinitionHref string
	Line           string
	Tail           string
	Figure         figureData
	States         map[string]string
	Hired          []string
	NotRunning     bool
	Broken         string
	Focused        string
}
