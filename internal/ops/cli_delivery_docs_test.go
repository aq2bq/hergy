package ops

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/migrate"
	"github.com/aq2bq/hergy/internal/ops/internal/deliver"
	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
	"github.com/aq2bq/hergy/internal/source"
)

func TestPreviousDocumentsBelongToTheirReader(t *testing.T) {
	initLocales(t)
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "markdown", true: "legacy YAML"}[legacy], func(t *testing.T) {
			log := launchlog.At(t.TempDir())
			text, ext := "# Shared wording\nDo the task.\n", ".md"
			rendered := text
			if legacy {
				ext = ".yaml"
				text = "version: 1\ntitle: Shared wording\nsections:\n  - rules:\n      - id: ABCD\n        text: Do the task.\n"
				var err error
				rendered, err = migrate.LegacyMarkdown(text, "role.yaml")
				if err != nil {
					t.Fatal(err)
				}
			}
			hash, err := log.Put([]byte(text))
			if err != nil {
				t.Fatal(err)
			}
			record := launchlog.Record{Team: fixtureSeen("demo"), Lang: "en", Docs: []launchlog.Doc{{Path: "/roles/first" + ext, SHA256: hash}, {Path: "/roles/second" + ext, SHA256: hash}}}
			for _, role := range []string{"first", "second"} {
				member := launchlog.Member{Name: fixtureSeen("demo-" + role), Short: role}
				prompt := "You are " + member.Name.String() + " (" + role + ") on team demo.\n\n" + rendered
				old := oldDocs(log, record, member, prompt)
				if len(old) != 1 || old["/roles/"+role+ext] != text {
					t.Fatalf("%s read %+v", role, old)
				}
				now := rendered
				if legacy {
					converted, err := migrate.Convert(text, "role.yaml")
					if err != nil {
						t.Fatal(err)
					}
					now = converted.Text
				}
				current := definition.Start{Prompt: prompt + "new roster", Segments: []definition.PromptSegment{{Kind: "roster", Text: "new roster"}}}
				docs := []deliver.Doc{{Path: "/roles/" + role + ".md", Display: role + ".md", Text: now}}
				got := deliver.Compare(fixtureDemoTeam(), "en", prompt, old, current, docs)
				if len(got) != 1 || got[0].Kind != "roster" {
					t.Fatalf("unchanged documents: %+v", got)
				}
				docs[0].Text += "Changed.\n"
				got = deliver.Compare(fixtureDemoTeam(), "en", prompt, old, current, docs)
				if len(got) != 2 || got[0].Document != role+".md" || strings.Contains(got[0].Text, ".yaml") {
					t.Fatalf("changed document: %+v", got)
				}

				got = deliver.Compare(fixtureDemoTeam(), "en", prompt, old, current, []deliver.Doc{{Path: "/roles/third.md", Display: "third.md", Text: "New role.\n"}})
				if len(got) != 3 || got[1].Document != "/roles/"+role+".md" {
					t.Fatalf("role switch: %+v", got)
				}
			}
		})
	}
}
func TestRecordedMemberDocsOverridePromptText(t *testing.T) {
	log := launchlog.At(t.TempDir())
	hash, err := log.Put([]byte("same text"))
	if err != nil {
		t.Fatal(err)
	}
	record := launchlog.Record{Docs: []launchlog.Doc{{Path: "/roles/old.md", SHA256: hash}, {Path: "/roles/peer.md", SHA256: hash}}}
	member := launchlog.Member{Docs: []string{"/roles/old.md"}}
	got := oldDocs(log, record, member, "same text")
	if len(got) != 1 || got["/roles/old.md"] != "same text" {
		t.Fatalf("member documents: %+v", got)
	}
	member.Docs = []string{}
	if got := oldDocs(log, record, member, "same text"); len(got) != 0 {
		t.Fatalf("empty reading: %+v", got)
	}
}
func TestRosterChangeHasNoOtherMembersDocumentsOnEitherDeliveryPath(t *testing.T) {
	f := applyDemo(t)
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
		`{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1},
		{"name":"demo-builder","agent":"claude","agent_status":"idle","pane_id":"p2","tab_id":"t1","state_change_seq":1}`)
	path := filepath.Join(f.proj, ".hergy/roles/builder.md")
	writeFile(t, path, strings.Replace(readFile(t, path), "# Builder", "# Updated builder", 1))

	env := serveEnv(t)
	cat := env.Catalog(source.Disk{})
	r, err := ReadDefinition(cat, env.ProjectRoot, "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}
	resent := promptsToResend(cat, env.ProjectRoot, r, r.Starts)
	for _, member := range resent {
		if member.Short.String() == "lead" && strings.Contains(strings.Split(member.Prompt, "You are ")[0], "builder.md") {
			t.Fatalf("reprompt leaked peer document: %s", member.Prompt)
		}
	}
	sheet := newActivityRead(env).Apply()
	checked := 0
	for _, team := range sheet.Teams {
		for _, member := range team.Members {
			checked++
			if member.Short == "lead" {
				for _, change := range member.Changes {
					if change.Kind == "document" {
						t.Fatalf("sheet lead document: %+v", change)
					}
				}
			}
		}
	}
	if checked != 2 {
		t.Fatalf("sheet members: %+v", sheet)
	}
	out := mustRun(t, "apply", "--dry", "--json")
	if !strings.Contains(out, "roster") {
		t.Fatalf("missing roster: %s", out)
	}

	var cli wireApplyList
	if err := json.Unmarshal([]byte(out), &cli); err != nil {
		t.Fatal(err)
	}
	for _, row := range cli.Teams[0].Members {
		for _, shown := range sheet.Teams[0].Members {
			if row.Name == shown.Name && !reflect.DeepEqual(row.Changes, shown.Changes) {
				t.Fatalf("CLI and sheet disagree: %+v / %+v", row, shown)
			}
		}
		for i, member := range resent {
			if row.Name == member.Name.String() && member.Prompt != deliver.Preamble(internalChanges(row.Changes))+r.Starts[i].Prompt {
				t.Fatalf("reprompt differs for %s", row.Name)
			}
		}
	}
	_, records := launchLog(t, f)
	for _, member := range records[len(records)-1].Members {
		if len(member.Docs) != 2 {
			t.Fatalf("record lacks reading: %+v", member)
		}
		other := "builder"
		if member.Short == "builder" {
			other = "lead"
		}
		if slices.Contains(member.Docs, filepath.Join(f.proj, ".hergy/roles/"+other+".md")) {
			t.Fatalf("record contains peer: %+v", member)
		}
	}
}
