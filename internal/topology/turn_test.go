package topology

import (
	"strings"
	"testing"
)

// edgeLines is each edge in a short form a table can hold: the rule id, the two
// ends, and the arrow between them.
func edgeLines(edges []Edge) []string {
	out := make([]string, len(edges))
	for i, e := range edges {
		arrow := string(arrowRight)
		if e.Both {
			arrow = string(arrowBoth)
		}
		out[i] = e.RuleID + " " + e.From.Key() + " " + arrow + " " + e.To.Key()
	}
	return out
}

// unreadLines is each unread arrow in a short form: the rule id, the words that
// matched nothing, and the text around the arrow.
func unreadLines(unread []Unread) []string {
	out := make([]string, len(unread))
	for i, u := range unread {
		out[i] = u.RuleID + " " + strings.Join(u.Missing, "|") + " " + u.Text
	}
	return out
}

// TestReadNames: the two ends are matched against the names the team knows —
// the member names, the role names and the human — as the longest name the
// token next to the arrow ends or starts with.
func TestReadNames(t *testing.T) {
	cases := []struct {
		name    string
		members []Member
		text    string
		edges   []string
		unread  []string
	}{
		{
			name:    "a name runs on into the sentence (Japanese)",
			members: []Member{{Name: "spec", Role: "要求定義"}},
			text:    "人間 → 要求定義である",
			edges:   []string{"C1 human → m:0"},
		},
		{
			name:    "a member name points at the member",
			members: []Member{{Name: "lead", Role: "進行管理"}},
			text:    "lead → 人間",
			edges:   []string{"C1 m:0 → human"},
		},
		{
			name:    "a role points at every member that reads it",
			members: []Member{{Name: "a", Role: "実装"}, {Name: "b", Role: "実装"}},
			text:    "実装 → 人間",
			edges:   []string{"C1 m:0 → human", "C1 m:1 → human"},
		},
		{
			name:    "the longest name wins",
			members: []Member{{Name: "a", Role: "要求"}, {Name: "b", Role: "要求定義"}},
			text:    "要求定義 → 人間",
			edges:   []string{"C1 m:1 → human"},
		},
		{
			name:    "a member name wins a tie with a role name",
			members: []Member{{Name: "abc", Role: "z"}, {Name: "q", Role: "abc"}},
			text:    "abc → 人間",
			edges:   []string{"C1 m:0 → human"},
		},
		{
			name:    "bold does not stand in the way",
			members: []Member{{Name: "a", Role: "実装"}},
			text:    "**実装** → 人間",
			edges:   []string{"C1 m:0 → human"},
		},
		{
			name:    "an English file says human",
			members: []Member{{Name: "lead", Role: "design"}},
			text:    "human → lead",
			edges:   []string{"C1 human → m:0"},
		},
		{
			name:    "a name away from the arrow matches nothing",
			members: []Member{{Name: "a", Role: "実装"}},
			text:    "実装の話 → 人間",
			edges:   []string{"C1 s:実装の話 → human"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Read(c.members, []Doc{{Key: "d", Rules: []Rule{{ID: "C1", Text: c.text}}}})
			if !equal(edgeLines(got.Edges), c.edges) {
				t.Fatalf("edges = %v, want %v", edgeLines(got.Edges), c.edges)
			}
			if !equal(unreadLines(got.Unread), c.unread) {
				t.Fatalf("unread = %v, want %v", unreadLines(got.Unread), c.unread)
			}
		})
	}
}

// TestReadOneSidedArrows: an arrow whose other side matches no team name is
// drawn dotted, to a party named by the written word.
func TestReadOneSidedArrows(t *testing.T) {
	members := []Member{{Name: "lead", Role: "進行管理"}}
	cases := []struct {
		name  string
		text  string
		edges []string
	}{
		{"the right side matches nothing", "人間 → 書いた担当", []string{"C1 human → s:書いた担当"}},
		{"the left side matches nothing", "書いた担当 → 人間", []string{"C1 s:書いた担当 → human"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Read(members, []Doc{{Key: "d", Rules: []Rule{{ID: "C1", Text: c.text}}}})
			if !equal(edgeLines(got.Edges), c.edges) {
				t.Fatalf("edges = %v, want %v", edgeLines(got.Edges), c.edges)
			}
			if len(got.Unread) != 0 {
				t.Fatalf("unread = %v, want none", unreadLines(got.Unread))
			}
		})
	}
}

// TestReadUnreadArrows: an arrow that names no member on either side is not
// drawn and is reported, so nothing is dropped silently.
func TestReadUnreadArrows(t *testing.T) {
	members := []Member{{Name: "lead", Role: "進行管理"}}
	cases := []struct {
		name   string
		text   string
		unread []string
	}{
		{
			name:   "neither side is a team name",
			text:   "判明した担当 → 振り出した担当",
			unread: []string{"C1 判明した担当|振り出した担当 判明した担当 → 振り出した担当"},
		},
		{
			name:   "the left side is empty",
			text:   "→ 人間",
			unread: []string{"C1 人間 → 人間"},
		},
		{
			name:   "the right side is empty",
			text:   "人間 →",
			unread: []string{"C1 人間 人間 →"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Read(members, []Doc{{Key: "d", Rules: []Rule{{ID: "C1", Text: c.text}}}})
			if len(got.Edges) != 0 {
				t.Fatalf("edges = %v, want none", edgeLines(got.Edges))
			}
			if !equal(unreadLines(got.Unread), c.unread) {
				t.Fatalf("unread = %v, want %v", unreadLines(got.Unread), c.unread)
			}
		})
	}
}

// TestReadArrows: ⇄ keeps both directions, an arrow pointing at itself is left
// out, and a repeated arrow is read once.
func TestReadArrows(t *testing.T) {
	members := []Member{{Name: "a", Role: "設計"}, {Name: "b", Role: "実装"}}
	cases := []struct {
		name  string
		text  string
		edges []string
	}{
		{"a one-way arrow", "設計 → 実装", []string{"C1 m:0 → m:1"}},
		{"both ways", "設計 ⇄ 実装", []string{"C1 m:0 ⇄ m:1"}},
		{"an arrow pointing at itself", "実装 → 実装", nil},
		{"a repeated arrow", "人間 → 設計 人間 → 設計", []string{"C1 human → m:0"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Read(members, []Doc{{Key: "d", Rules: []Rule{{ID: "C1", Text: c.text}}}})
			if !equal(edgeLines(got.Edges), c.edges) {
				t.Fatalf("edges = %v, want %v", edgeLines(got.Edges), c.edges)
			}
		})
	}
}

// TestReadFollowsTheArrowNotItsBold: the same arrow written both ways reads the
// same.
func TestReadBoth(t *testing.T) {
	members := []Member{{Name: "a", Role: "設計"}, {Name: "b", Role: "実装"}}
	one := Read(members, []Doc{{Key: "d", Rules: []Rule{{ID: "C1", Text: "設計 → 実装"}}}})
	both := Read(members, []Doc{{Key: "d", Rules: []Rule{{ID: "C1", Text: "設計 ⇄ 実装"}}}})
	if one.Edges[0].Both {
		t.Fatal("a one-way arrow came back as both ways")
	}
	if !both.Edges[0].Both {
		t.Fatal("a both-ways arrow came back as one way")
	}
}

// TestReadEveryDocument: the role files are read with the shared documents, and
// a rule in them carries its document's key.
func TestReadEveryDocument(t *testing.T) {
	members := []Member{{Name: "a", Role: "設計"}, {Name: "b", Role: "実装"}}
	docs := []Doc{
		{Key: "shared", Label: "Agreement", Rules: []Rule{{ID: "C1", Text: "人間 → 設計"}}},
		{Key: "role/実装", Label: "実装", Role: "実装", Rules: []Rule{{ID: "R1", Text: "設計 → 実装"}}},
	}
	got := Read(members, docs)
	want := []string{"C1 human → m:0", "R1 m:0 → m:1"}
	if !equal(edgeLines(got.Edges), want) {
		t.Fatalf("edges = %v, want %v", edgeLines(got.Edges), want)
	}
	if got.Edges[1].DocKey != "role/実装" {
		t.Fatalf("the role file's edge carries document %q, want %q", got.Edges[1].DocKey, "role/実装")
	}
}

// TestSideKey: the three kinds of side are told apart by their key.
func TestSideKey(t *testing.T) {
	cases := []struct {
		side Side
		want string
	}{
		{Side{Kind: SideMember, Member: 3}, "m:3"},
		{Side{Kind: SideHuman}, "human"},
		{Side{Kind: SideStranger, Word: "書いた担当"}, "s:書いた担当"},
	}
	for _, c := range cases {
		if got := c.side.Key(); got != c.want {
			t.Fatalf("Key() = %q, want %q", got, c.want)
		}
	}
}

// TestTokenCuts: the word next to an arrow stops at whitespace, punctuation and
// symbols, and never crosses another arrow.
func TestTokenCuts(t *testing.T) {
	text := "まず 要求定義 → 実装、次に A ⇄ B"
	at := strings.IndexRune(text, arrowRight)
	after := at + len(string(arrowRight))
	if got, want := tokenBefore(text, at), "要求定義"; got != want {
		t.Fatalf("tokenBefore = %q, want %q", got, want)
	}
	if got, want := tokenAfter(text, after), "実装"; got != want {
		t.Fatalf("tokenAfter = %q, want %q", got, want)
	}
	// The token before the second arrow does not run back over the first one.
	second := strings.LastIndex(text, string(arrowBoth))
	if got, want := tokenBefore(text, second), "A"; got != want {
		t.Fatalf("tokenBefore the second arrow = %q, want %q", got, want)
	}
}

// equal compares two string lists, treating nil and empty alike.
func equal(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
