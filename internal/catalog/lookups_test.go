package catalog

import (
	"path/filepath"
	"testing"

	"github.com/aq2bq/hergy/internal/source"
)

// refs is the refs of one definition written as "definition/member[/purpose]",
// so a test says what it wants in one line.
func refs(list []MemberRef) []string {
	out := make([]string, 0, len(list))
	for _, ref := range list {
		one := ref.Definition + "/" + ref.Member
		if ref.Purpose != "" {
			one += "/" + ref.Purpose
		}
		out = append(out, one)
	}
	return out
}

func sameRefs(t *testing.T, what string, got []MemberRef, want ...string) {
	t.Helper()
	have := refs(got)
	if len(have) != len(want) {
		t.Fatalf("%s = %v, want %v", what, have, want)
	}
	for i := range want {
		if have[i] != want[i] {
			t.Fatalf("%s = %v, want %v", what, have, want)
		}
	}
}

// TestReadersOfARoleDocument is who reads one role document: the members that
// play the role (C8).
func TestReadersOfARoleDocument(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerLocal, "t", "m")
	cat := l.catalog()

	lead := filepath.Join(l.local, "roles", "lead-role.yaml")
	sameRefs(t, "readers of the lead's role document", cat.Readers(lead), "t/a")
	impl := filepath.Join(l.local, "roles", "impl-role.yaml")
	sameRefs(t, "readers of the impl's role document", cat.Readers(impl), "t/b")
	sameRefs(t, "readers of a document nobody reads", cat.Readers(filepath.Join(l.local, "roles", "absent.yaml")))
}

// TestReadersOfAnAgreement: a team's own agreement is read by that team's
// members alone (C8). The agreement every team used to share is read by nobody
// now, so it names nobody (d-31ad).
func TestReadersOfAnAgreement(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerLocal, "t", "m")
	l.writeFullTeam(t, source.LayerGlobal, "other", "another team")
	shared := filepath.Join(l.local, "agreement.yaml")
	write(t, shared, "the agreement every team shares\n")
	cat := l.catalog()

	sameRefs(t, "readers of the shared agreement", cat.Readers(shared))

	own := filepath.Join(l.local, "teams", "t", "agreement.yaml")
	sameRefs(t, "readers of the team's own agreement", cat.Readers(own), "t/a", "t/b")
}

// TestReadersAcrossDefinitions: a role document is the layer's (K5), so the same
// role played in two definitions is read in both (C8).
func TestReadersAcrossDefinitions(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerLocal, "t", "m")
	l.writeFullTeam(t, source.LayerLocal, "second", "another description")
	cat := l.catalog()

	sameRefs(t, "readers of the shared role document",
		cat.Readers(filepath.Join(l.local, "roles", "lead-role.yaml")), "second/a", "t/a")
	sameRefs(t, "role users", cat.RoleUsers("lead-role"), "second/a", "t/a")
}

// TestReadersLeaveOutADefinitionThatDoesNotRead: what cannot be read has no
// members to name (C8).
func TestReadersLeaveOutADefinitionThatDoesNotRead(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerLocal, "t", "m")
	write(t, filepath.Join(l.local, "teams", "broken", "team.yaml"), "version: 1\ndescription:\nmembers: []\n")
	cat := l.catalog()

	sameRefs(t, "readers of the shared role document",
		cat.Readers(filepath.Join(l.local, "roles", "lead-role.yaml")), "t/a")
	sameRefs(t, "role users", cat.RoleUsers("lead-role"), "t/a")
}

// TestProfileUsersTellsMembersFromWorkers: one profile is named by members that
// are started with it and by workers that are hired with it, and the two are
// told apart (C8, I7, I12c).
func TestProfileUsersTellsMembersFromWorkers(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerLocal, "t", "m")
	cat := l.catalog()

	members, workers := cat.ProfileUsers("builder")
	sameRefs(t, "members that run builder", members, "t/b")
	sameRefs(t, "workers that run builder", workers)

	members, workers = cat.ProfileUsers("researcher")
	sameRefs(t, "members that run researcher", members)
	sameRefs(t, "workers that run researcher", workers, "t/b/調査")

	members, workers = cat.ProfileUsers("nobody")
	sameRefs(t, "members that run nobody", members)
	sameRefs(t, "workers that run nobody", workers)
}
