package app

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/aq2bq/hergy/internal/activity"
	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/deliver"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
)

// deliveryPlan is one reading of the running members: the answer a screen gets,
// every live member with the state and changes a send needs, and the drift the
// apply sheet reads. A member that is not stale is in the map too, so the API can
// tell "nothing to send" from "not running".
type deliveryPlan struct {
	answer  deliver.Answer
	apply   web.ApplyList
	members map[string]plannedMember
}

type plannedMember struct {
	team    runningTeam
	start   definition.Start
	state   string
	stale   bool
	changes []deliver.Change
	how     string
}

// Delivery is GET /api/delivery: the running members whose prompt is not the one
// hergy last handed them, by running team, and the band's count. It is held for
// five seconds like herdr, and Deliver clears it.
func (a *activityRead) Delivery() deliver.Answer { return a.plan(a.clock()).answer }

// DeliveryCount is the light answer the band reads on every page.
func (a *activityRead) DeliveryCount() int { return a.Delivery().Count }

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
		answer:  deliver.Answer{Teams: []deliver.Team{}, Herdr: deliver.Note{OK: true}},
		apply:   web.ApplyList{Teams: []web.ApplyTeam{}, Herdr: web.ActivityNote{OK: true}},
		members: map[string]plannedMember{},
	}
	read := a.readHerdr(now)
	if read.err != nil {
		one.answer.Herdr = deliver.Note{OK: false, Text: read.err.Error()}
		one.apply.Herdr = web.ActivityNote{OK: false, Text: read.err.Error()}
		return one
	}
	records := latestMemberRecords()
	log, logErr := launchlog.Open()
	for _, running := range a.teamsOf(read) {
		judged := judgeOf(running, read, now)
		byHerdr := map[string]activity.Node{}
		for _, node := range judged.Nodes {
			if node.Herdr != "" {
				byHerdr[node.Herdr] = node
			}
		}
		row := web.ApplyTeam{
			Team: running.Name, Definition: running.Definition,
			Project: running.Project, ProjectName: running.ProjectName,
			Notes: []string{}, Members: []web.ApplyMember{},
		}
		if running.Built == nil {
			// A definition that does not build has no member to apply: the row
			// says why in the words hergy status uses (2.2). The row is here
			// only when an agent of the team is live, because otherwise there
			// is no drift a person can act on.
			if len(judged.Nodes) > 0 || len(judged.Hired) > 0 {
				row.Error = running.Broken
				one.apply.Teams = append(one.apply.Teams, row)
			}
			continue
		}
		recorded := teamsRecorded(records, running.Project)[running.Name]
		deliveryRow := deliver.Team{Name: running.Name, Definition: running.Definition, Members: []deliver.Member{}}
		for i := range running.Built.starts {
			start := running.Built.starts[i]
			node, live := byHerdr[start.Name]
			if !live {
				// The definition has the member and no agent of the team is
				// live for it: hergy starts it (d-36ef).
				row.Members = append(row.Members, web.ApplyMember{
					Name: start.Name, Short: start.Short, Role: start.Role,
					State: "absent", How: web.ApplyStart, Changes: []deliver.Change{}, CanApply: true,
				})
				continue
			}
			record, hasRecord := recorded[start.Name]
			stale, changes := a.memberChanges(running.Catalog, log, logErr, running, start, record, hasRecord)
			state := node.State.Slug()
			member := plannedMember{
				team: running, start: start, state: state, stale: stale, changes: changes, how: web.ApplyResend,
			}
			one.members[start.Name] = member
			if !stale {
				continue
			}
			deliveryRow.Members = append(deliveryRow.Members, deliver.Member{
				Name: start.Name, Short: start.Short, Role: start.Role,
				State: state, CanSend: sendable(state), Changes: changes,
			})
			applied := web.ApplyMember{
				Name: start.Name, Short: start.Short, Role: start.Role,
				State: state, How: web.ApplyResend, Changes: changes, CanApply: sendable(state),
			}
			if !applied.CanApply {
				applied.How = web.ApplyCannot
				applied.WhyNot = i18n.Tf("apply.refused_state", start.Short)
			}
			row.Members = append(row.Members, applied)
		}
		// A member the definition no longer has is still live: hergy cannot put
		// it right without stopping and starting the team (d-36ef). Only an
		// agent the launch record holds as this team's member is that; the
		// others are hired workers, which a stop leaves alone too but which are
		// outside the apply.
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
		if len(deliveryRow.Members) > 0 {
			one.answer.Teams = append(one.answer.Teams, deliveryRow)
			one.answer.Count += len(deliveryRow.Members)
		}
		if len(row.Members) > 0 {
			one.apply.Teams = append(one.apply.Teams, row)
			one.apply.Count += len(row.Members)
		}
	}
	return one
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
	var recorded *launchlog.Member
	for i := range record.Members {
		if record.Members[i].Name == start.Name || record.Members[i].Short == start.Short {
			recorded = &record.Members[i]
			break
		}
	}
	if recorded == nil {
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

// Deliver is POST /api/delivery: hand the current prompt to the selected
// members, one team at a time. It refuses the whole request if any selected
// member is not a running, stale, waiting member; nothing is sent then.
func (a *activityRead) Deliver(names []string) (deliver.Answer, error) {
	if len(names) == 0 {
		return deliver.Answer{}, errors.New(i18n.T("delivery.none_selected"))
	}
	now := a.clock()
	one := a.plan(now)
	if !one.answer.Herdr.OK {
		return deliver.Answer{}, errors.New(one.answer.Herdr.Text)
	}
	var refusals []deliver.Refusal
	groups := map[string][]plannedMember{}
	order := []string{}
	for _, name := range names {
		member, ok := one.members[name]
		if !ok {
			refusals = append(refusals, deliver.Refusal{Name: name, Text: i18n.Tf("delivery.refused_unknown", name)})
			continue
		}
		switch {
		case !member.stale:
			refusals = append(refusals, deliver.Refusal{Name: name, Text: i18n.Tf("delivery.no_change", name)})
		case !sendable(member.state):
			refusals = append(refusals, deliver.Refusal{Name: name, Text: i18n.Tf("delivery.refused_state", name)})
		default:
			if _, ok := groups[member.team.Name]; !ok {
				order = append(order, member.team.Name)
			}
			groups[member.team.Name] = append(groups[member.team.Name], member)
		}
	}
	if len(refusals) > 0 {
		return deliver.Answer{Refused: refusals}, refusalError(refusals)
	}
	for _, name := range order {
		group := groups[name]
		projectEnv := a.env
		if scoped, err := a.env.AtProject(group[0].team.Project); err == nil {
			projectEnv = scoped
		}
		r, err := buildRunning(projectEnv.Catalog(source.Disk{}), projectEnv.ProjectRoot, team.Running{Team: group[0].team.Name, Definition: group[0].team.Definition})
		if err != nil {
			return deliver.Answer{}, err
		}
		selected := make([]definition.Start, 0, len(group))
		for _, member := range group {
			found, err := r.member(member.start.Name)
			if err != nil {
				return deliver.Answer{}, err
			}
			selected = append(selected, *found)
		}
		if err := newLauncher(r, 30000, io.Discard).Reprompt(selected); err != nil {
			return deliver.Answer{}, err
		}
		log, err := launchlog.Open()
		if err != nil {
			return deliver.Answer{}, err
		}
		if err := writeLaunch(log, projectEnv, r, launchlog.CommandReprompt, selected, launchlog.Workspace{}); err != nil {
			return deliver.Answer{}, err
		}
	}
	a.InvalidateDelivery()
	answer := a.plan(a.clock()).answer
	answer.Sent = append([]string(nil), names...)
	return answer, nil
}

// InvalidateDelivery drops the delivery answer held for the band and the sheet.
// A save calls it from the one place every write passes.
func (a *activityRead) InvalidateDelivery() {
	a.deliveryMu.Lock()
	a.deliveryAt = time.Time{}
	a.deliveryMu.Unlock()
}

// refusalError is one sentence for the API: every refused member and why.
func refusalError(refusals []deliver.Refusal) error {
	lines := make([]string, 0, len(refusals))
	for _, refusal := range refusals {
		lines = append(lines, refusal.Text)
	}
	return fmt.Errorf("%s", strings.Join(lines, "\n"))
}
