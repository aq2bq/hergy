package check

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/prose"
	"github.com/aq2bq/hergy/internal/source"
)

// RoleInput is what one role document's check needs (C6): the catalog to read
// through, where the document stands, the draft a screen holds, and the name it
// is saved under.
type RoleInput struct {
	// Catalog is what the check reads through. A screen hands a catalog with
	// whatever else it is editing laid over it (C5, C3b); the role document
	// itself is named by Place and Name here, so the caller leaves it out.
	Catalog *catalog.Catalog
	// Place is the layer the document stands in: source.LayerLocal ("project")
	// or source.LayerGlobal ("global").
	Place string
	// Name is the role's name: the file's name without .yaml.
	Name string
	// Draft is the document as a screen has it before it is written: the role
	// tab's fields, read as a document. It is written back out with Emit to be
	// checked as if it were saved, and it draws the Markdown preview. When it is
	// empty, Text is the draft instead.
	// Text is the YAML the YAML tab is typing, read as the draft: a problem it
	// finds carries the YAML line, so the tab can mark the line. It is read when
	// Draft is empty; an empty Text is a document with no lines, which is the
	// error.file_empty a person deleting the whole file gets.
	Text string
	// Original is the name the document is saved under when the draft renames
	// it; empty means the name has not changed. It is where the saved file is
	// read for the diff, and the file the draft replaces.
	Original string
}

// RoleProblem is a document finding, optionally tied to a field, line or team.
type RoleProblem struct {
	Sev        string `json:"sev"`
	Field      string `json:"field"`
	Definition string `json:"definition"`
	Line       int    `json:"line"`
	Text       string `json:"text"`
}

// ProseLine is a physical line of the raw document.
type ProseLine struct {
	Text string `json:"text"`
	Part string `json:"part"`
}

// RoleMarkdown is the raw document preview with its character count.
type RoleMarkdown struct {
	Lines []ProseLine `json:"lines"`
	Chars int         `json:"chars"`
}

// RoleDiff is how the Markdown a screen shows changed from the saved document
// (C6): the indexes of the lines added, the indexes a removed saved line stood
// before, and whether a removed saved line stood after the last line.
type RoleDiff struct {
	Added         []int `json:"added"`
	RemovedBefore []int `json:"removedBefore"`
	RemovedAtEnd  bool  `json:"removedAtEnd"`
}

// RoleReader is one member that reads the document: the team definition it
// plays the role in and its name. It says who uses the role (a reason to take it
// to another project), not how much it is handed: the amount is what the prompt
// preview shows, and no team-dependent amount is part of this check (d-8bbf).
type RoleReader struct {
	Definition string `json:"definition"`
	Member     string `json:"member"`
}

// DocReport holds a document's problems, raw preview, changed lines and readers.
type DocReport struct {
	Problems []RoleProblem `json:"problems"`
	Markdown RoleMarkdown  `json:"markdown"`
	Diff     RoleDiff      `json:"diff"`
	Readers  []RoleReader  `json:"readers"`
	// Limit is the provisional size budget of this unit, in characters, for a
	// screen that shows the document's size against it (d-8bbf).
	Limit         int `json:"limit,omitempty"`
	TemplateChars int `json:"template_chars"`
}

// RoleReport is the old name of DocReport, kept so a caller that reads one role
// document keeps its name (spec-teams 7).
type RoleReport = DocReport

// Role checks one role document the way the role screen needs it (C6, d-a575):
// the document's own problems read to the end (C2), an id or a report name it
// repeats, the model names its rules write (C8), the size of the part of it that
// goes into the prompt (d-00a9), and the Markdown a member reads, with how that
// changed from the saved document.
//
// The answer does not depend on which team definitions use the role (d-8bbf): a
// rule that names a team's turn is the team definition's check (TurnFindings),
// and the documents one member reads are checked together there
// (betweenFindings). Readers carries only who uses the role, by name, apart from
// the check.
//
// The draft is checked as if it were saved: the document is read through a
// catalog that carries the draft in its place (C5), so the readers see it.
func Role(in RoleInput) DocReport {
	if in.Catalog == nil {
		return DocReport{}
	}
	text := in.Text
	report := proseReport(in.Catalog, rolePath(in.Catalog, in.Place, in.Name), text)
	report.Limit = RoleCharLimit
	if err := catalog.RoleNameProblem(in.Name); err != nil {
		report.Problems = append(report.Problems, RoleProblem{Sev: "err", Field: "name", Text: err.Error()})
	}
	report.Problems = append(report.Problems, roleSizeProblems((prose.Document{Text: text}).Chars())...)
	for _, def := range roleDefinitions(in.Catalog, in.Name) {
		report.Readers = append(report.Readers, roleReaders(def, in.Name)...)
	}
	return report
}

// rolePath is where one layer keeps a role document.
func rolePath(cat *catalog.Catalog, place, name string) string {
	return filepath.Join(cat.LayerDir(place), "roles", name+".md")
}

// roleDefinitions is every readable team definition that plays one role, once
// each, in the order a name is looked up (the project's own first).
func roleDefinitions(cat *catalog.Catalog, role string) []*definition.Definition {
	seen := map[string]bool{}
	var out []*definition.Definition
	for _, ref := range cat.RoleUsers(role) {
		if seen[ref.Definition] {
			continue
		}
		seen[ref.Definition] = true
		def, err := cat.Definition(ref.Definition, "")
		if err != nil {
			continue
		}
		out = append(out, def)
	}
	return out
}

// roleReaders is the members that read the document, by the definition they
// play it in and their name: who uses the role, not how much each is handed
// (d-8bbf).
func roleReaders(def *definition.Definition, role string) []RoleReader {
	var out []RoleReader
	for i := range def.Graph.Members {
		member := &def.Graph.Members[i]
		if member.Role != role {
			continue
		}
		out = append(out, RoleReader{Definition: def.Name, Member: def.Name + "-" + member.Name})
	}
	return out
}

// LineDiff is how draft differs from saved, by line: the same alignment the
// Markdown preview uses, exported so the team screen's YAML preview marks the
// lines one edit would change the same way (spec-teams B7).
func LineDiff(saved, draft []string) RoleDiff { return lineDiff(saved, draft) }

// lineDiff is how draft differs from saved, by line: the indexes of the draft
// lines added, the indexes a removed saved line stood before (or RemovedAtEnd,
// for the ones after the last), found by aligning the two. It is what the
// Markdown preview marks (spec-roles 1.2.6).
func lineDiff(saved, draft []string) RoleDiff {
	common := lcsLengths(saved, draft)
	var diff RoleDiff
	i, j := 0, 0
	added := map[int]bool{}
	removed := map[int]bool{}
	for i < len(saved) && j < len(draft) {
		switch {
		case saved[i] == draft[j]:
			i++
			j++
		case common[i+1][j] >= common[i][j+1]:
			// A saved line is gone before draft[j].
			if j < len(draft) {
				removed[j] = true
			} else {
				diff.RemovedAtEnd = true
			}
			i++
		default:
			added[j] = true
			j++
		}
	}
	for ; i < len(saved); i++ {
		diff.RemovedAtEnd = true
	}
	for ; j < len(draft); j++ {
		added[j] = true
	}
	// A removed saved line whose place a draft line took is marked on the first
	// line after the added run (spec-roles 1.2.6): the mock draws the strike
	// under the block that replaced it, not on the same line as the new text.
	for j := range draft {
		if !removed[j] || !added[j] {
			continue
		}
		n := j
		for n < len(draft) && added[n] {
			n++
		}
		delete(removed, j)
		if n < len(draft) {
			removed[n] = true
		} else {
			diff.RemovedAtEnd = true
		}
	}
	for j := range draft {
		if added[j] {
			diff.Added = append(diff.Added, j)
		}
		if removed[j] {
			diff.RemovedBefore = append(diff.RemovedBefore, j)
		}
	}
	return diff
}

// lcsLengths is the table of the longest common subsequence of two line lists:
// common[i][j] is the length for saved[i:] and draft[j:].
func lcsLengths(saved, draft []string) [][]int {
	common := make([][]int, len(saved)+1)
	for i := range common {
		common[i] = make([]int, len(draft)+1)
	}
	for i, s := range slices.Backward(saved) {
		for j := len(draft) - 1; j >= 0; j-- {
			switch {
			case s == draft[j]:
				common[i][j] = common[i+1][j+1] + 1
			case common[i+1][j] >= common[i][j+1]:
				common[i][j] = common[i+1][j]
			default:
				common[i][j] = common[i][j+1]
			}
		}
	}
	return common
}

// proseReport compares the exact saved and drafted lines without interpreting prose.
func proseReport(cat *catalog.Catalog, path, text string) DocReport {
	saved, _ := cat.Reader().Read(source.Doc{Path: path})
	var lines []ProseLine
	for line := range strings.SplitSeq(text, "\n") {
		lines = append(lines, ProseLine{Text: line, Part: "text"})
	}
	return DocReport{Markdown: RoleMarkdown{Lines: lines, Chars: (prose.Document{Text: text}).Chars()}, Diff: lineDiff(strings.Split(saved, "\n"), strings.Split(text, "\n"))}
}
