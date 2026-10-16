package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/cheat"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
)

func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// testProfiles is the one profile a test team names: kind is the profile's, so a
// team.yaml body names it rather than writing the kind itself (I7).
func testProfiles(dir string) map[string]profile.Profile {
	return map[string]profile.Profile{
		"claude-default": {Name: "claude-default", Kind: "claude", Path: filepath.Join(dir, "profiles.yaml")},
	}
}

// teamText is a team.yaml body whose prompt is only the documents, so the
// tests read the concatenation without any other prompt part in the way.
func teamText(docs string) string {
	return "version: 1\nteam: t\nprompt: \"%{docs}\"\ndocs: " + docs + "\n" +
		"members:\n  - name: a\n    role: r\n    profile: claude-default\n"
}

func parseTeam(t *testing.T, dir, text string) (*Config, error) {
	t.Helper()
	return Parse(Input{
		YAML:         text,
		TeamName:     "t",
		Layer:        source.LayerLocal,
		TeamYAMLPath: filepath.Join(dir, "team.yaml"),
		TeamDir:      dir,
		Profiles:     testProfiles(dir),
		ProfilesYAML: filepath.Join(dir, "profiles.yaml"),
		Base:         dir,
		Reader:       source.Disk{},
		Executable:   "hergy",
	})
}

// oneRule is a rule file with one section (no heading) and one rule, so a test
// can put a known sentence into a member's prompt.
func oneRule(id, text string) string {
	return "version: 1\nsections:\n  - rules:\n      - id: " + id + "\n        text: |-\n          " + text + "\n"
}

func TestParseConcatenatesDocsInOrder(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "z.yaml"), oneRule("Z1", "Z doc"))
	write(t, filepath.Join(dir, "a.yaml"), oneRule("A1", "A doc"))
	write(t, filepath.Join(dir, "guide", "m.yaml"), oneRule("M1", "M doc"))
	write(t, filepath.Join(dir, "guide", "n.yaml"), oneRule("N1", "N doc"))
	write(t, filepath.Join(dir, "guide", "skip.txt"), "not a rule file\n")
	write(t, filepath.Join(dir, "guide", "deeper", "x.yaml"), oneRule("X1", "not directly inside"))

	cfg, err := parseTeam(t, dir, teamText(`["z.yaml", "a.yaml", "guide"]`))
	if err != nil {
		t.Fatal(err)
	}
	want := "\nZ doc\n\nA doc\n\nM doc\n\nN doc\n"
	if got := cfg.Members[0].Prompt; got != want {
		t.Errorf("prompt = %q, want %q", got, want)
	}

	var read []string
	for _, doc := range cfg.Docs {
		read = append(read, filepath.Base(doc.Path))
	}
	if wantDocs := []string{"z.yaml", "a.yaml", "m.yaml", "n.yaml"}; strings.Join(read, ",") != strings.Join(wantDocs, ",") {
		t.Errorf("Docs = %v, want %v", read, wantDocs)
	}
}

// TestParseRendersWhatIsSent: a rule file is rendered to Markdown, and only what
// a member reads reaches the prompt: the title, the headings, the intro, the
// text, the reason and the report shapes. The revision, the note, the origin,
// the report pointer and the tests stay in the file.
func TestParseRendersWhatIsSent(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "note.yaml"), `version: 1
title: TEAM PROTOCOL
revision: 3
sections:
  - heading: 基本
    intro: 前置きである。
    style: ids
    rules:
      - id: R1
        text: 本文である。
        why: 理由である。
        note: 保守する人のメモ
        report: 監査.scope
        tests: [T-1]
        origin:
          - kind: incident
            note: 覚え書き
reports:
  監査:
    where: 担当する Issue
    fields:
      scope: 変更の範囲
`)

	cfg, err := parseTeam(t, dir, teamText(`["note.yaml"]`))
	if err != nil {
		t.Fatal(err)
	}
	prompt := cfg.Members[0].Prompt
	for _, want := range []string{
		"# TEAM PROTOCOL", "## 基本", "前置きである。", "- **R1** 本文である。 理由である。",
		"## " + i18n.T("rules.reports_heading"), "**監査** (担当する Issue)", "- `scope`: 変更の範囲",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt is missing %q:\n%s", want, prompt)
		}
	}
	for _, unwanted := range []string{"revision", "保守する人のメモ", "覚え書き", "T-1", "監査.scope"} {
		if strings.Contains(prompt, unwanted) {
			t.Errorf("prompt still contains %q:\n%s", unwanted, prompt)
		}
	}
}

// TestParseStopsOnMarkdownInPlaceOfYAML: a docs entry that is still Markdown, a
// directory that holds only Markdown, and a role whose file is still Markdown
// each stop with the way to move over.
func TestParseStopsOnMarkdownInPlaceOfYAML(t *testing.T) {
	roleTeam := "version: 1\nteam: t\nprompt: \"%{role_doc}\"\nmembers:\n  - name: a\n    role: r\n    profile: claude-default\n"
	cases := []struct {
		name string
		file string
		text string
		team string
		want string
	}{
		{
			name: "a docs entry that is Markdown",
			file: "old.md",
			text: "- R1: old\n",
			team: teamText(`["old.md"]`),
			want: "old.md",
		},
		{
			name: "a directory that holds only Markdown",
			file: filepath.Join("docs", "old.md"),
			text: "- R1: old\n",
			team: teamText(`["docs"]`),
			want: "docs",
		},
		{
			name: "a role whose file is still Markdown",
			file: filepath.Join("roles", "r.md"),
			text: "- R1: old\n",
			team: roleTeam,
			want: "r.md",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, filepath.Join(dir, tc.file), tc.text)
			_, err := parseTeam(t, dir, tc.team)
			if err == nil {
				t.Fatal("Markdown in place of YAML did not stop the parse")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not name %q", err, tc.want)
			}
		})
	}
}

// TestParseStopsOnBreaksAcrossTheDocumentsOneMemberReads: a rule id and a report
// name are unique among the documents that go into one member's prompt, and a
// report and a derived rule point at something those documents hold.
func TestParseStopsOnBreaksAcrossTheDocumentsOneMemberReads(t *testing.T) {
	audit := "reports:\n  監査:\n    where: 記録\n    fields:\n      scope: 範囲\n"
	cases := []struct {
		name string
		docs map[string]string
		want string
	}{
		{
			name: "a rule id that stands twice",
			docs: map[string]string{
				"a.yaml": oneRule("R1", "one"),
				"b.yaml": oneRule("R1", "two"),
			},
			want: "R1",
		},
		{
			name: "a report name that stands twice",
			docs: map[string]string{
				"a.yaml": "version: 1\nsections:\n  - rules:\n      - id: A1\n        text: x\n" + audit,
				"b.yaml": "version: 1\nsections:\n  - rules:\n      - id: B1\n        text: y\n" + audit,
			},
			want: "監査",
		},
		{
			name: "a report that is not there",
			docs: map[string]string{
				"a.yaml": "version: 1\nsections:\n  - rules:\n      - id: A1\n        text: x\n        report: 監査.absent\n" + audit,
			},
			want: "監査.absent",
		},
		{
			name: "a derived rule that is not there",
			docs: map[string]string{
				"a.yaml": "version: 1\nsections:\n  - rules:\n      - id: A1\n        text: x\n        origin:\n          - kind: derived\n            refs: [B9]\n",
			},
			want: "B9",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			entries := make([]string, 0, len(tc.docs))
			for name, text := range tc.docs {
				write(t, filepath.Join(dir, name), text)
				entries = append(entries, name)
			}
			sort.Strings(entries)
			quoted := make([]string, len(entries))
			for i, name := range entries {
				quoted[i] = `"` + name + `"`
			}
			_, err := parseTeam(t, dir, teamText("["+strings.Join(quoted, ", ")+"]"))
			if err == nil {
				t.Fatal("a break across the documents did not stop the parse")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not name %q", err, tc.want)
			}
		})
	}
}

func TestParseErrorsOnMissingDocs(t *testing.T) {
	dir := t.TempDir()
	_, err := parseTeam(t, dir, teamText(`["nope.yaml"]`))
	if err == nil {
		t.Fatal("a docs entry that does not exist did not stop the parse")
	}
	if !strings.Contains(err.Error(), "nope.yaml") {
		t.Errorf("error %q does not name the missing entry", err)
	}
}

func TestParseStopsOnBrokenYAML(t *testing.T) {
	cases := []struct {
		name  string
		parse func() error
	}{
		{
			name: "team.yaml without a version",
			parse: func() error {
				_, err := parseTeam(t, t.TempDir(), "team: t\nmembers:\n  - name: a\n    role: r\n    profile: claude-default\n")
				return err
			},
		},
		{
			name: "team.yaml with an unknown key",
			parse: func() error {
				_, err := parseTeam(t, t.TempDir(), "version: 1\nteam: t\nunknown_key: 1\nmembers:\n  - name: a\n    role: r\n    profile: claude-default\n")
				return err
			},
		},
		{
			name: "config.yaml without a version",
			parse: func() error {
				_, err := ParseConfig("lang: ja\n", "config.yaml")
				return err
			},
		},
		{
			name: "config.yaml with an unknown key",
			parse: func() error {
				_, err := ParseConfig("version: 1\nlang: ja\nunknown_key: 1\n", "config.yaml")
				return err
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.parse(); err == nil {
				t.Error("broken YAML did not stop the parse")
			}
		})
	}
}

func TestParseReadsProfilesIntoMembers(t *testing.T) {
	dir := t.TempDir()
	profiles := map[string]profile.Profile{
		"lead": {Name: "lead", Kind: "claude", Args: []string{"--model", "opus"}, Path: "profiles.yaml"},
	}
	cfg, err := Parse(Input{
		YAML:         "version: 1\nteam: t\nprompt: \"%{name}\"\nmembers:\n  - name: a\n    role: r\n    profile: lead\n",
		TeamName:     "t",
		Layer:        source.LayerLocal,
		TeamYAMLPath: filepath.Join(dir, "team.yaml"),
		TeamDir:      dir,
		Base:         dir,
		Profiles:     profiles,
		ProfilesYAML: "profiles.yaml",
		Reader:       source.Disk{},
		Executable:   "hergy",
	})
	if err != nil {
		t.Fatal(err)
	}
	member := cfg.Members[0]
	if member.Kind != "claude" || member.Profile != "lead" {
		t.Errorf("member kind/profile = %q/%q, want claude/lead", member.Kind, member.Profile)
	}
	if strings.Join(member.Args, " ") != "--model opus" {
		t.Errorf("member args = %v", member.Args)
	}
	if !cfg.UsesProfiles() {
		t.Error("UsesProfiles() = false, want true")
	}
}

// parseMaySpawnTeam builds a one-member team whose may_spawn is the given block
// (already indented under may_spawn), with two profiles defined. The prompt is
// only %{spawn}, so no role file is needed.
func parseMaySpawnTeam(t *testing.T, maySpawn string) (*Config, error) {
	t.Helper()
	dir := t.TempDir()
	profiles := map[string]profile.Profile{
		"seeker":    {Name: "seeker", Kind: "claude"},
		"developer": {Name: "developer", Kind: "codex"},
	}
	return Parse(Input{
		YAML: "version: 1\nteam: t\nprompt: \"%{spawn}\"\nmembers:\n" +
			"  - name: lead\n    role: r\n    profile: seeker\n" +
			"    may_spawn:\n" + maySpawn,
		TeamName:     "t",
		Layer:        source.LayerLocal,
		TeamYAMLPath: filepath.Join(dir, "team.yaml"),
		TeamDir:      dir,
		Base:         dir,
		Profiles:     profiles,
		ProfilesYAML: "profiles.yaml",
		Reader:       source.Disk{},
		Executable:   "hergy",
	})
}

// TestMaySpawnReadsPurposeProfilePairs: may_spawn is a list of purpose/profile
// pairs, and each pair reaches the member's prompt.
func TestMaySpawnReadsPurposeProfilePairs(t *testing.T) {
	cfg, err := parseMaySpawnTeam(t, "      workers:\n"+
		"        - purpose: research\n          profile: seeker\n"+
		"        - purpose: build\n          profile: developer\n")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Members[0].MaySpawn == nil {
		t.Fatal("may_spawn was not read")
	}
	if got, ok := cfg.Members[0].MaySpawn.ProfileFor("build"); !ok || got != "developer" {
		t.Errorf("ProfileFor(build) = %q, %v, want developer, true", got, ok)
	}
	if !cfg.UsesProfiles() {
		t.Error("UsesProfiles() = false, want true")
	}
	for _, want := range []string{"research → seeker", "build → developer"} {
		if !strings.Contains(cfg.Members[0].Prompt, want) {
			t.Errorf("the prompt does not pair %q:\n%s", want, cfg.Members[0].Prompt)
		}
	}
}

// TestMaySpawnStopsOnBrokenPairs: every way a may_spawn block breaks stops the
// parse, naming the pair or the key it did not like.
func TestMaySpawnStopsOnBrokenPairs(t *testing.T) {
	cases := []struct{ name, block, want string }{
		{"the old purposes and profiles", "      purposes: [research]\n      profiles: [seeker]\n", "workers"},
		{"workers empty", "      workers: []\n", "workers"},
		{"workers null", "      workers:\n", "workers"},
		{"workers not a list", "      workers: research\n", "workers"},
		{"a worker without a purpose", "      workers:\n        - profile: seeker\n", "purpose"},
		{"a worker without a profile", "      workers:\n        - purpose: research\n", "profile"},
		{"a profile that is not in profiles.yaml", "      workers:\n        - purpose: research\n          profile: absent\n", "absent"},
		{"two workers with one purpose", "      workers:\n        - purpose: research\n          profile: seeker\n        - purpose: research\n          profile: developer\n", "research"},
		{"an unknown key in a worker", "      workers:\n        - purpose: research\n          profile: seeker\n          kind: claude\n", "kind"},
		{"an unknown key in may_spawn", "      purpose: research\n", "purpose"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseMaySpawnTeam(t, tc.block)
			if err == nil {
				t.Fatal("a broken may_spawn did not stop the parse")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not name %q", err, tc.want)
			}
		})
	}
}

func TestParseUnknownProfileStops(t *testing.T) {
	_, err := Parse(Input{
		YAML:         "version: 1\nteam: t\nmembers:\n  - name: a\n    role: r\n    profile: absent\n",
		TeamName:     "t",
		Layer:        source.LayerLocal,
		TeamYAMLPath: "team.yaml",
		Reader:       source.Disk{},
	})
	if err == nil {
		t.Fatal("a profile that does not exist did not stop the parse")
	}
	if !strings.Contains(err.Error(), "absent") {
		t.Errorf("error %q does not name the missing profile", err)
	}
}

// TestMemberCannotWriteItsOwnEngine: kind, args and env are the profile's, so a
// member that writes one of them is stopped with the sentence that says where it
// goes (I7). The three are refused the same way, so a file that reads a member
// with one of them is fixed before the team boots.
func TestMemberCannotWriteItsOwnEngine(t *testing.T) {
	cases := []struct{ name, member, want string }{
		{"kind", "    kind: claude\n", "kind"},
		{"args", "    args: [--model, opus]\n", "args"},
		{"env", "    env: {ANTHROPIC_MODEL: opus}\n", "env"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseTeam(t, t.TempDir(),
				"version: 1\nteam: t\nmembers:\n  - name: lead\n    role: r\n    profile: claude-default\n"+tc.member)
			if err == nil {
				t.Fatalf("a member writing %s did not stop the parse", tc.name)
			}
			for _, want := range []string{"lead", tc.want, "profiles.yaml"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name %q", err, want)
				}
			}
		})
	}
}

// TestMemberNeedsAProfile: a member with no profile has no engine at all, and is
// stopped with the way to give it one.
func TestMemberNeedsAProfile(t *testing.T) {
	_, err := parseTeam(t, t.TempDir(), "version: 1\nteam: t\nmembers:\n  - name: lead\n    role: r\n")
	if err == nil {
		t.Fatal("a member with no profile did not stop the parse")
	}
	for _, want := range []string{"lead", "profile", "profiles.yaml"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// TestMemberNameGetsTheTeamPrefix: the name written in team.yaml is the short member name;
// the member's herdr name is "<team>-<member>".
func TestMemberNameGetsTheTeamPrefix(t *testing.T) {
	dir := t.TempDir()
	cfg, err := parseTeam(t, dir, "version: 1\nteam: t\nprompt: \"%{name}\"\nmembers:\n  - name: lead\n    role: r\n    profile: claude-default\n")
	if err != nil {
		t.Fatal(err)
	}
	member := cfg.Members[0]
	if member.ShortName != "lead" {
		t.Errorf("ShortName = %q, want %q", member.ShortName, "lead")
	}
	if member.Name != "t-lead" {
		t.Errorf("Name = %q, want %q", member.Name, "t-lead")
	}
}

// TestMemberNameTakesTheRunName: --as replaces the prefix, not the member name.
func TestMemberNameTakesTheRunName(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Parse(Input{
		YAML:         "version: 1\nprompt: \"%{name}\"\nmembers:\n  - name: lead\n    role: r\n    profile: claude-default\n",
		TeamName:     "dev",
		RunName:      "dev2",
		Layer:        source.LayerLocal,
		TeamYAMLPath: filepath.Join(dir, "team.yaml"),
		TeamDir:      dir,
		Profiles:     testProfiles(dir),
		Base:         dir,
		Reader:       source.Disk{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Members[0].Name != "dev2-lead" {
		t.Errorf("Name = %q, want %q", cfg.Members[0].Name, "dev2-lead")
	}
	if cfg.Members[0].ShortName != "lead" {
		t.Errorf("ShortName = %q, want %q", cfg.Members[0].ShortName, "lead")
	}
}

// TestMemberNameWithTeamPlaceholderStops: the team name is added for the writer,
// so a literal %{team} left in a member name is refused.
func TestMemberNameWithTeamPlaceholderStops(t *testing.T) {
	_, err := parseTeam(t, t.TempDir(), "version: 1\nteam: t\nmembers:\n  - name: \"%{team}-lead\"\n    role: r\n    profile: claude-default\n")
	if err == nil {
		t.Fatal("a member name with %{team} did not stop the parse")
	}
	if !strings.Contains(err.Error(), "%{team}") {
		t.Errorf("error %q does not name the leftover placeholder", err)
	}
}

// TestMemberNameTooLongForHerdrStops: the team name and the member name are each within
// their own pattern, but the combined herdr name is checked too.
func TestMemberNameTooLongForHerdrStops(t *testing.T) {
	_, err := Parse(Input{
		YAML:         "version: 1\nmembers:\n  - name: worker\n    role: r\n    profile: claude-default\n",
		TeamName:     "abcdefghijklmnopqrstuvwxyz", // 26 chars, over the team-name limit
		Layer:        source.LayerLocal,
		TeamYAMLPath: "team.yaml",
		Reader:       source.Disk{},
	})
	if err == nil {
		t.Fatal("an over-long herdr name did not stop the parse")
	}
	if !strings.Contains(err.Error(), "abcdefghijklmnopqrstuvwxyz-worker") {
		t.Errorf("error %q does not name the combined name", err)
	}
}

// TestConfigYAMLRoundTrips: the config.yaml hergy writes is one ParseConfig
// reads back to the same language.
func TestConfigYAMLRoundTrips(t *testing.T) {
	for _, lang := range []string{"en", "ja"} {
		text := GlobalYAML(lang)
		got, err := ParseConfig(text, "config.yaml")
		if err != nil {
			t.Fatalf("ParseConfig(%q) = %v", text, err)
		}
		if got != lang {
			t.Errorf("ParseConfig(%q) lang = %q, want %q", text, got, lang)
		}
	}
}

// TestCheatMachineNamesEveryConfigKey collects every yaml key the settings
// structs read, by reflection over their yaml tags, and checks the agent
// cheatsheet names each one, so a key added later cannot be left out.
func TestCheatMachineNamesEveryConfigKey(t *testing.T) {
	keys := map[string]bool{}
	roots := []any{
		rawTeam{}, rawMember{}, rawMaySpawn{}, rawConfig{},
	}
	for _, root := range roots {
		collectYAMLKeys(keys, reflect.TypeOf(root))
	}
	if len(keys) == 0 {
		t.Fatal("no yaml keys were collected; the reflection walk is broken")
	}
	for key := range keys {
		if !strings.Contains(cheat.Machine, key) {
			t.Errorf("cheat --ai does not name the settings key %q", key)
		}
	}
}

// collectYAMLKeys adds every yaml tag under a type to keys, descending into the
// settings structs of this package only, so the yaml library's own structs are
// left alone. Pointers, slices, arrays and maps are unwrapped.
// TestParseReadsTheMission: team.yaml's mission: is the one line a team says it
// is for (r-341e). It is optional, so a file without it carries none; the ends
// of what it wrote are trimmed, and a mission written over more than one line
// keeps its lines, because hergy does not join them: validate says it should be
// one line, and MissionLine is what the output shows.
func TestParseReadsTheMission(t *testing.T) {
	dir := t.TempDir()
	// teamWith is a team.yaml body with the given text in place of the mission
	// line (empty for none), so each case writes only what it is about.
	teamWith := func(mission string) string {
		return "version: 1\n" + mission +
			"team: t\nprompt: \"%{docs}\"\ndocs: []\nmembers:\n  - name: a\n    role: r\n    profile: claude-default\n"
	}
	for _, tc := range []struct {
		name    string
		yaml    string
		mission string
		line    string
	}{
		{
			name:    "no mission",
			yaml:    teamWith(""),
			mission: "",
			line:    "",
		},
		{
			name:    "one line",
			yaml:    teamWith("mission:  hergy をよくする  \n"),
			mission: "hergy をよくする",
			line:    "hergy をよくする",
		},
		{
			name:    "a block scalar",
			yaml:    teamWith("mission: |-\n  one line\n"),
			mission: "one line",
			line:    "one line",
		},
		{
			name:    "over two lines",
			yaml:    teamWith("mission: |-\n  first\n  second\n"),
			mission: "first\nsecond",
			line:    "first",
		},
		{
			name:    "written empty",
			yaml:    teamWith("mission: \"\"\n"),
			mission: "",
			line:    "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := parseTeam(t, dir, tc.yaml)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Mission != tc.mission {
				t.Errorf("Mission = %q, want %q", cfg.Mission, tc.mission)
			}
			if got := cfg.MissionLine(); got != tc.line {
				t.Errorf("MissionLine = %q, want %q", got, tc.line)
			}
		})
	}
}

func collectYAMLKeys(keys map[string]bool, typ reflect.Type) {
	if typ == nil {
		return
	}
	switch typ.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		collectYAMLKeys(keys, typ.Elem())
	case reflect.Struct:
		if typ.PkgPath() != reflect.TypeFor[rawTeam]().PkgPath() {
			return
		}
		for field := range typ.Fields() {
			if tag, _, _ := strings.Cut(field.Tag.Get("yaml"), ","); tag != "" && tag != "-" {
				keys[tag] = true
			}
			collectYAMLKeys(keys, field.Type)
		}
	default:
		// scalars carry no yaml keys.
	}
}
