package app

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/migrate"
)

// oldDocs uses the previous member's reading, never the current role assignment.
// Old records lack this list: their generated identity identifies the role,
// while custom templates can be matched against the historical rendered text.
func oldDocs(log launchlog.Log, record launchlog.Record, member launchlog.Member, oldPrompt string) map[string]string {
	out := make(map[string]string)
	role := previousRole(record, member, oldPrompt)
	for _, doc := range record.Docs {
		if member.Docs != nil && !slices.Contains(member.Docs, doc.Path) {
			continue
		}
		text, err := log.Text(doc.SHA256)
		if err != nil {
			continue
		}
		if member.Docs == nil && !previouslyRead(doc.Path, string(text), role, oldPrompt) {
			continue
		}
		out[doc.Path] = string(text)
	}
	return out
}

func previousRole(record launchlog.Record, member launchlog.Member, oldPrompt string) string {
	header, _, _ := strings.Cut(i18n.Text(record.Lang, "prompt.member"), "\n")
	for _, doc := range record.Docs {
		if filepath.Base(filepath.Dir(doc.Path)) != "roles" {
			continue
		}
		role := strings.TrimSuffix(filepath.Base(doc.Path), filepath.Ext(doc.Path))
		line := strings.NewReplacer("%{team}", record.Team.String(), "%{name}", member.Name.String(), "%{role}", role).Replace(header)
		if strings.HasPrefix(oldPrompt, line+"\n") || oldPrompt == line {
			return role
		}
	}
	return ""
}

func previouslyRead(path, text, role, oldPrompt string) bool {
	if filepath.Base(filepath.Dir(path)) == "roles" && role != "" {
		return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) == role
	}
	if filepath.Ext(path) == ".yaml" {
		rendered, err := migrate.LegacyMarkdown(text, path)
		if err != nil {
			return false
		}
		text = rendered
	}
	// An empty document contributes no text and cannot be recovered from an old
	// prompt. New records retain its membership explicitly.
	return text != "" && strings.Contains(oldPrompt, text)
}
