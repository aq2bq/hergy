package ops

import (
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/launcher"
	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/version"
)

// recordLaunch writes one launch to this machine's record (spec/11): what hergy
// handed the members it handed a prompt to, and which rule files those prompts
// were built from. The WebUI reads it to show what changed since the last launch
// and how the prompts differ.
//
// Members record only confirmed recipients. Successful operations and missing
// receipts use recordReceivedLaunch: up always keeps the team's identity, even
// with no recipient; refresh, reprompt and apply write only with a recipient.
// Startup failures, sending failures and cancellation keep no launch record.
// A hire has its own recordHire, independently of prompt receipt. A record that
// cannot be written is reported on stderr and does not decide the operation.
func recordLaunch(env Environment, r *Resolved, command string, members []definition.Start) string {
	return recordLaunchIn(env, r, command, members, launchlog.Workspace{})
}

// recordReceivedLaunch owns which confirmed recipients enter a launch and when
// an empty launch is kept (d-8e2f). The team created by up must remain resolvable
// by its alias even when nobody received a prompt; other operations create no
// team and keep no new record when nobody received one.
func recordReceivedLaunch(env Environment, r *Resolved, command string, members []definition.Start, l *launcher.Launcher, workspace launchlog.Workspace, now func() time.Time) string {
	var received []definition.Start
	for _, member := range members {
		if l.Received(member.Name.String()) {
			received = append(received, member)
		}
	}
	if command != launchlog.CommandUp && len(received) == 0 {
		return ""
	}
	stateNote := runningStateWarning(receiveRunningState(r, command, received, now, l.Started, l.CreatedPane))
	if note := recordLaunchIn(env, r, command, received, workspace); note != "" {
		return note
	}
	return stateNote
}

// recordLaunchIn writes one launch, naming the workspace its tab was made in
// (d-73fc). A launch that makes no tab passes the zero workspace.
func recordLaunchIn(env Environment, r *Resolved, command string, members []definition.Start, workspace launchlog.Workspace) string {
	log, err := launchlog.Open()
	if err == nil {
		err = writeLaunch(log, env, r, command, members, workspace)
	}
	if err != nil {
		return i18n.Tf("launch.not_recorded", err)
	}
	return ""
}

// recordHire writes the one line a hire leaves in this machine's record: which
// worker it added, which member added it, the purpose and the tab (d-b4b9). The
// activity screen reads it to say "<member> が追加". A line that cannot be
// written is one line on stderr and nothing else: the worker is already up.
func recordHire(env Environment, r *Resolved, worker definition.Start, now func() time.Time) string {
	stateNote := runningStateWarning(hireRunningState(r, worker, now))
	log, err := launchlog.Open()
	if err == nil {
		workerName, _ := domainname.ParseWorkerName(worker.Short.String())
		err = log.Append(launchlog.Record{
			Time:       time.Now().Format(time.RFC3339),
			Command:    launchlog.CommandHire,
			Hergy:      version.Line(),
			Project:    env.ProjectRoot.Seen(),
			Layer:      domainplace.Selection(r.Team.Definition.Layer),
			Definition: r.Team.Definition.Name.String(),
			Team:       r.Team.Name.Observation(),
			Hire: &launchlog.Hire{
				Worker:  worker.Name.Observation(),
				From:    worker.From.String(),
				Purpose: worker.Purpose,
				Tab:     r.ModelTeamName().WorkerTab(workerName),
			},
		})
	}
	if err != nil {
		return i18n.Tf("launch.not_recorded", err)
	}
	return stateNote
}

// writeLaunch builds one record and appends it. The texts the record points at —
// prompt bodies before change-reason preambles and the files read — go into blobs/ first.
func writeLaunch(log launchlog.Log, env Environment, r *Resolved, command string, members []definition.Start, workspace launchlog.Workspace) error {
	material, err := teamMaterial(r)
	if err != nil {
		return err
	}
	rec := launchlog.Record{
		Time:        time.Now().Format(time.RFC3339),
		Command:     command,
		Hergy:       version.Line(),
		Project:     env.ProjectRoot.Seen(),
		Layer:       domainplace.Selection(r.Team.Definition.Layer),
		Definition:  r.Team.Definition.Name.String(),
		Team:        r.Team.Name.Observation(),
		TeamYAML:    r.Team.Definition.TeamYAML.Project().Seen(),
		Workspace:   workspace.Label,
		WorkspaceID: workspace.ID,
		Hash:        launchlog.Hash(material),
		// The facts the prompts were built with, so a screen can rebuild them
		// the way the launch did (C9, spec/43 S1).
		Executable: executable(),
		Lang:       i18n.Language(),
	}
	// A worker's profile is not a member's: it reaches the next worker hired,
	// so the record keeps its fingerprint for the apply sheet's note (d-36ef).
	for _, worker := range r.Team.Definition.Graph.Workers {
		p, ok := r.Profiles[worker.Profile]
		if !ok {
			continue
		}
		rec.Workers = append(rec.Workers, launchlog.Worker{
			From: worker.From, Purpose: worker.Purpose.String(), Profile: worker.Profile,
			Start: launchlog.StartFingerprint(p.Kind, p.Args),
			Env:   launchlog.EnvFingerprint(p.Env),
		})
	}
	// A document two members read is one entry, and one blob: the record names
	// the files, not the times they were read.
	seen := map[string]bool{}
	for i := range members {
		member := &members[i]
		hash, err := log.Put([]byte(member.Prompt))
		if err != nil {
			return err
		}
		paths := make([]string, 0, len(docsReadBy(member)))
		for _, doc := range docsReadBy(member) {
			paths = append(paths, doc.Path)
		}
		rec.Members = append(rec.Members, launchlog.Member{
			Docs:   paths,
			Name:   member.Name.Observation(),
			Short:  member.Short.String(),
			Prompt: hash,
			Start:  launchlog.StartFingerprint(member.Kind, member.Args),
			Env:    launchlog.EnvFingerprint(member.Env),
		})
		for _, doc := range docsReadBy(member) {
			if seen[doc.Path] {
				continue
			}
			seen[doc.Path] = true
			entry, err := docEntry(log, doc)
			if err != nil {
				return err
			}
			rec.Docs = append(rec.Docs, entry)
		}
	}
	return log.Append(rec)
}

// teamMaterial is the team definition as it was launched: the settings file, and
// every member of the definition after its profile is resolved. The hash is over
// this and not over the members one command handed a prompt to, so the same team
// definition hashes the same across up, refresh and reprompt, and a change to a
// profile no member reads does not change it.
func teamMaterial(r *Resolved) (launchlog.Material, error) {
	def := r.Team.Definition
	teamYAML, err := source.Disk{}.Read(source.Doc{Layer: def.Layer, Path: def.TeamYAML.String()})
	if err != nil {
		return launchlog.Material{}, err
	}
	material := launchlog.Material{
		TeamYAML: []byte(teamYAML),
		Members:  make([]launchlog.MaterialMember, 0, len(r.Starts)),
	}
	for i := range r.Starts {
		member := &r.Starts[i]
		material.Members = append(material.Members, launchlog.MaterialMember{
			Kind:   member.Kind,
			Args:   member.Args,
			Env:    member.Env,
			Prompt: member.Prompt,
		})
	}
	return material, nil
}

// docsReadBy is every rule file one member's prompt was built from: the role
// file first, then the documents, in the order the prompt read them.
func docsReadBy(member *definition.Start) []source.Doc {
	var docs []source.Doc
	if member.HasRole {
		docs = append(docs, member.RoleDoc)
	}
	return append(docs, member.Read...)
}

// docEntry is one rule file as the record holds it: where it is, which layer
// supplied it, its own revision when it has one, and the hash of its text, which
// is in blobs/. A file that does not read as a rule file — the record is taken
// from what was read, not only from what this hergy can parse — is recorded
// without a revision.
func docEntry(log launchlog.Log, doc source.Doc) (launchlog.Doc, error) {
	text, err := source.Disk{}.Read(doc)
	if err != nil {
		return launchlog.Doc{}, err
	}
	hash, err := log.Put([]byte(text))
	if err != nil {
		return launchlog.Doc{}, err
	}
	entry := launchlog.Doc{Path: doc.Path, Layer: doc.Layer, SHA256: hash}

	return entry, nil
}
