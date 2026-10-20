package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"slices"
	"strings"

	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
	"go.yaml.in/yaml/v3"
)

// This file gathers what the profile screens show (r-c683): the two
// profiles.yaml files and the profiles they hold, the team definitions that read
// them and the members and workers that name a profile, and the problems the
// same reading the command line uses found. The screens decide nothing of the
// model themselves; they lay out what this reading returned.

// The two places a profiles.yaml stands, as the screen names them.
const (
	p2Project = "project"
	p2Shared  = "global"
)

// p2Data is what one profile page's shell draws: the tab, its counts, the
// toolbar's chips and, for the 割り当て tab, the assignment cards. The rows and
// files the script draws travel in the page's island (p2Island), in the
// boundary's own shape (spec-boundary 5).
type p2Data struct {
	Lang         string      `json:"-"`
	Nav          navData     `json:"-"`
	Token        string      `json:"-"`
	Data         template.JS `json:"-"`
	Tab          string      `json:"tab"`
	ProfileCount int         `json:"profileCount"`
	AssignCount  int         `json:"assignCount"`
	HashProject  string      `json:"hashProject"`
	HashShared   string      `json:"hashShared"`
	PathProject  string      `json:"pathProject"`
	PathShared   string      `json:"pathShared"`
	OtherKinds   string      `json:"otherKinds"`
	Kinds        []p2Kind    `json:"kinds"`
	Query        string      `json:"query"`
	KindFilter   string      `json:"kindFilter"`
	Rows         []p2Row     `json:"rows"`
	Empty        bool        `json:"empty"`
	NoMatch      bool        `json:"noMatch"`
	Assign       *p2Assign   `json:"assign,omitempty"`
	Files        []p2Yed     `json:"files,omitempty"`
}

// p2Kind is one chip of the profile tab's toolbar.
type p2Kind struct {
	Name      string `json:"name"`
	Count     int    `json:"count"`
	KindClass string `json:"kindClass"`
	On        bool   `json:"on"`
}

// p2Place is one 「置き場所」 pill: プロジェクト or 共通, and whether it is the
// one in effect.
type p2Place struct {
	Label string `json:"label"`
	State string `json:"state"` // "on", "shadow" or "pending"
}

// p2UserChip is one person a profile is used by, as the expanded row shows it.
type p2UserChip struct {
	Def    string `json:"def"`
	ID     string `json:"id"` // m-<i> or w-<i>
	Who    string `json:"who"`
	Role   string `json:"role"`
	Worker bool   `json:"worker"`
	Local  bool   `json:"local"` // the definition is the project's own, so it reads the project's file
}

// p2Panel is one file's panel of an expanded row: the values a save would write
// and the values the file holds now.
type p2Panel struct {
	File        string            `json:"file"`
	Label       string            `json:"label"`
	Path        string            `json:"path"`
	Saved       bool              `json:"saved"`
	Shadowed    bool              `json:"shadowed"`
	Both        bool              `json:"both"`
	Other       string            `json:"other"`
	OtherLabel  string            `json:"otherLabel"`
	Name        string            `json:"name"`
	Kind        string            `json:"kind"`
	KindClass   string            `json:"kindClass"`
	Args        []string          `json:"args"`
	Env         []profile.EnvPair `json:"env"`
	ArgsJSON    string            `json:"argsJSON"`
	EnvJSON     string            `json:"envJSON"`
	SavedName   string            `json:"savedName"`
	SavedKind   string            `json:"savedKind"`
	SavedArgs   string            `json:"savedArgs"`
	SavedEnv    string            `json:"savedEnv"`
	Hash        string            `json:"hash"`
	Users       int               `json:"users"`
	RenameUsers int               `json:"renameUsers"`
	Problems    []profile.Problem `json:"-"`
	Err         int               `json:"err"`
	Warn        int               `json:"warn"`
	SavedLine   bool              `json:"savedLine"`
}

// p2Row is one profile of the list.
type p2Row struct {
	Name      string       `json:"name"`
	Kind      string       `json:"kind"`
	KindClass string       `json:"kindClass"`
	New       bool         `json:"new"`
	Open      bool         `json:"open"`
	Places    []p2Place    `json:"places"`
	Args      string       `json:"args"`
	Env       []string     `json:"env"`
	More      int          `json:"more"`
	Users     int          `json:"users"`
	Roles     string       `json:"roles"`
	Err       int          `json:"err"`
	Warn      int          `json:"warn"`
	Chips     []p2UserChip `json:"chips"`
	Panels    []p2Panel    `json:"panels"`
}

// p2KindBar is one segment of the assign tab's kind bar.
type p2KindBar struct {
	Name      string `json:"name"`
	Count     int    `json:"count"`
	Grow      string `json:"grow"`
	KindClass string `json:"kindClass"`
	Dim       bool   `json:"dim"`
	On        bool   `json:"on"`
}

// p2Assign is the assign tab.
type p2Assign struct {
	Kinds []p2KindBar `json:"kinds"`
	Teams []p2Team    `json:"teams"`
	Empty bool        `json:"empty"`
}

// p2Team is one team definition's card.
type p2Team struct {
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	Members int      `json:"members"`
	Workers int      `json:"workers"`
	Err     int      `json:"err"`
	Rows    []p2ARow `json:"rows"`
	Hash    string   `json:"hash"`
	Saved   bool     `json:"saved"`
}

// p2ARow is one member or worker of a card.
type p2ARow struct {
	Kind    string          `json:"kind"` // "m" or "w"
	ID      string          `json:"id"`   // "m-<i>" or "w-<i>"
	Name    string          `json:"name"`
	Role    string          `json:"role"`
	Profile string          `json:"profile"`
	Missing bool            `json:"missing"`
	Error   string          `json:"error"`
	Options []p2OptionGroup `json:"options"`
	KindOf  string          `json:"kindOf"`
	KindCls string          `json:"kindCls"`
	Dim     bool            `json:"dim"`
}

// p2OptionGroup is one kind's choices of a select.
type p2OptionGroup struct {
	Kind  string   `json:"kind"`
	Names []string `json:"names"`
}

// p2Yed is one file's section of the YAML tab.
type p2Yed struct {
	File     string       `json:"file"`
	Label    string       `json:"label"`
	Path     string       `json:"path"`
	Text     string       `json:"text"`
	Hash     string       `json:"hash"`
	Height   string       `json:"height"`
	Lines    []p2Line     `json:"lines"`
	Problems []p2YProblem `json:"problems"`
	Err      int          `json:"err"`
	Warn     int          `json:"warn"`
	Dirty    bool         `json:"dirty"`
	Missing  bool         `json:"missing"`
}

// p2Line is one line of the gutter, with the mark a problem put on it.
type p2Line struct {
	N    int    `json:"n"`
	Mark string `json:"mark"`
}

// p2YProblem is one problem of the YAML tab.
type p2YProblem struct {
	Sev  string `json:"sev"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// p2File is one profiles.yaml as this reading found it. ok says the file is
// there; a file that is not there reads as empty so a save may make it.
type p2File struct {
	file     string
	label    string
	path     string
	display  string
	ok       bool
	text     string
	hash     string
	entries  []profile.Entry
	problems []profile.Problem
}

// p2Member and p2Worker are one person of a team definition that names a
// profile.
type p2Member struct {
	Name    string
	Role    string
	Profile string
}

type p2Worker struct {
	From    string
	Purpose string
	Profile string
}

// p2Def is one team definition and the people that read profiles.
type p2Def struct {
	name    string
	path    string
	display string
	local   bool
	members []p2Member
	workers []p2Worker
	hash    string
}

// p2Read reads both profiles.yaml files and every team definition.
func (s *Server) p2Read() (p2File, p2File, []p2Def) {
	project := s.p2ReadFile(p2Project, s.cat.ProjectProfileFile())
	shared := s.p2ReadFile(p2Shared, s.cat.SharedProfileFile())
	return project, shared, s.p2Defs()
}

// p2ReadFile reads one profiles.yaml through the catalog's reader, so a text a
// screen laid over the file reads back. A file that is not there is empty and
// its hash is the hash of the empty text, which is what a save that makes the
// file carries.
func (s *Server) p2ReadFile(file, path string) p2File {
	out := p2File{file: file, label: p2Label(file), path: path, display: s.cat.Display(path), hash: source.Hash("")}
	text, err := s.cat.Reader().Read(source.Doc{Path: path})
	if err != nil {
		_ = err
		return out
	}
	out.ok = true
	out.text = text
	out.hash = source.Hash(text)
	out.entries, out.problems = profile.ReadAll(text, out.display)
	return out
}

// p2Label is the word the screen uses for one file's place.
func p2Label(file string) string {
	if file == p2Shared {
		return "共通"
	}
	return "プロジェクト"
}

// p2Defs reads every team definition's members and workers. It reads the YAML
// directly rather than through definition.Read, because a definition whose
// profile is missing still has to be shown and fixed here.
func (s *Server) p2Defs() []p2Def {
	rows := s.cat.Definitions()
	out := make([]p2Def, 0, len(rows))
	for _, row := range rows {
		path := row.GlobalPath
		local := false
		if row.Local {
			path, local = row.LocalPath, true
		}
		def := p2Def{name: row.Name, path: path, display: s.cat.Display(path), local: local}
		text, err := s.cat.Reader().Read(source.Doc{Path: path})
		if err != nil {
			out = append(out, def)
			continue
		}
		def.hash = source.Hash(text)
		var raw struct {
			Members []struct {
				Name    string `yaml:"name"`
				Role    string `yaml:"role"`
				Profile string `yaml:"profile"`
			} `yaml:"members"`
			Workers []struct {
				From    string `yaml:"from"`
				Purpose string `yaml:"purpose"`
				Profile string `yaml:"profile"`
			} `yaml:"workers"`
		}
		if err := yaml.Unmarshal([]byte(text), &raw); err != nil {
			out = append(out, def)
			continue
		}
		for _, m := range raw.Members {
			def.members = append(def.members, p2Member{Name: m.Name, Role: m.Role, Profile: m.Profile})
		}
		for _, w := range raw.Workers {
			def.workers = append(def.workers, p2Worker{From: w.From, Purpose: w.Purpose, Profile: w.Profile})
		}
		out = append(out, def)
	}
	return out
}

// p2Chips is every person that names a profile, by the profile's name, in the
// order the definitions and their people are read.
func p2Chips(defs []p2Def) map[string][]p2UserChip {
	out := map[string][]p2UserChip{}
	for _, def := range defs {
		for i, member := range def.members {
			if member.Profile == "" {
				continue
			}
			out[member.Profile] = append(out[member.Profile], p2UserChip{
				Def: def.name, ID: fmt.Sprintf("m-%d", i), Who: member.Name, Role: member.Role, Local: def.local,
			})
		}
		for i, worker := range def.workers {
			if worker.Profile == "" {
				continue
			}
			out[worker.Profile] = append(out[worker.Profile], p2UserChip{
				Def: def.name, ID: fmt.Sprintf("w-%d", i), Who: worker.From, Role: worker.Purpose, Worker: true, Local: def.local,
			})
		}
	}
	return out
}

// p2Names is every profile name the two files hold, the project's order first.
func p2Names(project, shared p2File) []string {
	names := make([]string, 0, len(project.entries)+len(shared.entries))
	seen := map[string]bool{}
	for _, entry := range project.entries {
		if !seen[entry.Name] {
			seen[entry.Name] = true
			names = append(names, entry.Name)
		}
	}
	for _, entry := range shared.entries {
		if !seen[entry.Name] {
			seen[entry.Name] = true
			names = append(names, entry.Name)
		}
	}
	return names
}

// p2Find is one name's entry in one file, and whether the file holds it.
func p2Find(file p2File, name string) (profile.Entry, bool) {
	for _, entry := range file.entries {
		if entry.Name == name {
			return entry, true
		}
	}
	return profile.Entry{}, false
}

// p2People is how many members and workers every definition has, the number the
// 割り当て tab carries.
func p2People(defs []p2Def) int {
	total := 0
	for _, def := range defs {
		total += len(def.members) + len(def.workers)
	}
	return total
}

// p2Users is how many people name one profile.
func p2Users(chips map[string][]p2UserChip, name string) []p2UserChip {
	return chips[name]
}

// p2KindClass is the colour class a kind carries, empty for a kind outside the
// four the stylesheet colours.
func p2KindClass(kind string) string {
	switch kind {
	case "claude", "codex", "opencode", "gemini":
		return "k-" + kind
	default:
		return ""
	}
}

// p2KindOrder is where a kind stands in the order the toolbar shows: claude,
// codex, opencode, gemini, then the rest in profile.Kinds()'s order.
func p2KindOrder(kind string) int {
	switch kind {
	case "claude":
		return 0
	case "codex":
		return 1
	case "opencode":
		return 2
	case "gemini":
		return 3
	}
	for i, k := range profile.Kinds() {
		if k == kind {
			return 4 + i
		}
	}
	return 4 + len(profile.Kinds())
}

// p2Counts is the number of errors and warnings of a list of problems.
func p2Counts(problems []profile.Problem) (errs, warns int) {
	for _, problem := range problems {
		if problem.Sev == profile.Err {
			errs++
		} else {
			warns++
		}
	}
	return errs, warns
}

// p2JSON writes a value as the JSON a data attribute carries.
func p2JSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "null"
	}
	return string(raw)
}

// p2EnvJSON is one environment variable as a data attribute carries it.
type p2EnvJSON struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// p2EnvPairs is env as the data attributes carry it, the order the file wrote
// kept.
func p2EnvPairs(env []profile.EnvPair) []p2EnvJSON {
	out := make([]p2EnvJSON, 0, len(env))
	for _, pair := range env {
		out = append(out, p2EnvJSON{Key: pair.Key, Value: pair.Value})
	}
	return out
}

// p2Roles is the roles and purposes of the people that use one profile, joined
// the way the row shows them, each word once.
func p2Roles(chips []p2UserChip) string {
	seen := map[string]bool{}
	var out []string
	for _, chip := range chips {
		if chip.Role == "" || seen[chip.Role] {
			continue
		}
		seen[chip.Role] = true
		out = append(out, chip.Role)
	}
	return strings.Join(out, "・")
}

// p2Sorted is a list of problems in the line order the YAML tab reads.
func p2Sorted(problems []profile.Problem) []profile.Problem {
	out := slices.Clone(problems)
	slices.SortStableFunc(out, func(a, b profile.Problem) int { return a.Line - b.Line })
	return out
}
