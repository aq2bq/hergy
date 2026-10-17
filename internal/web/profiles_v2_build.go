package web

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/aq2bq/hergy/internal/profile"
)

// This file builds the three tabs' parts out of one reading (profiles_v2_view.go).

// p2ProfilesData builds the profile tab: the filter, the search, the open rows
// and the list.
func (s *Server) p2ProfilesData(data p2Data, project, shared p2File, defs []p2Def, q url.Values) p2Data {
	chips := p2Chips(defs)
	names := p2Names(project, shared)
	data.ProfileCount = len(names)
	data.Kinds = p2KindChips(project, shared)
	data.KindFilter = q.Get("kind")
	data.Query = q.Get("q")
	for i := range data.Kinds {
		data.Kinds[i].On = data.Kinds[i].Name == data.KindFilter
	}
	data.Empty = len(names) == 0
	open := p2OpenSet(q.Get("open"))
	for _, name := range names {
		data.Rows = append(data.Rows, p2RowOf(name, project, shared, chips, open[name]))
	}
	return data
}

// p2OpenSet is the open= names of the URL.
func p2OpenSet(value string) map[string]bool {
	out := map[string]bool{}
	for name := range strings.SplitSeq(value, ",") {
		if name != "" {
			out[name] = true
		}
	}
	return out
}

// p2KindChips is the toolbar's chips: すべて and every kind a profile runs, in
// the toolbar's order, with the number of profiles that run it.
func p2KindChips(project, shared p2File) []p2Kind {
	kinds := map[string]int{}
	total := 0
	for _, entry := range p2Entries(project, shared) {
		total++
		kinds[entry.Kind]++
	}
	out := []p2Kind{{Count: total}}
	for _, kind := range p2KindList(kinds) {
		out = append(out, p2Kind{Name: kind, Count: kinds[kind], KindClass: p2KindClass(kind)})
	}
	return out
}

// p2KindList is every kind that has a profile, in the toolbar's order.
func p2KindList(counts map[string]int) []string {
	var out []string
	for _, kind := range []string{"claude", "codex", "opencode", "gemini"} {
		if counts[kind] > 0 {
			out = append(out, kind)
		}
	}
	for _, kind := range profile.Kinds() {
		if counts[kind] == 0 || p2KindClass(kind) != "" {
			continue
		}
		out = append(out, kind)
	}
	return out
}

// p2Effective is every name with the entry that is in effect: the project's own
// when it has one, the shared one otherwise.
func p2Entries(project, shared p2File) []profile.Entry {
	out := append([]profile.Entry{}, project.entries...)
	for _, entry := range shared.entries {
		if _, ok := p2Find(project, entry.Name); !ok {
			out = append(out, entry)
		}
	}
	return out
}

// p2Row is one row of the profile list, with its detail when it is open.
func p2RowOf(name string, project, shared p2File, chips map[string][]p2UserChip, open bool) p2Row {
	projectEntry, hasProject := p2Find(project, name)
	sharedEntry, hasShared := p2Find(shared, name)
	effective := projectEntry
	if !hasProject {
		effective = sharedEntry
	}
	row := p2Row{Name: name, Kind: effective.Kind, KindClass: p2KindClass(effective.Kind), Open: open}
	if hasProject {
		row.Places = append(row.Places, p2Place{Label: p2Label(p2Project), State: "on"})
	}
	if hasShared {
		state := "on"
		if hasProject {
			state = "shadow"
		}
		row.Places = append(row.Places, p2Place{Label: p2Label(p2Shared), State: state})
	}
	row.Args = strings.Join(effective.Args, " ")
	for i, pair := range effective.Env {
		if i < 2 {
			row.Env = append(row.Env, pair.Key+"="+pair.Value)
		} else {
			row.More = len(effective.Env) - 2
		}
	}
	users := p2Users(chips, name)
	row.Users, row.Roles, row.Chips = len(users), p2Roles(users), users
	var problems []profile.Problem
	if hasProject {
		problems = append(problems, projectEntry.Problems...)
	}
	if hasShared {
		problems = append(problems, sharedEntry.Problems...)
	}
	row.Err, row.Warn = p2Counts(problems)
	row.Open = open
	projectReaders, sharedReaders := p2Readers(users, hasProject)
	row.Panels = p2Panels(projectEntry, hasProject, sharedEntry, hasShared, len(users), projectReaders, sharedReaders, project, shared)
	return row
}

// p2Panels is one row's open panels: the project's file on the left, the shared
// one on the right, either of them alone when only one holds the profile.
func p2Panels(projectEntry profile.Entry, hasProject bool, sharedEntry profile.Entry, hasShared bool, users, projectReaders, sharedReaders int, project, shared p2File) []p2Panel {
	both := hasProject && hasShared
	var out []p2Panel
	if hasProject {
		shown := users
		if hasShared {
			shown = 0
		}
		out = append(out, p2PanelOf(p2Project, projectEntry, both, false, shown, projectReaders, project.hash, project.display))
	}
	if hasShared {
		shown := users
		if hasProject {
			shown = 0
		}
		out = append(out, p2PanelOf(p2Shared, sharedEntry, both, hasProject, shown, sharedReaders, shared.hash, shared.display))
	}
	return out
}

// p2Readers is how many people read one name in each file: the project's own
// definitions read the project's file, and the definitions outside the project
// read it too only when the project's file does not hold the name (R1).
func p2Readers(users []p2UserChip, hasProject bool) (project, shared int) {
	for _, chip := range users {
		if chip.Local {
			project++
		} else {
			shared++
		}
	}
	if !hasProject {
		shared = len(users)
	}
	return project, shared
}

// p2PanelOf is one file's panel. shadowed says the project's file hides this one;
// users is the number of people shown on the panel, which only a file that is
// the only one holding the name shows.
func p2PanelOf(file string, entry profile.Entry, both, shadowed bool, users, renameUsers int, hash, path string) p2Panel {
	errs, warns := p2Counts(entry.Problems)
	return p2Panel{
		File: file, Label: p2Label(file), Path: path, Saved: true, Both: both, Shadowed: shadowed,
		Other:       p2Other(file),
		OtherLabel:  p2Label(p2Other(file)),
		Name:        entry.Name,
		Kind:        entry.Kind,
		KindClass:   p2KindClass(entry.Kind),
		Args:        entry.Args,
		Env:         entry.Env,
		ArgsJSON:    p2JSON(entry.Args),
		EnvJSON:     p2JSON(p2EnvPairs(entry.Env)),
		SavedName:   entry.Name,
		SavedKind:   entry.Kind,
		SavedArgs:   p2JSON(entry.Args),
		SavedEnv:    p2JSON(p2EnvPairs(entry.Env)),
		Hash:        hash,
		Problems:    entry.Problems,
		Err:         errs,
		Warn:        warns,
		Users:       users,
		RenameUsers: renameUsers,
	}
}

// p2Other is the other place a profile may stand.
func p2Other(file string) string {
	if file == p2Shared {
		return p2Project
	}
	return p2Shared
}

// p2YamlData builds the YAML tab: both files, their lines and their problems.
func (s *Server) p2YamlData(data p2Data, project, shared p2File) p2Data {
	for _, file := range []p2File{project, shared} {
		data.Files = append(data.Files, s.p2Yed(file))
	}
	return data
}

// p2Yed is one file's section of the YAML tab.
func (s *Server) p2Yed(file p2File) p2Yed {
	text := file.text
	missing := !file.ok
	if missing {
		text = "version: 1\n"
	}
	_, problems := profile.ReadAll(text, s.cat.Display(file.path))
	out := p2Yed{
		File: file.file, Label: file.label, Path: s.cat.Display(file.path),
		Text: text, Hash: file.hash, Missing: missing,
	}
	out.Err, out.Warn = p2Counts(problems)
	marks := map[int]string{}
	for _, problem := range problems {
		if marks[problem.Line] != profile.Err.Word() {
			marks[problem.Line] = problem.Sev.Word()
		}
	}
	for i := 1; i <= len(strings.Split(text, "\n")); i++ {
		out.Lines = append(out.Lines, p2Line{N: i, Mark: marks[i]})
	}
	out.Height = fmt.Sprintf("h%d", clamp(len(strings.Split(text, "\n")), 6, 28))
	for _, problem := range p2Sorted(problems) {
		out.Problems = append(out.Problems, p2YProblem{Sev: problem.Sev.Word(), Line: problem.Line, Text: problem.Text})
	}
	return out
}

// clamp holds a number between two ends.
func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

// p2AssignData builds the assign tab: the kind bar and one card per definition.
func (s *Server) p2AssignData(data p2Data, project, shared p2File, defs []p2Def, q url.Values) p2Data {
	view := &p2Assign{Empty: len(defs) == 0}
	counts := map[string]int{}
	for _, def := range defs {
		team := s.p2Team(def, project, shared)
		for _, row := range team.Rows {
			if row.KindOf != "" {
				counts[row.KindOf]++
			}
		}
		view.Teams = append(view.Teams, team)
	}
	kindHi := q.Get("kind")
	for _, kind := range p2KindList(counts) {
		bar := p2KindBar{Name: kind, Count: counts[kind], Grow: fmt.Sprintf("g%d", clamp(counts[kind], 1, 20)), KindClass: p2KindClass(kind)}
		bar.On = kindHi != "" && kind == kindHi
		bar.Dim = kindHi != "" && !bar.On
		view.Kinds = append(view.Kinds, bar)
	}
	for i := range view.Teams {
		for j := range view.Teams[i].Rows {
			row := &view.Teams[i].Rows[j]
			row.Dim = kindHi != "" && row.KindOf != kindHi
		}
	}
	data.Assign = view
	return data
}

// p2Team is one team definition's card on the assign tab.
func (s *Server) p2Team(def p2Def, project, shared p2File) p2Team {
	options, kinds := p2Resolved(def, project, shared)
	out := p2Team{Name: def.name, Path: def.display, Members: len(def.members), Workers: len(def.workers), Hash: def.hash}
	for i, member := range def.members {
		row := p2ARow{Kind: "m", ID: fmt.Sprintf("m-%d", i), Name: member.Name, Role: member.Role, Profile: member.Profile}
		p2FillRow(&row, fmt.Sprintf("members[%d]", i), options, kinds)
		out.Rows = append(out.Rows, row)
	}
	for i, worker := range def.workers {
		row := p2ARow{Kind: "w", ID: fmt.Sprintf("w-%d", i), Name: worker.Purpose, Role: worker.From, Profile: worker.Profile}
		p2FillRow(&row, fmt.Sprintf("workers[%d]", i), options, kinds)
		out.Rows = append(out.Rows, row)
	}
	for _, row := range out.Rows {
		if row.Missing {
			out.Err++
		}
	}
	return out
}

// p2FillRow fills one row's choices and the sentence of a profile that is not
// there.
func p2FillRow(row *p2ARow, label string, options []p2Option, kinds map[string]string) {
	row.Options = p2GroupOptions(options)
	if kind, ok := kinds[row.Profile]; ok {
		row.KindOf, row.KindCls = kind, p2KindClass(kind)
		return
	}
	row.Missing = true
	if row.Kind == "m" {
		row.Error = tf("error.no_profile", label, row.Profile)
	} else {
		row.Error = tf("error.workers_no_profile", label, row.Profile)
	}
}

// p2Option is one name a definition resolves, with its kind.
type p2Option struct {
	Name string
	Kind string
}

// p2Resolved is the profiles one definition reads, in the order a name is
// written (the project's file first, then the shared one), and their kinds. A
// definition outside the project reads the shared file alone.
func p2Resolved(def p2Def, project, shared p2File) ([]p2Option, map[string]string) {
	var out []p2Option
	kinds := map[string]string{}
	add := func(entry profile.Entry) {
		if _, ok := kinds[entry.Name]; ok {
			return
		}
		kinds[entry.Name] = entry.Kind
		out = append(out, p2Option{Name: entry.Name, Kind: entry.Kind})
	}
	if def.local {
		for _, entry := range project.entries {
			add(entry)
		}
	}
	for _, entry := range shared.entries {
		add(entry)
	}
	return out, kinds
}

// p2GroupOptions is the choices of a select, one group per kind in the toolbar's
// order.
func p2GroupOptions(options []p2Option) []p2OptionGroup {
	order := map[string]int{}
	var kinds []string
	for _, option := range options {
		if _, ok := order[option.Kind]; !ok {
			order[option.Kind] = len(kinds)
			kinds = append(kinds, option.Kind)
		}
	}
	for i := 1; i < len(kinds); i++ {
		for j := i; j > 0 && p2KindOrder(kinds[j]) < p2KindOrder(kinds[j-1]); j-- {
			kinds[j], kinds[j-1] = kinds[j-1], kinds[j]
		}
	}
	out := make([]p2OptionGroup, 0, len(kinds))
	for _, kind := range kinds {
		group := p2OptionGroup{Kind: kind}
		for _, option := range options {
			if option.Kind == kind {
				group.Names = append(group.Names, option.Name)
			}
		}
		out = append(out, group)
	}
	return out
}
