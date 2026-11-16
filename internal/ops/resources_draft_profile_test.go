package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/definition"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/source"
)

// TestNewDefinitionRefusesAMemberWhoseProfileIsNowhere is ac-5aa7: a new team
// definition that takes the starter profile is saved only while its members name
// that profile. A member rewritten to a profile neither place holds is refused,
// by the check a screen shows and by the save, as an existing definition's edit
// refuses it.
func TestNewDefinitionRefusesAMemberWhoseProfileIsNowhere(t *testing.T) {
	f := newFixture(t)
	useJapanese(t)
	env := serveEnv(t)
	scope := ResourceScope{Catalog: env.Catalog(source.Disk{})}
	at := domainplace.In(env.ProjectRoot)
	if err := os.MkdirAll(filepath.Join(f.proj, ".hergy"), 0o755); err != nil {
		t.Fatal(err)
	}
	draft := func(name string) ResourceDraftInput {
		return ResourceDraftInput{Place: at, Name: name, New: true, StarterRoles: true, StarterProfile: true}
	}
	rewrite := []definition.Op{definition.MemberProfile("lead", "nowhere")}

	// 1. The rewritten member stops the check and the save, and both say which
	// member and which field.
	checked, bad := scope.CheckDefinitionDraft(draft("one"), rewrite)
	if bad != nil {
		t.Fatal(bad.Text)
	}
	stops := 0
	for _, finding := range checked.Findings {
		if finding.Stops && finding.Field == "members[0].profile" {
			stops++
			if !strings.Contains(finding.Text, "lead") {
				t.Errorf("the finding does not name the member: %s", finding.Text)
			}
		}
	}
	if stops != 1 {
		t.Errorf("the check stops %d times on members[0].profile, want once: %+v", stops, checked.Findings)
	}
	one, err := NewResourceRef(domainplace.Definition, at, "one")
	if err != nil {
		t.Fatal(err)
	}
	_, bad = scope.SaveDefinitionDraft(one, draft("one"), rewrite, nil)
	if bad == nil {
		t.Fatal("the save took a member whose profile is nowhere")
	}
	if bad.Field != "members[0].profile" || !strings.Contains(bad.Text, "lead") {
		t.Errorf("the refusal reads field %q, text %q", bad.Field, bad.Text)
	}
	if _, err := os.Stat(filepath.Join(f.proj, ".hergy", "teams", "one")); !os.IsNotExist(err) {
		t.Errorf("the refused save left the folder: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.proj, ".hergy", "profiles.yaml")); !os.IsNotExist(err) {
		t.Errorf("the refused save wrote profiles.yaml: %v", err)
	}

	// 2. With the starter's own profile name the save goes through.
	two, err := NewResourceRef(domainplace.Definition, at, "two")
	if err != nil {
		t.Fatal(err)
	}
	saved, bad := scope.SaveDefinitionDraft(two, draft("two"), nil, nil)
	if bad != nil {
		t.Fatalf("the starter's own profile was refused: %s", bad.Text)
	}

	// 3. The same rewrite on the saved definition is refused too.
	if _, bad := scope.SaveResource(two, ResourceEdit{Ops: rewrite, Hash: saved.Hash}); bad == nil {
		t.Error("an existing definition's edit took the profile the new one refuses")
	}
}
