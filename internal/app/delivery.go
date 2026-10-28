package app

import (
	"path/filepath"
	"time"

	"github.com/aq2bq/hergy/internal/activity"
	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/deliver"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/web"
)

// deliveryPlan is one reading of the running members: the drift the apply sheet
// reads, and every live member the request may name, whether it changed or not,
// so the API can tell "nothing to apply" from "not running".
type deliveryPlan struct {
	apply   web.ApplyList
	members map[string]plannedMember
}

type plannedMember struct {
	team    runningTeam
	start   definition.Start
	state   string
	changed bool
	changes []deliver.Change
	how     string
	whyNot  string
}

// canApply says whether hergy may apply the drift to this member now: a member
// with no drift of its own is not in the plan, a start needs no waiting, and a
// send or a restart needs a member that is not working (d-aacf).
func (m plannedMember) canApply() bool {
	switch m.how {
	case web.ApplyCannot:
		return false
	case web.ApplyStart:
		return true
	default:
		return sendable(m.state)
	}
}

// Apply is GET /api/apply: the drift between the saved definitions and the teams
// running from them, and how hergy would put each member right (d-36ef). It is
// read from the same plan as the delivery answer, so the band and the sheet
// cannot disagree about what changed.
func (a *activityRead) Apply() web.ApplyList { return a.plan(a.clock()).apply }

// ApplyCount is the light answer the band reads on every page.
func (a *activityRead) ApplyCount() int { return a.Apply().Count }

// plan is the delivery answer for one moment, or the one read less than five
// seconds ago.
func (a *activityRead) plan(now time.Time) deliveryPlan {
	a.deliveryMu.Lock()
	if !a.deliveryAt.IsZero() && now.Sub(a.deliveryAt) < herdrMemory {
		one := a.deliveryPlan
		a.deliveryMu.Unlock()
		return one
	}
	a.deliveryMu.Unlock()
	one := a.computeDelivery(now)
	a.deliveryMu.Lock()
	a.deliveryPlan, a.deliveryAt = one, now
	a.deliveryMu.Unlock()
	return one
}

// computeDelivery reads herdr, the launch record and the current definitions.
// herdr is read for the members' states only; what changed is the difference
// between the prompt in the record and the prompt the current definition builds.
func (a *activityRead) computeDelivery(now time.Time) deliveryPlan {
	one := deliveryPlan{
		apply:   web.ApplyList{Teams: []web.ApplyTeam{}, Herdr: web.ActivityNote{OK: true}},
		members: map[string]plannedMember{},
	}
	read := a.readHerdr(now)
	if read.err != nil {
		one.apply.Herdr = web.ActivityNote{OK: false, Text: read.err.Error()}
		return one
	}
	records := latestMemberRecords()
	log, logErr := launchlog.Open()
	for _, running := range a.teamsOf(read) {
		judged := judgeOf(running, read, now)
		a.planTeam(&one, running, judged, byHerdrOf(judged), records, log, logErr)
	}
	return one
}

// byHerdrOf is the judged members by the name herdr knows them by: the members
// an agent is live for.
func byHerdrOf(judged activity.Team) map[string]activity.Node {
	out := map[string]activity.Node{}
	for _, node := range judged.Nodes {
		if node.Herdr != "" {
			out[node.Herdr] = node
		}
	}
	return out
}

// planTeam adds one running team's drift to the plan: the members whose prompt,
// start or env differs, the members the definition has and no agent is live for,
// and the members the definition no longer has.
func (a *activityRead) planTeam(one *deliveryPlan, running runningTeam, judged activity.Team, byHerdr map[string]activity.Node, records map[string]map[string]map[string]launchlog.Record, log launchlog.Log, logErr error) {
	row := web.ApplyTeam{
		Team: running.Name, Definition: running.Definition,
		Project: running.Project, ProjectName: running.ProjectName,
		Notes: []string{}, Members: []web.ApplyMember{},
	}
	if running.Built == nil {
		// A definition that does not build has no member to apply: the row says
		// why in the words hergy status uses (2.2). The row is here only when
		// an agent of the team is live, because otherwise there is no drift a
		// person can act on.
		if len(judged.Nodes) > 0 || len(judged.Hired) > 0 {
			row.Error = running.Broken
			one.apply.Teams = append(one.apply.Teams, row)
		}
		return
	}
	recorded := teamsRecorded(records, running.Project)[running.Name]
	oldRecord := false
	for i := range running.Built.starts {
		start := running.Built.starts[i]
		node, live := byHerdr[start.Name]
		if !live {
			// The definition has the member and no agent of the team is live
			// for it: hergy starts it (d-36ef).
			one.members[start.Name] = plannedMember{
				team: running, start: start, state: "absent", changed: true, how: web.ApplyStart,
			}
			row.Members = append(row.Members, web.ApplyMember{
				Name: start.Name, Short: start.Short, Role: start.Role,
				State: "absent", How: web.ApplyStart, Changes: []deliver.Change{}, CanApply: true,
			})
			continue
		}
		record, hasRecord := recorded[start.Name]
		if hasRecord && !fingerprinted(record, start) {
			oldRecord = true
		}
		drift := a.memberDrift(running.Catalog, log, logErr, running, start, record, hasRecord)
		state := node.State.Slug()
		planned := plannedMember{
			team: running, start: start, state: state,
			changed: drift.any(), changes: drift.changes, how: drift.how(state),
		}
		if !planned.canApply() {
			planned.whyNot = drift.whyNot(start.Short)
		}
		one.members[start.Name] = planned
		if !drift.any() {
			continue
		}
		applied := web.ApplyMember{
			Name: start.Name, Short: start.Short, Role: start.Role,
			State: state, How: drift.how(state), Changes: drift.changes,
			CanApply: drift.canApply(state), ConversationLost: drift.how(state) == web.ApplyRestart,
		}
		if !applied.CanApply {
			applied.WhyNot = drift.whyNot(start.Short)
		}
		row.Members = append(row.Members, applied)
	}
	if newest, ok := newestRecord(recorded); ok {
		row.Notes = append(row.Notes, workerNotes(newest, running.Built)...)
	}
	if oldRecord {
		// The record was written before hergy kept the fingerprint: a profile
		// that changed since then cannot be told from one that did not, so the
		// sheet says so rather than claiming the member is current.
		row.Notes = append(row.Notes, i18n.T("apply.note_old_record"))
	}
	// A member the definition no longer has is still live: hergy cannot put it
	// right without stopping and starting the team (d-36ef). Only an agent the
	// launch record holds as this team's member is that; the others are hired
	// workers, which a stop leaves alone too but which are outside the apply.
	for _, hired := range judged.Hired {
		if _, was := recorded[hired.Herdr]; !was {
			continue
		}
		short := shortMember(hired.Herdr, running.Name)
		row.Members = append(row.Members, web.ApplyMember{
			Name: hired.Herdr, Short: short, State: hired.State.Slug(),
			How: web.ApplyCannot, Changes: []deliver.Change{}, CanApply: false,
			WhyNot: i18n.Tf("apply.refused_removed", short),
		})
	}
	if len(row.Members) > 0 {
		one.apply.Teams = append(one.apply.Teams, row)
		one.apply.Count += len(row.Members)
	}
}

// newestRecord is the newest launch among the records of one team's members: the
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
func workerNotes(record launchlog.Record, r *resolved) []string {
	if len(record.Workers) == 0 {
		return nil
	}
	before := map[string]launchlog.Worker{}
	for _, worker := range record.Workers {
		before[worker.From+"/"+worker.Purpose] = worker
	}
	var out []string
	for _, worker := range r.team.Definition.Graph.Workers {
		was, ok := before[worker.From+"/"+worker.Purpose]
		if !ok {
			continue
		}
		p, ok := r.profiles[worker.Profile]
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
	start   bool
	env     bool
	changes []deliver.Change
}

// any says whether hergy has anything to apply to the member.
func (d memberDrift) any() bool { return d.prompt || d.start || d.env }

// how is the way hergy would put the member right (d-3d30): an env change needs
// the pane to be made again, a changed start restarts the agent, and a changed
// prompt is sent again. A member that is working is not touched now, whatever
// the drift is (d-aacf).
func (d memberDrift) how(state string) string {
	switch {
	case d.env:
		return web.ApplyCannot
	case !sendable(state):
		return web.ApplyCannot
	case d.start:
		return web.ApplyRestart
	default:
		return web.ApplyResend
	}
}

// canApply says whether hergy may apply the member now.
func (d memberDrift) canApply(state string) bool { return d.how(state) != web.ApplyCannot }

// whyNot is the sentence the sheet writes beside a member it cannot apply.
func (d memberDrift) whyNot(short string) string {
	if d.env {
		return i18n.Tf("apply.refused_env", short)
	}
	return i18n.Tf("apply.refused_state", short)
}

// memberDrift reads one live member's drift: the prompt comparison, and the
// fingerprints the launch record keeps of the start and the env.
func (a *activityRead) memberDrift(cat *catalog.Catalog, log launchlog.Log, logErr error, running runningTeam, start definition.Start, record launchlog.Record, hasRecord bool) memberDrift {
	var drift memberDrift
	drift.prompt, drift.changes = a.memberChanges(cat, log, logErr, running, start, record, hasRecord)
	if recorded, ok := recordedMemberOf(record, start); ok && hasRecord {
		if recorded.Start != "" {
			drift.start = recorded.Start != launchlog.StartFingerprint(start.Kind, start.Args)
		}
		if recorded.Env != "" {
			drift.env = recorded.Env != launchlog.EnvFingerprint(start.Env)
		}
	}
	if drift.start {
		drift.changes = append(drift.changes, deliver.Change{Kind: "start", Text: i18n.T("apply.start_changed")})
	}
	if drift.env {
		drift.changes = append(drift.changes, deliver.Change{Kind: "env", Text: i18n.T("apply.env_changed")})
	}
	return drift
}

// recordedMemberOf is one member in the launch record, by herdr name first and
// then by short name.
func recordedMemberOf(record launchlog.Record, start definition.Start) (launchlog.Member, bool) {
	for i := range record.Members {
		if record.Members[i].Name == start.Name || record.Members[i].Short == start.Short {
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
func (a *activityRead) memberChanges(cat *catalog.Catalog, log launchlog.Log, logErr error, running runningTeam, start definition.Start, record launchlog.Record, hasRecord bool) (bool, []deliver.Change) {
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
	node, ok := running.Built.team.Definition.Graph.Member(start.Short)
	if !ok {
		return false, nil
	}
	run := prompt.Run{
		Team: running.Name, Executable: record.Executable, Base: record.Project, Lang: record.Lang,
		Reader: source.Disk{}, Sources: cat, Display: cat.Display,
	}
	if run.Executable == "" {
		run.Executable = executable()
	}
	if run.Base == "" {
		// The team's own project, not the one serve was started in: a team of
		// another listed project is built from that project (r-2e49).
		run.Base = running.Project
	}
	built, err := prompt.Member(running.Built.team.Definition, &node, run)
	if err != nil {
		return true, []deliver.Change{{Kind: "prompt", Text: i18n.T("delivery.prompt_changed")}}
	}
	if string(old) == built.Text {
		return false, nil
	}
	current := start
	current.Prompt, current.Segments = built.Text, built.Segments
	current.RoleDoc, current.HasRole, current.Read = built.Role, built.HasRole, built.Read
	changes := deliver.Compare(string(old), oldDocs(log, record), current, currentDocs(cat, built))
	if len(changes) == 0 {
		changes = []deliver.Change{{Kind: "prompt", Text: i18n.T("delivery.prompt_changed")}}
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
func latestMemberRecords() map[string]map[string]map[string]launchlog.Record {
	out := map[string]map[string]map[string]launchlog.Record{}
	log, err := launchlog.Open()
	if err != nil {
		return out
	}
	records, err := log.Read()
	if err != nil {
		return out
	}
	for _, record := range records {
		if record.Team == "" || record.Definition == "" || record.IsHire() {
			continue
		}
		project := ""
		if record.Project != "" {
			project = filepath.Clean(record.Project)
		}
		if out[project] == nil {
			out[project] = map[string]map[string]launchlog.Record{}
		}
		if out[project][record.Team] == nil {
			out[project][record.Team] = map[string]launchlog.Record{}
		}
		// The record is history, oldest first: the last record that carries a
		// member is that member's last prompt.
		for _, member := range record.Members {
			if member.Name != "" {
				out[project][record.Team][member.Name] = record
			}
		}
	}
	return out
}

// teamsRecorded is one project's teams as the launch record holds them, found the
// way the record names the project: the same directory, symlinks and all. A
// project with no record of its own falls back to the records that name none
// (r-2e49).
func teamsRecorded(records map[string]map[string]map[string]launchlog.Record, project string) map[string]map[string]launchlog.Record {
	if teams, ok := records[filepath.Clean(project)]; ok {
		return teams
	}
	for key, teams := range records {
		if key != "" && samePath(key, project) {
			return teams
		}
	}
	return records[""]
}

// oldDocs is the documents the launch record holds, by the path the record
// names. A missing blob simply leaves that document out.
func oldDocs(log launchlog.Log, record launchlog.Record) map[string]string {
	out := make(map[string]string, len(record.Docs))
	for _, doc := range record.Docs {
		text, err := log.Text(doc.SHA256)
		if err == nil {
			out[doc.Path] = string(text)
		}
	}
	return out
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

// InvalidateDelivery drops the delivery answer held for the band and the sheet.
// A save calls it from the one place every write passes.
func (a *activityRead) InvalidateDelivery() {
	a.deliveryMu.Lock()
	a.deliveryAt = time.Time{}
	a.deliveryMu.Unlock()
}
