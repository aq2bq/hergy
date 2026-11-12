package ops

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/ops/internal/catalog"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/source"
)

func TestResourceSavingSequenceAcrossKinds(t *testing.T) {
	for _, kind := range []domainplace.Kind{domainplace.Definition, domainplace.Role, domainplace.Profile} {
		for _, tc := range []struct {
			name, stop string
			expected   []string
		}{
			{"success", "", []string{"read", "edit", "check", "write", "after"}},
			{"unreadable", "read", []string{"read"}},
			{"conflict", "hash", []string{"read"}},
			{"edit refusal", "edit", []string{"read", "edit"}},
			{"validation refusal", "check", []string{"read", "edit", "check"}},
			{"conditional write refusal", "write", []string{"read", "edit", "check", "write"}},
		} {
			t.Run(kindLabel(kind)+"/"+tc.name, func(t *testing.T) {
				var steps []string
				failure := &ResourceFailure{Text: "refused"}
				stage := func(name string) *ResourceFailure {
					steps = append(steps, name)
					if tc.stop == name {
						return failure
					}
					return nil
				}
				d := resourceDefinition{
					read: func() (ResourceFile, *ResourceFailure) {
						return ResourceFile{Path: "target", Text: "before", Hash: source.Hash("before"), Exists: true}, stage("read")
					},
					edit: func(before ResourceFile) (string, *ResourceFailure) {
						if before.Text != "before" {
							t.Fatal("wrong file")
						}
						return "after", stage("edit")
					},
					check: func(before ResourceFile, text string) *ResourceFailure {
						if before.Text != "before" || text != "after" {
							t.Fatal("wrong validation inputs")
						}
						return stage("check")
					},
					write: func(before ResourceFile, text string) *ResourceFailure {
						if before.Hash != source.Hash("before") || text != "after" {
							t.Fatal("wrong conditional write inputs")
						}
						return stage("write")
					},
					after: func() *ResourceFailure { return stage("after") },
				}
				hash := source.Hash("before")
				if tc.stop == "hash" {
					hash = "stale"
				}
				got, bad := saveResource(hash, false, d)
				if !reflect.DeepEqual(steps, tc.expected) {
					t.Fatalf("steps=%v want=%v", steps, tc.expected)
				}
				if tc.stop == "" {
					if bad != nil || got.Hash != source.Hash("after") {
						t.Fatalf("result=%+v bad=%v", got, bad)
					}
				} else if bad == nil {
					t.Fatal("refusal was lost")
				}
			})
		}
	}
}
func kindLabel(kind domainplace.Kind) string {
	switch kind {
	case domainplace.Definition:
		return "definition"
	case domainplace.Role:
		return "role"
	default:
		return "profile"
	}
}

func TestResourceReferenceWriteFailureListsCompletedAndPending(t *testing.T) {
	refs := []resourceRewrite{{path: "first"}, {path: "second"}}
	bad := referenceWriteFailure(errors.New("write refused"), refs, 1, "second")
	if !bad.Internal || bad.Text == "" {
		t.Fatalf("%+v", bad)
	}
	// The two scopes are distinct even if the same filesystem error is returned.
	done := referenceWriteFailure(errors.New("rename refused"), refs, 2, "resource")
	if done.Text == bad.Text {
		t.Fatal("completed references were reported as pending")
	}
}

func TestResourceTargetsKeepObservedNamesSeparateFromDestinations(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_STATE_HOME", dir)
	project, err := domainplace.NewProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	at := domainplace.In(project)
	observed, err := ObserveResource(domainplace.Role, at, "Upper.old")
	if err != nil {
		t.Fatal(err)
	}
	if observed.Name() != "Upper.old" {
		t.Fatal("existing name was rewritten")
	}
	if _, err := NewResourceRef(domainplace.Role, at, observed.Name()); err == nil {
		t.Fatal("observed old name became a new destination")
	}
	for _, n := range []string{"", ".", "..", "a/b", "a\\b"} {
		if _, err := ObserveResource(domainplace.Role, at, n); err == nil {
			t.Fatalf("unsafe component accepted: %q", n)
		}
	}
	cat := catalog.New(dir, filepath.Join(dir, ".hergy"), project, source.Disk{}, func(p string) string { return p })
	next, _ := NewResourceRef(domainplace.Role, at, "valid")
	other, _ := ObserveResource(domainplace.Profile, at, "old")
	if _, bad := (ResourceScope{Catalog: cat}).SaveResource(next, ResourceEdit{Original: &other}); bad == nil {
		t.Fatal("a role save accepted a profile source")
	}
}

func TestResourceReferencesStopBeforeTheFirstUnwritableReference(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.yaml")
	second := filepath.Join(dir, "second.yaml")
	if err := os.WriteFile(first, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("changed elsewhere"), 0600); err != nil {
		t.Fatal(err)
	}
	bad := writeResourceReferences([]resourceRewrite{{path: first, before: "before", after: "after"}, {path: second, before: "before", after: "after"}})
	if bad == nil || !bad.Internal {
		t.Fatalf("%+v", bad)
	}
	if !strings.Contains(bad.Text, first) || !strings.Contains(bad.Text, second) {
		t.Fatalf("missing completed/pending reference: %s", bad.Text)
	}
	firstText, _ := os.ReadFile(first)
	secondText, _ := os.ReadFile(second)
	if string(firstText) != "after" || string(secondText) != "changed elsewhere" {
		t.Fatalf("first=%q second=%q", firstText, secondText)
	}
}
