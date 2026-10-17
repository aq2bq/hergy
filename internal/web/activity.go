package web

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/aq2bq/hergy/internal/activity"
	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/effect"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/source"
)

// activityList is the activity screen (設計 3.1): one section per running team,
// waiting teams first, with what each is doing and how to reach the pane that
// waits.
func (s *Server) activityList(_ *http.Request) (view, error) {
	return s.activityView("")
}

// activityView draws the activity with one line the response alone shows: the
// pane a press moved to, or the reason it could not (N4). Nothing is kept on the
// server, so another tab or the next redraw does not repeat it.
func (s *Server) activityView(focused string) (view, error) {
	data := activityData{Crumb: []crumb{{Name: t("web.band.activity")}}, Token: s.token, Focused: focused}
	running, err := s.port.Running()
	if err != nil {
		data.ErrLine = tf("web.activity.herdr_error", err)
	} else {
		now := time.Now()
		agents, agentErrs, seen := s.observeAll(running, now)
		questions, qerr := s.port.Questions()
		if qerr != nil {
			// A gy that is there and cannot be read says so where the questions
			// would stand; no gy at all is silent (I17, 設計 3.1).
			data.QuestionErr = tf("web.activity.gy_error", qerr)
		} else {
			data.Questions = questionLines(questions)
		}
		known, byName, unknown := s.sections(running, agents, agentErrs, seen, questions, now)
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
// with each node's state overlaid, what a member hired, when the team was
// launched and what it still has to take up, and the questions its own writers
// asked.
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
	crumb := []crumb{{Name: t("web.band.activity"), Href: "/activity"}, {Name: tf("web.activity.team_crumb", name)}}
	page := view{title: tf("web.activity.team_title", name), band: bandActivity, crumb: crumb}
	if !found {
		data.NotRunning = true
		return s.pageBody(page, "team.html", data)
	}
	def, foreign, err := s.teamDefinition(entry)
	if err != nil {
		data.Broken = s.noDefinition(err, entry)
		return s.pageBody(page, "team.html", data)
	}
	launch, launched := s.port.Launched(entry.Team)
	if entry.Definition != name {
		data.From = s.fromLine(entry, foreign, launch, launched)
		if !foreign {
			// The definition's own page, as its own line: the line above it
			// carries the definition's name and the time, and a link inside it
			// would stand between the two parts (spec/43 N2, 設計 3.2).
			data.Definition = entry.Definition
			data.DefinitionHref = definitionPath(entry.Definition)
		}
	}
	now := time.Now()
	agents, agentErrs, seen := s.observeAll(running, now)
	if err := agentErrs[name]; err != nil {
		return view{}, err
	}
	questions, qerr := s.port.Questions()
	if qerr != nil {
		data.QuestionErr = tf("web.activity.gy_error", qerr)
	}
	verdict := s.judge(name, def, agents[name], teamQuestions(questions, agents[name]), seen, now)
	data.Line = verdict.Line
	data.Tail = verdict.Tail
	data.States = stateClasses(verdict.Nodes)
	data.Figure = s.figure(def, data.States, nil, "")
	if foreign {
		// A team from another project has no definition or field pages here, so
		// its figure opens nothing (W16).
		data.Figure = plainFigure(data.Figure)
	}
	data.Hired = hiredLines(verdict.Hired)
	data.Questions = questionLines(verdict.Questions)
	if launched {
		data.Launched = launchedText(entry, launch)
		changes := s.deliveredChanges(entry, def, foreign, launch)
		data.Undelivered = effect.UndeliveredLines(changes)
		if len(changes) > 0 {
			data.Handover = handoverOf(effect.Commands(changes, []effect.Running{{Team: entry.Team, Definition: def.Name}}))
		}
	}
	return s.pageBody(page, "team.html", data)
}

// deliveredChanges is what a running team still has to take up: the launch
// record's prompt hashes compared with the prompts as they read now (C9). The
// prompts are rebuilt the way the launch built them, from the facts the record
// kept; a record from before those were kept gives no change, because a prompt
// cannot be rebuilt to match it (spec/43 S1). A team from another project is
// rebuilt from that project's own catalog (T1).
func (s *Server) deliveredChanges(entry effect.Running, def *definition.Definition, foreign bool, launch Launch) []effect.Change {
	if foreign {
		view, ok := s.port.LaunchedView(entry.Team)
		if !ok {
			return nil
		}
		return effect.Undelivered(launch.Prompts, view)
	}
	if launch.Executable == "" || launch.Lang == "" {
		return nil
	}
	return effect.Undelivered(launch.Prompts, s.teamView(def, entry.Team, launch, s.cat.Reader()))
}

// fromLine is how a running team's own line names the definition it was booted
// from, or "" when it runs from a definition of its own name: "（agile から起動）",
// and, for a team from another project, the place that project stands, so a
// person can tell it from this place's own teams of the same shape (W16,
// spec/43 N3). It is plain text: a link in it would stand between the two parts
// of the line.
func (s *Server) fromLine(entry effect.Running, foreign bool, launch Launch, launched bool) string {
	if foreign && launched && launch.Project != "" {
		return tf("web.activity.from_place", entry.Definition, s.cat.Display(launch.Project))
	}
	return tf("web.activity.from", entry.Definition)
}

// teamDefinition is the definition a running team was booted from: this place's
// own file, or, when this place holds none, the project the launch record names
// (W16). foreign says the second case, so the screens draw no link to a
// definition page this place cannot answer. A definition that is here but does
// not read keeps its own error (L4).
func (s *Server) teamDefinition(entry effect.Running) (def *definition.Definition, foreign bool, err error) {
	def, err = s.readDefinition(entry.Definition)
	if err == nil {
		return def, false, nil
	}
	if !errors.Is(err, catalog.ErrNotFound) {
		return nil, false, err
	}
	other, otherErr := s.port.LaunchedDefinition(entry.Team)
	if otherErr != nil {
		return nil, false, err
	}
	return other, true, nil
}

// plainFigure is a figure whose boxes and arrows open nothing: a team from
// another project has no definition page here (W16).
func plainFigure(f figureData) figureData {
	f.Human.Href = ""
	for i := range f.Members {
		f.Members[i].Href = ""
		for j := range f.Members[i].Workers {
			f.Members[i].Workers[j].Href = ""
		}
	}
	for i := range f.Edges {
		f.Edges[i].Href = ""
	}
	return f
}

// teamView is one running team as effect reads it: the definition as it stands,
// with the team's own name in the prompt facts, because a prompt names each
// member by its herdr name, and the facts the launch record kept — the hergy
// binary, the project root and the language its prompts were written in — so the
// prompt is rebuilt the way the launch built it (C9).
func (s *Server) teamView(def *definition.Definition, team string, launch Launch, reader source.Reader) effect.View {
	profiles, _, _ := s.cat.Profiles()
	return effect.View{
		Def:      def,
		Profiles: profiles,
		Run: prompt.Run{
			Team:       team,
			Executable: launch.Executable,
			Base:       launch.Project,
			Lang:       launch.Lang,
			Reader:     reader,
			Sources:    s.cat,
			Display:    s.cat.Display,
		},
	}
}

// launchedText is the launch record's time as the team's first line shows it:
// with a label when the line carries no definition name, so the bare time is
// not read as anything else (設計 3.2).
func launchedText(entry effect.Running, launch Launch) string {
	at := stamp(launch.At)
	if at == "" {
		return ""
	}
	if entry.Definition != entry.Team {
		return at
	}
	return tf("web.activity.launched", at)
}

// stamp is one time as a screen writes it, in this machine's own zone:
// "10/17 09:12" (設計 3.2). A time gy wrote in UTC is read here, so the time a
// question was asked is the time the person's clock shows (spec/43 N1).
func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("01/02 15:04")
}

// focus is the one thing the activity writes: where herdr's focus stands. It
// changes no file (P6). The pane has to be one of the team's own agents: a name
// from the form is not trusted, and a name that is not there is one line rather
// than a focus moved somewhere else (N4).
func (s *Server) focus(r *http.Request) (view, error) {
	team := r.PathValue("team")
	pane := r.PostFormValue("pane")
	agents, err := s.port.Agents(team)
	if err != nil {
		return s.activityView(tf("web.activity.herdr_error", err))
	}
	for _, agent := range agents {
		if agent.Herdr != pane {
			continue
		}
		if err := s.port.Focus(pane); err != nil {
			return view{}, err
		}
		node := agent.Node
		if node == "" {
			node = agent.Herdr
		}
		return s.activityView(tf("web.activity.moved", team, node))
	}
	return s.activityView(t("web.activity.no_pane"))
}

// observeAll reads every running team's agents and hands them to the watcher once
// per redraw: the watcher forgets the agents it is not handed, so observing one
// team at a time would take every other team's length again on the next draw
// (設計 5.2、spec/39 S2). The result is shared by every section of this answer.
func (s *Server) observeAll(running []effect.Running, now time.Time) (map[string][]activity.Agent, map[string]error, map[string]activity.Watch) {
	byTeam := make(map[string][]activity.Agent, len(running))
	errs := make(map[string]error, len(running))
	var all []activity.Agent
	for _, entry := range running {
		agents, err := s.port.Agents(entry.Team)
		if err != nil {
			errs[entry.Team] = err
			continue
		}
		byTeam[entry.Team] = agents
		all = append(all, agents...)
	}
	return byTeam, errs, s.watcher.Observe(all, now)
}

// sections judges each running team and turns it into a section, keeping the
// judgement so the list can be ordered by it (5.2). A team that cannot be judged
// (its definition is not here, nor in the project its launch record names, or
// does not read) is returned apart: it is listed after the judged ones and is in
// no count (L4).
func (s *Server) sections(running []effect.Running, agents map[string][]activity.Agent, agentErrs map[string]error, seen map[string]activity.Watch, questions []activity.Question, now time.Time) ([]activity.Team, map[string]teamSection, []teamSection) {
	known := make([]activity.Team, 0, len(running))
	byName := make(map[string]teamSection, len(running))
	var unknown []teamSection
	for _, entry := range running {
		section, verdict, ok := s.section(entry, agents[entry.Team], agentErrs[entry.Team], seen, questions, now)
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
func (s *Server) section(entry effect.Running, agents []activity.Agent, agentsErr error, seen map[string]activity.Watch, questions []activity.Question, now time.Time) (teamSection, activity.Team, bool) {
	section := teamSection{Name: entry.Team, Href: teamPath(entry.Team)}
	verdict := activity.Team{Name: entry.Team}
	def, foreign, err := s.teamDefinition(entry)
	if entry.Definition != entry.Team {
		launch, launched := s.port.Launched(entry.Team)
		section.From = s.fromLine(entry, foreign, launch, launched)
	}
	if err != nil {
		section.Unjudged = true
		section.Line = s.noDefinition(err, entry)
		return section, verdict, false
	}
	if agentsErr != nil {
		section.Unjudged = true
		section.Line = tf("web.activity.herdr_error", agentsErr)
		return section, verdict, false
	}
	judged := s.judge(entry.Team, def, agents, teamQuestions(questions, agents), seen, now)
	section.Line = judged.Line
	section.Tail = judged.Tail
	section.State = judged.State
	section.Questions = questionLines(judged.Questions)
	section.Panes = focusPanes(entry.Team, judged.Nodes)
	return section, judged, true
}

// noDefinition is the one line a team whose definition this place does not hold
// gets: that the definition is not here. The team's own name and the definition
// it was started from are already on the line above it, so they are not repeated
// (walk-6 P1). The find-the-file sentence (the paths it looked in, and the next
// command) is for the one person who typed a name, not for the activity list
// (L4, E2).
func (s *Server) noDefinition(err error, entry effect.Running) string {
	if errors.Is(err, catalog.ErrNotFound) {
		return tf("web.activity.no_definition", entry.Definition)
	}
	return s.breakLine(readErrorPath(err, s.teamYAMLOf(entry.Definition)), err)
}

// judge is one team's judgement: the agents the caller read, the pane tail of
// the node that waits, and the watcher's memory of how long it has been there
// (設計 5.2). The watcher is not called here: the caller observed every team's
// agents once (S2).
func (s *Server) judge(team string, def *definition.Definition, agents []activity.Agent, questions []activity.Question, seen map[string]activity.Watch, now time.Time) activity.Team {
	verdict := activity.Of(activity.Input{
		Team:      team,
		Graph:     def.Graph,
		Agents:    agents,
		Questions: questions,
		Seen:      seen,
		Now:       now,
	})
	if verdict.Last != "" {
		if pane := herdrNameOf(agents, verdict.Last); pane != "" {
			if tail, err := s.port.Tail(pane, partCount); err == nil {
				verdict.Tail = tail
			}
		}
	}
	return verdict
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
	Crumb       []crumb
	Count       string
	Teams       []teamSection
	Empty       bool
	ErrLine     string
	UpCommand   string
	Focused     string
	Token       string
	Questions   []questionLine
	QuestionErr string
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
	Questions []questionLine
	Panes     []focusPane
}

// questionLine is one of gy's open questions as a screen shows it: its title,
// its options, and who decides it and when it was asked (設計 3.1・5.3).
type questionLine struct {
	Title   string
	Options string
	Meta    string
}

// questionLines is the questions as lines, in gy's order.
func questionLines(questions []activity.Question) []questionLine {
	out := make([]questionLine, 0, len(questions))
	for _, question := range questions {
		out = append(out, questionLine{
			Title:   question.Title,
			Options: questionOptions(question.Options),
			Meta:    questionMeta(question),
		})
	}
	return out
}

// questionOptions is a question's options as one line.
func questionOptions(options []string) string {
	if len(options) == 0 {
		return ""
	}
	return tf("web.activity.question_options", strings.Join(options, t("list.separator")))
}

// questionMeta is who decides a question and when it was asked.
func questionMeta(question activity.Question) string {
	at := stamp(question.At)
	if at == "" {
		return tf("web.activity.question_decider", question.Decider)
	}
	return tf("web.activity.question_meta", question.Decider, at)
}

// teamQuestions is the questions one team's own section carries: the ones whose
// writer is one of the team's herdr names (設計 5.3, W4). A question asked by
// anyone else stands only under the project's own questions.
func teamQuestions(questions []activity.Question, agents []activity.Agent) []activity.Question {
	var out []activity.Question
	for _, question := range questions {
		if question.Writer == "" {
			continue
		}
		for _, agent := range agents {
			if agent.Herdr == question.Writer {
				out = append(out, question)
				break
			}
		}
	}
	return out
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
	Definition     string
	DefinitionHref string
	Line           string
	Tail           string
	Figure         figureData
	States         map[string]string
	Hired          []string
	NotRunning     bool
	Broken         string
	Questions      []questionLine
	QuestionErr    string
	Launched       string
	Undelivered    []string
	Handover       handover
}
