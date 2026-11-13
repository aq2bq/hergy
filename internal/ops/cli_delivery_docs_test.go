package ops

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/ops/internal/deliver"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
	"github.com/aq2bq/hergy/internal/source"
)

func TestPreviousDocumentsBelongToTheirReader(t *testing.T) {
	receipt := runningstate.Receipt{Docs: []runningstate.Doc{{Path: "/roles/old.md", Text: "same text"}, {Path: "/agreement.md", Text: ""}}}
	got := oldDocs(receipt)
	if len(got) != 2 || got["/roles/old.md"] != "same text" {
		t.Fatalf("member documents: %+v", got)
	}
	receipt.Docs = nil
	if got := oldDocs(receipt); len(got) != 0 {
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
