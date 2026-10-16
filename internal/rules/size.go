package rules

import (
	"strings"
	"unicode/utf8"
)

// The size of what a team hands its members, and of the files a person keeps.
// `hergy validate` counts both and warns when one is over a limit.
//
// The limits are provisional (d-f2bc, 2026-10-16). The survey behind them found
// no measured basis for the numbers themselves: they are a working budget to
// review a team's rules against, not a bound on what a model can follow.
//
// Review them once hergy-dev, the team definition hergy develops itself with,
// has gone through ten changes: move them on the evidence of what failed and of
// how much human work went up or down, and write the reason to gy (d-f2bc).
const (
	// MemberRuleLimit is how many rules one member may be handed.
	MemberRuleLimit = 20
	// MemberCharLimit is how many characters one member may be handed.
	MemberCharLimit = 6000
	// TeamSourceLimit is how many rule files one team definition may read.
	TeamSourceLimit = 8
	// TeamSourceCharLimit is how many characters those files may hold.
	TeamSourceCharLimit = 20000
)

// MemberSize is what one member is handed: the rules of the documents that go
// into its prompt, and the characters of the Markdown hergy renders from them.
// The prompt's own frame (its template, and the roster and contact hergy adds)
// is not counted, and the same document read twice counts twice: it is handed
// over twice.
type MemberSize struct {
	Rules int
	Chars int
}

// Add counts one document of a member's prompt.
func (s *MemberSize) Add(doc *Document) {
	s.Rules += len(doc.Rules())
	s.Chars += CharCount(doc.Markdown())
}

// OverMemberRules says whether the rules this member is handed are over the
// provisional limit.
func (s MemberSize) OverMemberRules() bool { return s.Rules > MemberRuleLimit }

// OverMemberChars says whether the characters this member is handed are over the
// provisional limit.
func (s MemberSize) OverMemberChars() bool { return s.Chars > MemberCharLimit }

// TeamSize is what one team definition keeps: the rule files its members read. A
// person maintains these files, so one that two members read is counted once.
type TeamSize struct {
	Sources int
	Chars   int
	seen    map[string]bool
}

// AddSource counts one rule file of a team, once, from the text as it is on
// disk.
func (t *TeamSize) AddSource(path, text string) {
	if t.seen == nil {
		t.seen = map[string]bool{}
	}
	if t.seen[path] {
		return
	}
	t.seen[path] = true
	t.Sources++
	t.Chars += CharCount(text)
}

// OverTeamSources says whether the team reads more rule files than the
// provisional limit.
func (t TeamSize) OverTeamSources() bool { return t.Sources > TeamSourceLimit }

// OverTeamChars says whether the team's rule files hold more characters than the
// provisional limit.
func (t TeamSize) OverTeamChars() bool { return t.Chars > TeamSourceCharLimit }

// CharCount is the characters of a text in Unicode code points, with CRLF
// brought to LF first so a file written on Windows counts the same as one
// written here. Whitespace, line breaks, headings and URLs are counted.
func CharCount(text string) int {
	return utf8.RuneCountInString(strings.ReplaceAll(text, "\r\n", "\n"))
}
