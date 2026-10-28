package app

import (
	"fmt"
	"io"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/version"
)

// recordLaunch writes one launch to this machine's record (spec/11): what hergy
// handed the members it handed a prompt to, and which rule files those prompts
// were built from. The WebUI reads it to show what changed since the last launch
// and how the prompts differ.
//
// It runs after a launch has succeeded (up, refresh and reprompt; not --dry and
// not a launch that failed). A hire writes its own line (recordHire): a worker
// is not a member of the team definition, so it has no prompt to point at. A
// record that cannot be written is one line on stderr and nothing else: the
// team is already up, so the command still succeeds.
func recordLaunch(stderr io.Writer, env team.Env, r *resolved, command string, members []definition.Start) {
	recordLaunchIn(stderr, env, r, command, members, launchlog.Workspace{})
}

// recordLaunchIn writes one launch, naming the workspace its tab was made in
// (d-73fc). A launch that makes no tab passes the zero workspace.
func recordLaunchIn(stderr io.Writer, env team.Env, r *resolved, command string, members []definition.Start, workspace launchlog.Workspace) {
	log, err := launchlog.Open()
	if err == nil {
		err = writeLaunch(log, env, r, command, members, workspace)
	}
	if err != nil {
		fmt.Fprintln(stderr, i18n.Tf("launch.not_recorded", err))
	}
}

// recordHire writes the one line a hire leaves in this machine's record: which
// worker it added, which member added it, the purpose and the tab (d-b4b9). The
// activity screen reads it to say "<member> が追加". A line that cannot be
// written is one line on stderr and nothing else: the worker is already up.
func recordHire(stderr io.Writer, env team.Env, r *resolved, worker definition.Start) {
	log, err := launchlog.Open()
	if err == nil {
		err = log.Append(launchlog.Record{
			Time:       time.Now().Format(time.RFC3339),
			Command:    launchlog.CommandHire,
			Hergy:      version.Line(),
			Project:    env.ProjectRoot,
			Layer:      r.team.Definition.Layer,
			Definition: r.team.Definition.Name,
			Team:       r.team.Name,
			Hire: &launchlog.Hire{
				Worker:  worker.Name,
				From:    worker.From,
				Purpose: worker.Purpose,
				Tab:     protocol.WorkerTabPrefix(r.team.Name) + worker.Short,
			},
		})
	}
	if err != nil {
		fmt.Fprintln(stderr, i18n.Tf("launch.not_recorded", err))
	}
}

// writeLaunch builds one record and appends it. The texts the record points at —
// the prompts it handed out and the rule files it read — go into blobs/ first.
func writeLaunch(log launchlog.Log, env team.Env, r *resolved, command string, members []definition.Start, workspace launchlog.Workspace) error {
	material, err := teamMaterial(r)
	if err != nil {
		return err
	}
	rec := launchlog.Record{
		Time:        time.Now().Format(time.RFC3339),
		Command:     command,
		Hergy:       version.Line(),
		Project:     env.ProjectRoot,
		Layer:       r.team.Definition.Layer,
		Definition:  r.team.Definition.Name,
		Team:        r.team.Name,
		TeamYAML:    r.team.Definition.TeamYAML,
		Workspace:   workspace.Label,
		WorkspaceID: workspace.ID,
		Hash:        launchlog.Hash(material),
		// The facts the prompts were built with, so a screen can rebuild them
		// the way the launch did (C9, spec/43 S1).
		Executable: executable(),
		Lang:       i18n.Language(),
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
		rec.Members = append(rec.Members, launchlog.Member{
			Name:   member.Name,
			Short:  member.Short,
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
func teamMaterial(r *resolved) (launchlog.Material, error) {
	def := r.team.Definition
	teamYAML, err := source.Disk{}.Read(source.Doc{Layer: def.Layer, Path: def.TeamYAML})
	if err != nil {
		return launchlog.Material{}, err
	}
	material := launchlog.Material{
		TeamYAML: []byte(teamYAML),
		Members:  make([]launchlog.MaterialMember, 0, len(r.starts)),
	}
	for i := range r.starts {
		member := &r.starts[i]
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
	if parsed, err := rules.Parse(text, doc.Path); err == nil && parsed.Revision != nil {
		entry.Revision = *parsed.Revision
	}
	return entry, nil
}
