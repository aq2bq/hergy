package topology

import (
	"testing"
)

// sampleMembers is a small team: a lead with a role file, and a builder that may
// hire one worker.
func sampleMembers() []Member {
	return []Member{
		{Name: "lead", Role: "進行管理"},
		{Name: "impl", Role: "設計実装", Workers: []Worker{{Purpose: "調査", Profile: "researcher"}}},
	}
}

// sampleDocs is a shared document and one role file per role. Its rules still
// write turn directions; the figure no longer reads them (段 5).
func sampleDocs() []Doc {
	return []Doc{
		{Key: "shared", Label: "合意", Rules: []Rule{
			{ID: "C1", Text: "人間 → lead"},
			{ID: "C2", Text: "人間 → 書いた担当"},
		}},
		{Key: "role/進行管理", Label: "進行管理", Role: "進行管理", Rules: []Rule{
			{ID: "P1", Text: "lead → impl"},
		}},
		{Key: "role/設計実装", Label: "設計実装", Role: "設計実装", Rules: []Rule{
			{ID: "D1", Text: "進行管理 ⇄ 人間"},
		}},
	}
}

// sampleTurns is the graph the figure draws: the human and the lead hand the
// turn to each other, and lead hands it to impl.
func sampleTurns() []Turn {
	return []Turn{
		{From: Side{Kind: SideHuman}, To: Side{Kind: SideMember, Member: 0}},
		{From: Side{Kind: SideMember, Member: 0}, To: Side{Kind: SideHuman}},
		{From: Side{Kind: SideMember, Member: 0}, To: Side{Kind: SideMember, Member: 1}},
		{From: Side{Kind: SideMember, Member: 1}, To: Side{Kind: SideMember, Member: 0}},
	}
}

// TestLayoutBoxes: the figure keeps its parts in order and inside its bounds.
func TestLayoutBoxes(t *testing.T) {
	f := Layout(sampleMembers(), sampleDocs(), sampleTurns())

	if f.Human.Y >= f.Members[0].Y {
		t.Fatalf("the human is not above the members: human y %v, member y %v", f.Human.Y, f.Members[0].Y)
	}
	for i := 1; i < len(f.Members); i++ {
		prev, cur := f.Members[i-1], f.Members[i]
		if cur.Y < prev.Y+prev.H {
			t.Fatalf("member %d overlaps member %d: %v then %v", i, i-1, prev.Box, cur.Box)
		}
	}
	if len(f.Docs) != 1 || f.Docs[0].Key != "shared" {
		t.Fatalf("the shared band holds %d docs, want just the shared one: %+v", len(f.Docs), f.Docs)
	}
	doc := f.Docs[0]
	if doc.X < f.Shared.X || doc.Y < f.Shared.Y || doc.Y+doc.H > f.Shared.Y+f.Shared.H {
		t.Fatalf("the document %v is not inside the band %v", doc.Box, f.Shared)
	}
	if f.Width <= 0 || f.Height <= 0 {
		t.Fatalf("the figure has no size: %v x %v", f.Width, f.Height)
	}
	for _, b := range allBoxes(f) {
		if b.X < 0 || b.Y < 0 || b.X+b.W > f.Width || b.Y+b.H > f.Height {
			t.Fatalf("a box %v falls outside the figure %v x %v", b, f.Width, f.Height)
		}
	}
}

// allBoxes is every box the figure draws, for a bounds check.
func allBoxes(f Figure) []Box {
	out := []Box{f.Shared, f.Human}
	for _, doc := range f.Docs {
		out = append(out, doc.Box)
	}
	for _, member := range f.Members {
		out = append(out, member.Box)
		if member.RoleFile != nil {
			out = append(out, *member.RoleFile)
		}
		for _, worker := range member.Workers {
			out = append(out, worker.Box)
		}
	}
	return out
}

// TestLayoutRoleFilesAndWorkers: a member carries the role file of its role, and
// the workers it may hire sit under it.
func TestLayoutRoleFilesAndWorkers(t *testing.T) {
	f := Layout(sampleMembers(), sampleDocs(), nil)

	lead, impl := f.Members[0], f.Members[1]
	if lead.RoleFile == nil || lead.RoleFileKey != "role/進行管理" {
		t.Fatalf("the lead's role file = %v (%q), want the 進行管理 file", lead.RoleFile, lead.RoleFileKey)
	}
	if impl.RoleFile == nil || impl.RoleFileKey != "role/設計実装" {
		t.Fatalf("impl's role file = %v (%q), want the 設計実装 file", impl.RoleFile, impl.RoleFileKey)
	}
	if len(lead.Workers) != 0 {
		t.Fatalf("the lead hired %d workers, want none", len(lead.Workers))
	}
	if len(impl.Workers) != 1 {
		t.Fatalf("impl has %d worker boxes, want 1", len(impl.Workers))
	}
	worker := impl.Workers[0]
	if worker.Purpose != "調査" || worker.Profile != "researcher" {
		t.Fatalf("the worker box = %+v, want 調査 / researcher", worker)
	}
	if worker.Y < impl.Y+impl.H {
		t.Fatalf("the worker box %v is not under impl %v", worker.Box, impl.Box)
	}
	if worker.X+worker.W > impl.X+impl.W {
		t.Fatalf("the worker box %v sticks out of impl %v", worker.Box, impl.Box)
	}
	if lead.RoleFile != nil && (lead.RoleFile.X != roleX || lead.RoleFile.W != roleW) {
		t.Fatalf("the role file box is at %v, want the role column at x %v", lead.RoleFile, roleX)
	}
}

// TestLayoutNoRoleFile: a role with no file gets no role file box.
func TestLayoutNoRoleFile(t *testing.T) {
	f := Layout([]Member{{Name: "solo", Role: "設計"}}, nil, nil)
	if f.Members[0].RoleFile != nil {
		t.Fatalf("a role with no file got the box %v", f.Members[0].RoleFile)
	}
}

// TestLayoutArrowsMeetTheirRows: every arrow meets both its rows, inside them.
func TestLayoutArrowsMeetTheirRows(t *testing.T) {
	f := Layout(sampleMembers(), sampleDocs(), sampleTurns())
	if len(f.Edges) == 0 {
		t.Fatal("the figure has no arrows")
	}
	if len(f.Edges) != len(sampleTurns()) {
		t.Fatalf("the figure draws %d arrows, want the %d turns", len(f.Edges), len(sampleTurns()))
	}
	rows := func(side Side) (Box, bool) {
		switch side.Kind {
		case SideHuman:
			return f.Human, true
		case SideMember:
			if side.Member < 0 || side.Member >= len(f.Members) {
				return Box{}, false
			}
			member := f.Members[side.Member]
			// A member's row grows by the workers under it.
			h := member.H
			if len(member.Workers) > 0 {
				h += workerStep
			}
			return Box{X: member.X, Y: member.Y, W: member.W, H: h}, true
		case SideStranger:
			// The sample has no stranger; a side the figure has no row for is
			// reported as one.
		}
		return Box{}, false
	}
	for _, arrow := range f.Edges {
		from, ok := rows(arrow.From)
		if !ok {
			t.Fatalf("arrow %v points from a party with no row", arrow.From)
		}
		to, ok := rows(arrow.To)
		if !ok {
			t.Fatalf("arrow %v points to a party with no row", arrow.To)
		}
		if !within(arrow.FromY, from) || !within(arrow.ToY, to) {
			t.Fatalf("arrow %v meets %v at %v and %v at %v", arrow.Turn, from, arrow.FromY, to, arrow.ToY)
		}
	}
}

// within reports whether y is inside the box's vertical span.
func within(y float64, b Box) bool {
	return y >= b.Y && y <= b.Y+b.H
}

// TestLayoutDrawsTheGraphOnly: a turn that names a member the figure does not
// have is not drawn. The definition's turns: is what the figure draws, and
// nothing the rules' text says changes it (段 5).
func TestLayoutDrawsTheGraphOnly(t *testing.T) {
	turns := []Turn{
		{From: Side{Kind: SideHuman}, To: Side{Kind: SideMember, Member: 0}},
		{From: Side{Kind: SideMember, Member: 0}, To: Side{Kind: SideMember, Member: 7}},
	}
	f := Layout(sampleMembers(), sampleDocs(), turns)
	if len(f.Edges) != 1 {
		t.Fatalf("the figure draws %d arrows, want only the one whose member is there", len(f.Edges))
	}
}

// TestLayoutLanesSeparate: each arrow gets its own lane to the right of the
// members, so two arrows do not share one.
func TestLayoutLanesSeparate(t *testing.T) {
	f := Layout(sampleMembers(), sampleDocs(), sampleTurns())
	seen := map[float64]bool{}
	right := memberX + memberW
	for _, arrow := range f.Edges {
		if arrow.LaneX <= right {
			t.Fatalf("arrow %v sits at x %v, not right of the members (%v)", arrow.Turn, arrow.LaneX, right)
		}
		if seen[arrow.LaneX] {
			t.Fatalf("two arrows share lane x %v", arrow.LaneX)
		}
		seen[arrow.LaneX] = true
	}
}

// TestLayoutDeterministic: the same definition lays out the same way every time.
func TestLayoutDeterministic(t *testing.T) {
	a := Layout(sampleMembers(), sampleDocs(), sampleTurns())
	b := Layout(sampleMembers(), sampleDocs(), sampleTurns())
	if !reflectEqual(t, a, b) {
		t.Fatal("two layouts of one definition differ")
	}
}

// TestLayoutNoMembers: a definition with no members still lays out, with only
// the human row.
func TestLayoutNoMembers(t *testing.T) {
	f := Layout(nil, nil, nil)
	if len(f.Members) != 0 || len(f.Edges) != 0 {
		t.Fatalf("an empty team laid out %d members and %d arrows", len(f.Members), len(f.Edges))
	}
	if f.Width <= 0 || f.Height <= 0 {
		t.Fatalf("the empty figure has no size: %v x %v", f.Width, f.Height)
	}
}

// TestBoxCenter: a box knows its center.
func TestBoxCenter(t *testing.T) {
	b := Box{X: 10, Y: 20, W: 30, H: 40}
	if got, want := b.CX(), 25.0; got != want {
		t.Fatalf("CX = %v, want %v", got, want)
	}
	if got, want := b.CY(), 40.0; got != want {
		t.Fatalf("CY = %v, want %v", got, want)
	}
}

// reflectEqual compares two figures field by field, without a reflect import, so
// the test says which part moved.
func reflectEqual(t *testing.T, a, b Figure) bool {
	t.Helper()
	if a.Width != b.Width || a.Height != b.Height || a.Shared != b.Shared || a.Human != b.Human {
		return false
	}
	if len(a.Docs) != len(b.Docs) || len(a.Members) != len(b.Members) || len(a.Edges) != len(b.Edges) {
		return false
	}
	for i := range a.Members {
		if !sameMember(a.Members[i], b.Members[i]) {
			return false
		}
	}
	for i := range a.Edges {
		if a.Edges[i] != b.Edges[i] {
			return false
		}
	}
	return true
}

func sameMember(a, b MemberBox) bool {
	if a.Box != b.Box || a.Index != b.Index || a.Name != b.Name || a.Role != b.Role ||
		a.RoleFileKey != b.RoleFileKey || len(a.Workers) != len(b.Workers) {
		return false
	}
	if (a.RoleFile == nil) != (b.RoleFile == nil) {
		return false
	}
	if a.RoleFile != nil && *a.RoleFile != *b.RoleFile {
		return false
	}
	for i := range a.Workers {
		if a.Workers[i] != b.Workers[i] {
			return false
		}
	}
	return true
}
