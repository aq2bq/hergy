package ops

import (
	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/activity"
	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/deliver"
	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/source"
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
func planTeam(one *DeliveryPlan, running RunningTeam, judged activity.Team, byHerdr map[string]activity.Node, records map[domainplace.Project]map[string]map[string]launchlog.Record, log launchlog.Log, logErr error) {
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
	recorded := teamsRecorded(records, running.Project)[running.Name.String()]
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
		if hasRecord && !fingerprinted(record, start) {
			oldRecord = true
		}
		drift := readMemberDrift(running.Catalog, log, logErr, running, start, record, hasRecord)
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
	if newest, ok := newestRecord(recorded); ok {
		for _, note := range workerNotes(newest, running.Built) {
			addNote(note)
		}
	}
	if oldRecord {
		// The record was written before hergy kept the fingerprint: a profile
		// that changed since then cannot be told from one that did not, so the
		// sheet says so rather than claiming the member is current.
		addNote(i18n.T("apply.note_old_record"))
	}
	// A member the definition no longer has is still live: hergy cannot put it
	// right without stopping and starting the team (d-36ef). Only an agent the
	// launch record holds as this team's member is that; the others are hired
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

// newestRecord is the newest launch among the records of one team's Members: the
// team-level facts (its worker entries) are the same in each, so the newest is
// the one the note reads.
func newestRecord(recorded map[string]launchlog.Record) (launchlog.Record, bool) {
	var best launchlog.Record
	found := false
	for _, record := range recorded {
		if !found || record.Time > best.Time {
			best, found = record, true
		}
	}
	return best, found
}

// workerNotes is the team's own note: a worker entry's profile changed since the
// launch, so the next worker hired from it gets the new engine, while the workers
// already hired keep theirs (d-36ef). A record written before the worker
// fingerprints were kept says nothing.
func workerNotes(record launchlog.Record, r *Resolved) []string {
	if len(record.Workers) == 0 {
		return nil
	}
	before := map[string]launchlog.Worker{}
	for _, worker := range record.Workers {
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
// launch record keeps, so a changed profile is told from a changed prompt; a
// record written before the fingerprints were kept leaves both false.
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
// fingerprints the launch record keeps of the start and the env.
func readMemberDrift(cat *catalog.Catalog, log launchlog.Log, logErr error, running RunningTeam, start definition.Start, record launchlog.Record, hasRecord bool) memberDrift {
	var drift memberDrift
	drift.prompt, drift.Changes = memberChanges(cat, log, logErr, running, start, record, hasRecord)
	if recorded, ok := recordedMemberOf(record, start); ok && hasRecord {
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

// recordedMemberOf is one member in the launch record, by herdr name first and
// then by short name.
func recordedMemberOf(record launchlog.Record, start definition.Start) (launchlog.Member, bool) {
	for i := range record.Members {
		if record.Members[i].Name.String() == start.Name.String() || record.Members[i].Short == start.Short.String() {
			return record.Members[i], true
		}
	}
	return launchlog.Member{}, false
}

// fingerprinted says the record holds the fingerprints the drift needs.
func fingerprinted(record launchlog.Record, start definition.Start) bool {
	member, ok := recordedMemberOf(record, start)
	return ok && member.Start != "" && member.Env != ""
}

// memberChanges says whether the prompt this member last read differs from the
// prompt its current definition builds, and what changed. The old prompt and
// documents come from the launch record; the current prompt is built with the
// facts recorded at that launch, so the comparison is about the definition, not
// about which hergy or project is reading it now.
func memberChanges(cat *catalog.Catalog, log launchlog.Log, logErr error, running RunningTeam, start definition.Start, record launchlog.Record, hasRecord bool) (bool, []Change) {
	if !hasRecord || logErr != nil {
		return false, nil
	}
	recorded, ok := recordedMemberOf(record, start)
	if !ok {
		return false, nil
	}
	old, err := log.Text(recorded.Prompt)
	if err != nil {
		return false, nil
	}
	node, ok := running.Built.Team.Definition.Graph.Member(start.Short.String())
	if !ok {
		return false, nil
	}
	run := prompt.Run{
		Team: domainname.RunningNames(running.Built.Team.Name), Executable: record.Executable, Base: record.Project.Project(), Lang: record.Lang,
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
	changes := readChanges(deliver.Compare(running.Built.ModelTeamName(), record.Lang, string(old), oldDocs(log, record, recorded, string(old)), current, currentDocs(cat, built)))
	if len(changes) == 0 {
		changes = []Change{{Kind: "prompt", Text: i18n.T("delivery.prompt_changed")}}
	}
	return true, changes
}

// latestMemberRecords is, for each project, each of its teams and each of that
// team's members, the newest launch that handed that member a prompt. A reprompt
// records only the members it sent, so a team's newest line is not a snapshot of
// the others: each member's own last prompt is the one to compare.
//
// The project stands in the key: the headquarters shows several projects, and a
// record is the one its own project wrote (r-2e49). A record that names no project
// — one written before the record kept it — stands under "", and is read for a
// team whose project has no record of its own (teamsRecorded).
func latestMemberRecords() map[domainplace.Project]map[string]map[string]launchlog.Record {
	out := map[domainplace.Project]map[string]map[string]launchlog.Record{}
	log, err := launchlog.Open()
	if err != nil {
		return out
	}
	records, err := log.Read()
	if err != nil {
		return out
	}
	for _, record := range records {
		if record.Team.String() == "" || record.Definition == "" || record.IsHire() {
			continue
		}
		project := record.Project.Project()
		if out[project] == nil {
			out[project] = map[string]map[string]launchlog.Record{}
		}
		if out[project][record.Team.String()] == nil {
			out[project][record.Team.String()] = map[string]launchlog.Record{}
		}
		// The record is history, oldest first: the last record that carries a
		// member is that member's last prompt.
		for _, member := range record.Members {
			if member.Name.String() != "" {
				out[project][record.Team.String()][member.Name.String()] = record
			}
		}
	}
	return out
}

// teamsRecorded is one project's teams as the launch record holds them, found the
// way the record names the project: the same directory, symlinks and all. A
// project with no record of its own falls back to the records that name none
// (r-2e49).
func teamsRecorded(records map[domainplace.Project]map[string]map[string]launchlog.Record, project domainplace.Project) map[string]map[string]launchlog.Record {
	if teams, ok := records[project]; ok {
		return teams
	}
	return records[domainplace.Project{}]
}

// currentDocs is the rule documents of one current prompt, with the path a
// screen shows.
func currentDocs(cat *catalog.Catalog, built *prompt.Built) []deliver.Doc {
	var out []deliver.Doc
	add := func(doc source.Doc) {
		text, err := source.Disk{}.Read(doc)
		if err != nil {
			return
		}
		out = append(out, deliver.Doc{Path: doc.Path, Display: cat.Display(doc.Path), Text: string(text)})
	}
	if built.HasRole {
		add(built.Role)
	}
	for _, doc := range built.Read {
		add(doc)
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
	records := latestMemberRecords()
	log, logErr := launchlog.Open()
	for _, running := range readRunningTeamsIn(in.Views, in.Session, false) {
		judged := judgeOf(running, in.Session, in.Now)
		planTeam(&one, running, judged, byHerdrOf(judged), records, log, logErr)
	}
	return one
}
