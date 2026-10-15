package app

import (
	"fmt"
	"io"
	"time"

	"github.com/aq2bq/hergy/internal/config"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
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
// It runs after a launch has succeeded (up, refresh and reprompt; not --dry, not
// a launch that failed, and not hire, because a worker is not a member of the
// team definition). A record that cannot be written is one line on stderr and
// nothing else: the team is already up, so the command still succeeds.
func recordLaunch(stderr io.Writer, env team.Env, cfg *config.Config, command string, members []*config.Member) {
	log, err := launchlog.Open()
	if err == nil {
		err = writeLaunch(log, env, cfg, command, members)
	}
	if err != nil {
		fmt.Fprintln(stderr, i18n.Tf("launch.not_recorded", err))
	}
}

// writeLaunch builds one record and appends it. The texts the record points at —
// the prompts it handed out and the rule files it read — go into blobs/ first.
func writeLaunch(log launchlog.Log, env team.Env, cfg *config.Config, command string, members []*config.Member) error {
	material, err := teamMaterial(cfg)
	if err != nil {
		return err
	}
	rec := launchlog.Record{
		Time:       time.Now().Format(time.RFC3339),
		Command:    command,
		Hergy:      version.Line(),
		Project:    env.ProjectRoot,
		Layer:      cfg.Layer,
		Definition: cfg.Definition,
		Team:       cfg.Team,
		TeamYAML:   cfg.TeamYAML.Path,
		Hash:       launchlog.Hash(material),
	}
	// A document two members read is one entry, and one blob: the record names
	// the files, not the times they were read.
	seen := map[string]bool{}
	for _, member := range members {
		hash, err := log.Put([]byte(member.Prompt))
		if err != nil {
			return err
		}
		rec.Members = append(rec.Members, launchlog.Member{
			Name:   member.Name,
			Short:  member.ShortName,
			Prompt: hash,
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

// definitionMembers is every member of a definition, in the written order: who
// up hands a prompt to.
func definitionMembers(cfg *config.Config) []*config.Member {
	members := make([]*config.Member, len(cfg.Members))
	for i := range cfg.Members {
		members[i] = &cfg.Members[i]
	}
	return members
}

// teamMaterial is the team definition as it was launched: the settings file, and
// every member of the definition after its profile is resolved. The hash is over
// this and not over the members one command handed a prompt to, so the same team
// definition hashes the same across up, refresh and reprompt, and a change to a
// profile no member reads does not change it.
func teamMaterial(cfg *config.Config) (launchlog.Material, error) {
	teamYAML, err := source.Disk{}.Read(source.Doc{Layer: cfg.Layer, Path: cfg.TeamYAML.Path})
	if err != nil {
		return launchlog.Material{}, err
	}
	material := launchlog.Material{
		TeamYAML: []byte(teamYAML),
		Members:  make([]launchlog.MaterialMember, 0, len(cfg.Members)),
	}
	for i := range cfg.Members {
		member := &cfg.Members[i]
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
func docsReadBy(member *config.Member) []source.Doc {
	var docs []source.Doc
	if member.HasRoleSource {
		docs = append(docs, member.RoleSource)
	}
	return append(docs, member.DocsRead...)
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
