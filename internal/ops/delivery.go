package ops

import (
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/activity"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/catalog"
	"github.com/aq2bq/hergy/internal/ops/internal/deliver"
	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
	"github.com/aq2bq/hergy/internal/ops/internal/prompt"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
	"github.com/aq2bq/hergy/internal/words"
)

// DeliveryPlan is one reading of the running Members: the drift the apply sheet
// reads, and every live member the request may name, whether it changed or not,
// so the API can tell "nothing to apply" from "not running".
type DeliveryPlan struct {
	Apply   ApplyList
	Members map[string]PlannedMember
}

// PlannedMember binds a member to its run, drift and execution choice.
type PlannedMember struct {
	Team    RunningTeam
	Start   definition.Start
	State   string
	Changed bool
	Changes []Change
	How     ApplyHow
	WhyNot  string
	Rule    words.RuleID
}

// CanApply says whether hergy may apply the drift to this member now: a member
// with no drift of its own is not in the plan, a start needs no waiting, and a
// send or a restart needs a member that is not working (d-aacf).
func (m PlannedMember) CanApply() bool {
	switch m.How {
	case ApplyCannot:
		return false
	case ApplyStart:
		return true
	default:
		return sendable(m.State)
	}
}

// byHerdrOf is the judged members by the name herdr knows them by: the members
// an agent is live for.
func byHerdrOf(judged activity.Team) map[string]activity.Node {
	out := map[string]activity.Node{}
	for _, node := range judged.Nodes {
		if node.Herdr.String() != "" {
			out[node.Herdr.String()] = node
		}
	}
	return out
}

// planTeam adds one running team's drift to the plan: the members whose prompt,
// start or env differs, the members the definition has and no agent is live for,
// and the members the definition no longer has.
func planTeam(one *DeliveryPlan, running RunningTeam, judged activity.Team, byHerdr map[string]activity.Node) {
	row := ApplyTeam{
		Team: running.Name.String(), Definition: running.Definition,
		Project: running.Project, ProjectName: running.ProjectName,
		Notes: []string{}, Members: []ApplyMember{},
	}
	if running.Built == nil {
		// A definition that does not build has no member to Apply: the row says
		// why in the words hergy status uses (2.2). The row is here only when
		// an agent of the team is live, because otherwise there is no drift a
		// person can act on.
		if len(judged.Nodes) > 0 || len(judged.Hired) > 0 {
			row.Error = running.Broken
			one.Apply.Teams = append(one.Apply.Teams, row)
		}
		return
	}
	recorded := memberReceipts(running.state)
	oldRecord := false
	noted := map[string]bool{}
	addNote := func(text string) {
		if text == "" || noted[text] {
			return
		}
		noted[text] = true
		row.Notes = append(row.Notes, text)
	}
	for i := range running.Built.Starts {
		start := running.Built.Starts[i]
		node, live := byHerdr[start.Name.String()]
		if !live {
			// The definition has the member and no agent of the team is live
			// for it: hergy starts it (d-36ef).
			one.Members[start.Name.String()] = PlannedMember{
				Team: running, Start: start, State: "absent", Changed: true, How: ApplyStart,
			}
			row.Members = append(row.Members, ApplyMember{
				Name: start.Name.String(), Short: start.Short.String(), Role: start.Role,
				State: "absent", How: ApplyStart, Changes: []Change{}, CanApply: true,
			})
			continue
		}
		record, hasRecord := recorded[start.Name.String()]
		if hasRecord && !fingerprinted(record) {
			oldRecord = true
		}
		drift := readMemberDrift(running.Catalog, running, start, record, hasRecord)
		state := node.State.Slug()
		planned := PlannedMember{
			Team: running, Start: start, State: state,
			Changed: drift.any(), Changes: drift.Changes, How: drift.How(state),
		}
		if !planned.CanApply() {
			planned.Rule = drift.RefusalRule()
			planned.WhyNot = words.Errorf(planned.Rule, i18n.Language(), start.Short.String()).Error()
		}
		one.Members[start.Name.String()] = planned
		for _, change := range drift.Changes {
			if change.Kind == "worker_profile" {
				addNote(i18n.Tf("apply.note_worker", change.Worker))
			}
		}
		if !drift.any() {
			continue
		}
		applied := ApplyMember{
			Name: start.Name.String(), Short: start.Short.String(), Role: start.Role,
			State: state, How: drift.How(state), Changes: drift.Changes,
			CanApply: drift.CanApply(state), ConversationLost: drift.How(state) == ApplyRestart,
		}
		if !applied.CanApply {
			applied.Rule = planned.Rule
			applied.WhyNot = planned.WhyNot
		}
		row.Members = append(row.Members, applied)
	}
	for _, note := range workerNotes(running.state.Launch().Workers, running.Built) {
		addNote(note)
	}

	if oldRecord {
		// Without a known process fingerprint a changed profile cannot be
		// distinguished from the one still running.
		addNote(i18n.T("apply.note_old_record"))
	}
	// A member the definition no longer has is still live: hergy cannot put it
	// right without stopping and starting the team (d-36ef). Only an agent the
	// running state holds as this team's member is that; the others are hired
	// workers, which a stop leaves alone too but which are outside the apply.
	for _, hired := range judged.Hired {
		if _, was := recorded[hired.Herdr.String()]; !was {
			continue
		}
		short := domainname.ShortIn(hired.Herdr, running.Name.Observation()).String()
		why := words.Errorf("apply.removed", i18n.Language(), short).Error()
		one.Members[hired.Herdr.String()] = PlannedMember{
			Team: running, State: hired.State.Slug(), Changed: true,
			How: ApplyCannot, WhyNot: why, Rule: "apply.removed",
		}
		row.Members = append(row.Members, ApplyMember{
			Name: hired.Herdr.String(), Short: short, State: hired.State.Slug(),
			How: ApplyCannot, Changes: []Change{}, CanApply: false,
			WhyNot: why, Rule: "apply.removed",
		})
	}
	if len(row.Members) > 0 {
		one.Apply.Teams = append(one.Apply.Teams, row)
		one.Apply.Count += len(row.Members)
	}
}

// workerNotes is the team's own note: a worker entry's profile changed since the
// launch, so the next worker hired from it gets the new engine, while the workers
// already hired keep theirs (d-36ef). A run with no worker entries says nothing.
func workerNotes(workers []runningstate.WorkerDefinition, r *Resolved) []string {
	if len(workers) == 0 {
		return nil
	}
	before := map[string]runningstate.WorkerDefinition{}
	for _, worker := range workers {
		before[worker.From+"/"+worker.Purpose] = worker
	}
	var out []string
	for _, worker := range r.Team.Definition.Graph.Workers {
		was, ok := before[worker.From+"/"+worker.Purpose.String()]
		if !ok {
			continue
		}
		p, ok := r.Profiles[worker.Profile]
		if !ok {
			continue
		}
		if was.Profile == worker.Profile &&
			was.Start == launchlog.StartFingerprint(p.Kind, p.Args) &&
			was.Env == launchlog.EnvFingerprint(p.Env) {
			continue
		}
		out = append(out, i18n.Tf("apply.note_worker", worker.Purpose))
	}
	return out
}

// memberDrift is what one live member differs in: its prompt, how it is started,
// and its environment. The start and the env come from the fingerprints the
// running state keeps, so a changed profile is told from a changed prompt; a
// receipt without known fingerprints leaves both false.
type memberDrift struct {
	prompt  bool
	Start   bool
	env     bool
	Changes []Change
}

// any says whether hergy has anything to apply to the member.
func (d memberDrift) any() bool { return d.prompt || d.Start || d.env }

// how is the way hergy would put the member right (d-3d30): an env change needs
// the pane to be made again, a changed start restarts the agent, and a changed
// prompt is sent again. A member that is working is not touched now, whatever
// the drift is (d-aacf).
func (d memberDrift) How(state string) ApplyHow {
	switch {
	case d.env:
		return ApplyCannot
	case !sendable(state):
		return ApplyCannot
	case d.Start:
		return ApplyRestart
	default:
		return ApplyResend
	}
}

// CanApply says whether hergy may apply the member now.
func (d memberDrift) CanApply(state string) bool { return d.How(state) != ApplyCannot }

// RefusalRule keeps the reason in the operation's result; CLI and HTTP
// adapters present the same record's explanation.
func (d memberDrift) RefusalRule() words.RuleID {
	if d.env {
		return "apply.env-changed"
	}
	return "apply.state"
}

// memberDrift reads one live member's drift: the prompt comparison, and the
// fingerprints the running state keeps of the start and the env.
func readMemberDrift(cat *catalog.Catalog, running RunningTeam, start definition.Start, recorded runningstate.Receipt, hasRecord bool) memberDrift {
	var drift memberDrift
	drift.prompt, drift.Changes = memberChanges(cat, running, start, recorded, hasRecord)
	if hasRecord {
		if recorded.Start != "" {
			drift.Start = recorded.Start != launchlog.StartFingerprint(start.Kind, start.Args)
		}
		if recorded.Env != "" {
			drift.env = recorded.Env != launchlog.EnvFingerprint(start.Env)
		}
	}
	if drift.Start {
		drift.Changes = append(drift.Changes, Change{Kind: "start", Text: i18n.T("apply.start_changed")})
	}
	if drift.env {
		drift.Changes = append(drift.Changes, Change{Kind: "env", Text: i18n.T("apply.env_changed")})
	}
	return drift
}

// fingerprinted says this receipt identifies how the member's process started.
func fingerprinted(receipt runningstate.Receipt) bool {
	return receipt.Start != "" && receipt.Env != ""
}

// memberChanges says whether the prompt this member last read differs from the
// prompt its current definition builds, and what changed. The old prompt and
// documents come from the running state; the current prompt is built with the
// facts recorded at that receipt, so the comparison is about the definition, not
// about which hergy or project is reading it now.
func memberChanges(cat *catalog.Catalog, running RunningTeam, start definition.Start, recorded runningstate.Receipt, hasRecord bool) (bool, []Change) {
	if !hasRecord {
		return false, nil
	}
	old := recorded.Prompt
	node, ok := running.Built.Team.Definition.Graph.Member(start.Short.String())
	if !ok {
		return false, nil
	}
	run := prompt.Run{
		Team: domainname.RunningNames(running.Built.Team.Name), Executable: recorded.Executable, Base: running.Project, Lang: recorded.Lang,
		Reader: cat.Reader(), Sources: cat, Display: cat.Display,
	}
	if run.Executable == "" {
		run.Executable = executable()
	}
	if run.Base.IsZero() {
		// The team's own project, not the one serve was started in: a team of
		// another listed project is built from that project (r-2e49).
		run.Base = running.Project
	}
	built, err := prompt.Member(running.Built.Team.Definition, &node, run)
	if err != nil {
		return true, []Change{{Kind: "prompt", Text: i18n.T("delivery.prompt_changed")}}
	}
	if string(old) == built.Text {
		return false, nil
	}
	current := start
	current.Prompt, current.Segments = built.Text, built.Segments
	current.RoleDoc, current.HasRole, current.Read = built.Role, built.HasRole, built.Read
	changes := readChanges(deliver.Compare(running.Built.ModelTeamName(), recorded.Lang, string(old), oldDocs(recorded), current, deliveryDocs(cat, built.Documents)))
	if len(changes) == 0 {
		changes = []Change{{Kind: "prompt", Text: i18n.T("delivery.prompt_changed")}}
	}
	return true, changes
}

// deliveryDocs projects the captured rule contents for comparison and display.
func deliveryDocs(cat *catalog.Catalog, documents []definition.ReadDocument) []deliver.Doc {
	var out []deliver.Doc
	for _, doc := range documents {
		out = append(out, deliver.Doc{Path: doc.Doc.Path, Display: cat.Display(doc.Doc.Path), Text: doc.Text})
	}
	return out
}

// sendable is whether a member may be handed a prompt now: a stopped member,
// or one that handed the turn to the human, which is also stopped in herdr
// (r-a8fa, d-1c07). A working or blocked member is not.
func sendable(state string) bool { return state == "idle" || state == "handed" }

// deliveryInput is the current snapshot and the visible project catalogs.
type deliveryInput struct {
	Views   []ProjectView
	Session session
	Now     time.Time
}

// computeDelivery calculates drift without depending on a web DTO.
func computeDelivery(in deliveryInput) DeliveryPlan {
	one := DeliveryPlan{Apply: ApplyList{Teams: []ApplyTeam{}, Herdr: ReadNote{OK: true}}, Members: map[string]PlannedMember{}}
	if in.Session.Err != nil {
		one.Apply.Herdr = ReadNote{Text: in.Session.Err.Error()}
		return one
	}
	for _, running := range readRunningTeamsIn(in.Views, in.Session, false) {
		judged := judgeOf(running, in.Session, in.Now)
		planTeam(&one, running, judged, byHerdrOf(judged))
	}
	return one
}
