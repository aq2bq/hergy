// Package topology lays out one team definition as the figure the WebUI draws,
// and reads the turn directions a figure's arrows come from. It reads no files
// and touches no Herdr: the caller hands it the members and the rule files, and
// it returns boxes, arrows and the arrows it could not read. The screen draws
// what this computes.
//
// A turn direction is written in a rule's text as "<name> → <name>" or
// "<name> ⇄ <name>" (d-0f08, d-23bf). The names are matched against the names
// the team knows — the member names, the role names, and the human — and never
// split out as words, because a name runs on into the sentence around it
// ("人間 → 要求定義である").
package topology

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Member is one member the figure draws: the short name written in team.yaml,
// the role it reads, and the workers it may hire.
type Member struct {
	Name    string
	Role    string
	Workers []Worker
}

// Worker is one worker a member may hire: what it is for, and the profile it
// runs.
type Worker struct {
	Purpose string
	Profile string
}

// Rule is one rule of a rule file: its id, and the text a member reads.
type Rule struct {
	ID   string
	Text string
}

// Doc is one rule file. Role is empty for a document every member reads, and the
// role whose file it is otherwise. Key is a stable identity the screen links to
// and uses to read a file once, however many members read it.
type Doc struct {
	Key   string
	Label string
	Role  string
	Rules []Rule
}

// SideKind says what one end of a turn arrow is.
type SideKind int

const (
	// SideMember is a member of the team definition: Member indexes Members.
	SideMember SideKind = iota
	// SideHuman is the human reading the rules.
	SideHuman
	// SideStranger is a name that matched no member of the team: Word is the
	// word as the rule wrote it.
	SideStranger
)

// Side is one end of a turn arrow.
type Side struct {
	Kind   SideKind
	Member int    // the index into the members slice, when Kind is SideMember
	Word   string // the written word, when Kind is SideStranger
}

// Edge is one turn direction read from a rule's text: A → B, or A ⇄ B when Both.
type Edge struct {
	RuleID string
	DocKey string
	From   Side
	To     Side
	Both   bool
}

// Unread is an arrow that named no member of the team on either side. It is not
// drawn; a figure lists it so nothing is dropped silently.
type Unread struct {
	RuleID  string
	DocKey  string
	Missing []string // the words as written, one per side
	Text    string   // the part of the rule's text around the arrow
}

// Reading is what the rule files say about turn directions.
type Reading struct {
	Edges  []Edge
	Unread []Unread
}

// The two arrows a turn direction is written with.
const (
	arrowRight = '→'
	arrowBoth  = '⇄'
)

// Read reads the turn directions from the rule files the members read. A rule's
// why and note are never read: a member reads the text alone (d-0f08).
func Read(members []Member, docs []Doc) Reading {
	var r Reading
	seen := map[string]bool{}
	for _, doc := range docs {
		for _, rule := range doc.Rules {
			readRule(&r, members, doc, rule, seen)
		}
	}
	return r
}

// readRule reads every arrow of one rule's text and files the result.
func readRule(r *Reading, members []Member, doc Doc, rule Rule, seen map[string]bool) {
	text := strings.ReplaceAll(rule.Text, "**", "")
	for _, spot := range arrows(text) {
		left := tokenBefore(text, spot.pos)
		right := tokenAfter(text, spot.after)
		// An arrow with nothing on one side names no party there.
		if left == "" || right == "" {
			r.Unread = append(r.Unread, Unread{
				RuleID: rule.ID, DocKey: doc.Key, Missing: missing(left, right),
				Text: arrowText(left, spot.arrow, right),
			})
			continue
		}
		from := match(members, text, left, true)
		to := match(members, text, right, false)
		switch {
		case from != nil && to != nil:
			addEdges(r, seen, doc, rule, spot, from, to)
		case from != nil:
			addEdges(r, seen, doc, rule, spot, from, []Side{{Kind: SideStranger, Word: right}})
		case to != nil:
			addEdges(r, seen, doc, rule, spot, []Side{{Kind: SideStranger, Word: left}}, to)
		default:
			r.Unread = append(r.Unread, Unread{
				RuleID: rule.ID, DocKey: doc.Key, Missing: missing(left, right),
				Text: arrowText(left, spot.arrow, right),
			})
		}
	}
}

// addEdges adds one arrow for every from-to pair, skipping a party that points
// at itself and a pair already added for this rule.
func addEdges(r *Reading, seen map[string]bool, doc Doc, rule Rule, spot arrowSpot, from, to []Side) {
	for _, f := range from {
		for _, g := range to {
			if f.Key() == g.Key() {
				continue
			}
			key := f.Key() + ">" + g.Key() + ">" + doc.Key + "#" + rule.ID
			if seen[key] {
				continue
			}
			seen[key] = true
			r.Edges = append(r.Edges, Edge{
				RuleID: rule.ID, DocKey: doc.Key, From: f, To: g, Both: spot.arrow == arrowBoth,
			})
		}
	}
}

// missing is the words of an arrow that matched nothing, in the written order.
func missing(from, to string) []string {
	var out []string
	for _, word := range []string{from, to} {
		if word != "" {
			out = append(out, word)
		}
	}
	return out
}

// match is the sides a token names: the longest known name the token ends with
// (left) or starts with (right), and the sides that name stands for. A role
// stands for every member that reads it. Nothing matching gives nil, and the
// caller falls back to the written word.
func match(members []Member, text, token string, left bool) []Side {
	if token == "" {
		return nil
	}
	var best *name
	for _, candidate := range known(members, text) {
		if !candidate.hit(token, left) {
			continue
		}
		if best == nil || len(candidate.text) > len(best.text) {
			held := candidate
			best = &held
		}
	}
	if best == nil {
		return nil
	}
	return best.sides
}

// name is one known name and the sides it stands for, in match order.
type name struct {
	text  string
	sides []Side
}

// hit reports whether this name ends (left) or starts (right) the token.
func (n name) hit(token string, left bool) bool {
	if left {
		return strings.HasSuffix(token, n.text)
	}
	return strings.HasPrefix(token, n.text)
}

// known is the names the team knows, in the order they are tried: the member
// names, then the role names, then the human. The list is rebuilt per rule,
// because the human's word follows the file's language.
func known(members []Member, text string) []name {
	out := make([]name, 0, len(members)*2+1)
	for i, member := range members {
		out = append(out, name{text: member.Name, sides: []Side{{Kind: SideMember, Member: i}}})
	}
	out = append(out, roleNames(members)...)
	out = append(out, name{text: humanName(text), sides: []Side{{Kind: SideHuman}}})
	return out
}

// roleNames is one entry per role, in the order the roles first appear, each
// standing for every member that reads it.
func roleNames(members []Member) []name {
	index := map[string]int{}
	var out []name
	for i, member := range members {
		if member.Role == "" {
			continue
		}
		if at, ok := index[member.Role]; ok {
			out[at].sides = append(out[at].sides, Side{Kind: SideMember, Member: i})
			continue
		}
		index[member.Role] = len(out)
		out = append(out, name{text: member.Role, sides: []Side{{Kind: SideMember, Member: i}}})
	}
	return out
}

// humanName is the word a file uses for the human: 人間 in a file written with
// kanji, human in an English file.
func humanName(text string) string {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return "人間"
		}
	}
	return "human"
}

// Key is how one side is told apart from another: a member by index, the human,
// or a stranger by its word.
func (s Side) Key() string {
	switch s.Kind {
	case SideMember:
		return "m:" + strconv.Itoa(s.Member)
	case SideHuman:
		return "human"
	default:
		return "s:" + s.Word
	}
}

// arrowSpot is one arrow of a rule's text: where it starts, where it ends, and
// which arrow it is.
type arrowSpot struct {
	pos   int
	after int
	arrow rune
}

// arrows finds every turn arrow in a text, in order.
func arrows(text string) []arrowSpot {
	var out []arrowSpot
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == arrowRight || r == arrowBoth {
			out = append(out, arrowSpot{pos: i, after: i + size, arrow: r})
		}
		i += size
	}
	return out
}

// tokenBefore is the run just before the arrow: the whitespace next to the
// arrow is ignored, then the characters up to the first punctuation or symbol,
// so the word next to the arrow, not the whole sentence.
func tokenBefore(text string, stop int) string {
	end := stop
	for end > 0 {
		r, size := utf8.DecodeLastRuneInString(text[:end])
		if !unicode.IsSpace(r) {
			break
		}
		end -= size
	}
	start := end
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(text[:start])
		if separator(r) {
			break
		}
		start -= size
	}
	return text[start:end]
}

// tokenAfter is the run just after the arrow, by the same cut.
func tokenAfter(text string, start int) string {
	begin := start
	for begin < len(text) {
		r, size := utf8.DecodeRuneInString(text[begin:])
		if !unicode.IsSpace(r) {
			break
		}
		begin += size
	}
	end := begin
	for end < len(text) {
		r, size := utf8.DecodeRuneInString(text[end:])
		if separator(r) {
			break
		}
		end += size
	}
	return text[begin:end]
}

// separator ends a word: whitespace, punctuation, or a symbol (the arrows are
// symbols).
func separator(r rune) bool {
	return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
}

// arrowText is the part of a rule's text a person sees next to a dropped arrow:
// the words on either side and the arrow between them.
func arrowText(left string, arrow rune, right string) string {
	return strings.TrimSpace(strings.Join([]string{left, string(arrow), right}, " "))
}
