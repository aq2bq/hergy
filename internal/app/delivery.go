package app

import (
	"errors"
	"fmt"
	"io"
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
)

// deliveryPlan is one reading of the running members: the answer a screen gets,
// and every live member with the state and changes a send needs. A member that
// is not stale is in the map too, so the API can tell "nothing to send" from
// "not running".
type deliveryPlan struct {
	answer  deliver.Answer
	members map[string]plannedMember
}

type plannedMember struct {
	team    runningTeam
	start   definition.Start
	state   string
	stale   bool
	changes []deliver.Change
}

// Delivery is GET /api/delivery: the running members whose prompt is not the one
// hergy last handed them, by running team, and the band's count. It is held for
// five seconds like herdr, and Deliver clears it.
func (a *activityRead) Delivery() deliver.Answer { return a.plan(a.clock()).answer }

// DeliveryCount is the light answer the band reads on every page.
func (a *activityRead) DeliveryCount() int { return a.Delivery().Count }

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
		members: map[string]plannedMember{},
	}
	read := a.readHerdr(now)
	if read.err != nil {
		one.answer.Herdr = deliver.Note{OK: false, Text: read.err.Error()}
		return one
	}
	records := latestMemberRecords(a.env)
	cat := a.env.Catalog(source.Disk{})
	log, logErr := launchlog.Open()
	for _, running := range a.teamsOf(read) {
		if running.Built == nil {
			continue
		}
		judged := judgeOf(running, read, now)
		byHerdr := map[string]activity.Node{}
		for _, node := range judged.Nodes {
			if node.Herdr != "" {
				byHerdr[node.Herdr] = node
			}
		}
		recorded := records[running.Name]
		row := deliver.Team{Name: running.Name, Definition: running.Definition, Members: []deliver.Member{}}
		for i := range running.Built.starts {
			start := running.Built.starts[i]
			node, live := byHerdr[start.Name]
			if !live {
				continue
			}
			record, hasRecord := recorded[start.Name]
			stale, changes := a.memberChanges(cat, log, logErr, running, start, record, hasRecord)
			state := node.State.Slug()
			one.members[start.Name] = plannedMember{
				team: running, start: start, state: state, stale: stale, changes: changes,
			}
			if !stale {
				continue
			}
			row.Members = append(row.Members, deliver.Member{
				Name: start.Name, Short: start.Short, Role: start.Role,
				State: state, CanSend: sendable(state), Changes: changes,
			})
		}
		if len(row.Members) > 0 {
			one.answer.Teams = append(one.answer.Teams, row)
			one.answer.Count += len(row.Members)
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
		run.Base = a.env.ProjectRoot
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

// latestMemberRecords is, for each team and each member, the newest launch that
// handed that member a prompt. A reprompt records only the members it sent, so a
// team's newest line is not a snapshot of the others: each member's own last
// prompt is the one to compare.
func latestMemberRecords(env team.Env) map[string]map[string]launchlog.Record {
	out := map[string]map[string]launchlog.Record{}
	log, err := launchlog.Open()
	if err != nil {
		return out
	}
	records, err := log.Read()
	if err != nil {
		return out
	}
	for _, record := range records {
		if record.Team == "" || record.Definition == "" {
			continue
		}
		if record.Project != "" && !samePath(record.Project, env.ProjectRoot) {
			continue
		}
		if out[record.Team] == nil {
			out[record.Team] = map[string]launchlog.Record{}
		}
		// The record is history, oldest first: the last record that carries a
		// member is that member's last prompt.
		for _, member := range record.Members {
			if member.Name != "" {
				out[record.Team][member.Name] = record
			}
		}
	}
	return out
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
		r, err := buildRunning(a.env.Catalog(source.Disk{}), a.env.ProjectRoot, team.Running{Team: group[0].team.Name, Definition: group[0].team.Definition})
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
		if err := writeLaunch(log, a.env, r, launchlog.CommandReprompt, selected); err != nil {
			return deliver.Answer{}, err
		}
	}
	a.deliveryMu.Lock()
	a.deliveryAt = time.Time{}
	a.deliveryMu.Unlock()
	answer := a.plan(a.clock()).answer
	answer.Sent = append([]string(nil), names...)
	return answer, nil
}

// refusalError is one sentence for the API: every refused member and why.
func refusalError(refusals []deliver.Refusal) error {
	lines := make([]string, 0, len(refusals))
	for _, refusal := range refusals {
		lines = append(lines, refusal.Text)
	}
	return fmt.Errorf("%s", strings.Join(lines, "\n"))
}
