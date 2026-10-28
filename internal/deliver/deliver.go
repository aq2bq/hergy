// Package deliver says what changed for a running member since the prompt hergy
// last handed it, and names the rules, turns and workers that changed. It reads
// no herdr and no files: the caller hands it the old prompt and documents from
// the launch record and the current member's prompt, so the comparison can be
// tested without a running team.
package deliver

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
)

// Answer is what a screen reads about the running members whose prompt is not
// the one hergy last handed them. It is also what a send answers with: the
// people sent, and the count after.
type Answer struct {
	Count   int       `json:"count"`
	Teams   []Team    `json:"teams"`
	Sent    []string  `json:"sent,omitempty"`
	Refused []Refusal `json:"refused,omitempty"`
	Herdr   Note      `json:"herdr"`
}

// Team is one running team's stale members, in the order the activity lists
// them.
type Team struct {
	Name       string   `json:"name"`
	Definition string   `json:"definition"`
	Members    []Member `json:"members"`
}

// Member is one running member whose prompt differs from the one hergy last
// handed it. State is the activity's own word (idle, handed, active, blocked,
// absent); CanSend says the member is waiting, not working.
type Member struct {
	Name    string   `json:"name"`
	Short   string   `json:"short"`
	Role    string   `json:"role"`
	State   string   `json:"state"`
	CanSend bool     `json:"can_send"`
	Changes []Change `json:"changes"`
}

// Refusal is one person a send did not reach, and the sentence saying why.
type Refusal struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// Note is a reader that could not be read: whether it could, and the sentence a
// person reads when it could not.
type Note struct {
	OK   bool   `json:"ok"`
	Text string `json:"text"`
}

// Change is one thing a member would read differently. Kind is rule, turn,
// worker, roster, role, template or prompt. Document, Rule, Turn and Worker are
// the ids a screen may link to; Text is the sentence a person reads.
type Change struct {
	Kind     string `json:"kind"`
	Document string `json:"document,omitempty"`
	Rule     string `json:"rule,omitempty"`
	Turn     string `json:"turn,omitempty"`
	Worker   string `json:"worker,omitempty"`
	Text     string `json:"text"`
}

// Doc is one rule document as the comparison reads it. Path is the file's own
// path, Display the path a screen shows, and Text its contents.
type Doc struct {
	Path    string
	Display string
	Text    string
}

// Compare is what changed between the prompt a member last read and the prompt
// its current definition builds. oldDocs is the launch record's documents by
// path; current is the current Start (its prompt, segments, turns and workers)
// and docs are the current rule documents.
func Compare(oldPrompt string, oldDocs map[string]string, current definition.Start, docs []Doc) []Change {
	if oldPrompt == "" {
		return []Change{{Kind: "no_record", Text: i18n.T("delivery.no_record")}}
	}
	if oldPrompt == current.Prompt {
		return nil
	}
	var out []Change
	seenDoc := map[string]bool{}
	for _, segment := range current.Segments {
		if segment.Text == "" || strings.Contains(oldPrompt, segment.Text) {
			continue
		}
		switch segment.Kind {
		case "team_rules", "role_doc":
			doc, ok := findDoc(docs, segment.Origin.Path)
			if !ok {
				out = append(out, Change{Kind: "document", Text: i18n.Tf("delivery.document_changed", segment.Origin.Path)})
				continue
			}
			if seenDoc[doc.Path] {
				continue
			}
			seenDoc[doc.Path] = true
			changes := ruleChanges(doc, oldDocs[doc.Path])
			if len(changes) == 0 {
				changes = []Change{{Kind: "document", Document: doc.Display, Text: i18n.Tf("delivery.document_changed", displayOf(doc))}}
			}
			out = append(out, changes...)
		case "turns":
			out = append(out, turnChanges(current, segment.Text, oldPrompt)...)
		case "workers":
			out = append(out, workerChanges(current, segment.Text, oldPrompt)...)
		case "roster":
			out = append(out, Change{Kind: "roster", Text: i18n.T("delivery.roster_changed")})
		case "template":
			out = append(out, Change{Kind: "template", Text: i18n.T("delivery.template_changed")})
		default:
			out = append(out, Change{Kind: "prompt", Text: i18n.T("delivery.prompt_changed")})
		}
	}
	if len(out) == 0 {
		out = append(out, Change{Kind: "prompt", Text: i18n.T("delivery.prompt_changed")})
	}
	return dedupe(out)
}

// findDoc finds the document a prompt segment names: the display path first,
// then the path itself.
func findDoc(docs []Doc, path string) (Doc, bool) {
	for _, doc := range docs {
		if doc.Display == path || doc.Path == path {
			return doc, true
		}
	}
	for _, doc := range docs {
		if filepath.Base(doc.Path) == filepath.Base(path) {
			return doc, true
		}
	}
	return Doc{}, false
}

// displayOf is the document as a summary names it: its base name, so the
// sentence stays short while the Change keeps the whole path.
func displayOf(doc Doc) string { return filepath.Base(doc.Path) }

// ruleChanges compares the rules of one document by id.
func ruleChanges(doc Doc, oldText string) []Change {
	if oldText == "" {
		return nil
	}
	oldDoc, _ := rules.ReadAll(oldText, doc.Path)
	nowDoc, _ := rules.ReadAll(doc.Text, doc.Path)
	if oldDoc == nil || nowDoc == nil {
		return nil
	}
	oldRules := map[string]rules.Rule{}
	for _, rule := range oldDoc.Rules() {
		oldRules[rule.ID] = rule
	}
	nowRules := map[string]rules.Rule{}
	for _, rule := range nowDoc.Rules() {
		nowRules[rule.ID] = rule
	}
	name := displayOf(doc)
	var out []Change
	for _, rule := range nowDoc.Rules() {
		before, ok := oldRules[rule.ID]
		switch {
		case !ok:
			out = append(out, Change{Kind: "rule", Document: doc.Display, Rule: rule.ID,
				Text: i18n.Tf("delivery.rule_added", name, rule.ID)})
		case before.Text != rule.Text:
			out = append(out, Change{Kind: "rule", Document: doc.Display, Rule: rule.ID,
				Text: i18n.Tf("delivery.rule_changed", name, rule.ID)})
		case before.Included() != rule.Included():
			key := "delivery.rule_off"
			if rule.Included() {
				key = "delivery.rule_on"
			}
			out = append(out, Change{Kind: "rule", Document: doc.Display, Rule: rule.ID,
				Text: i18n.Tf(key, name, rule.ID)})
		}
	}
	for _, rule := range oldDoc.Rules() {
		if _, ok := nowRules[rule.ID]; !ok {
			out = append(out, Change{Kind: "rule", Document: doc.Display, Rule: rule.ID,
				Text: i18n.Tf("delivery.rule_removed", name, rule.ID)})
		}
	}
	return out
}

// turnChanges compares the turns a member hands to, by their endpoints. The
// current endpoints come from the definition (short member names and human);
// the old endpoints are read from the old prompt's turns section. The summary
// is the pair "lead → analyst", never a prompt line (r-a8fa audit 1 B).
func turnChanges(current definition.Start, segment, oldPrompt string) []Change {
	team := teamOf(current)
	now := turnNames(current.Turns, team)
	old := oldTurnNames(bulletLines(section(oldPrompt, firstHeading(segment))), team)
	var out []Change
	for _, to := range now {
		if !containsLine(old, to) {
			pair := turnPair(current.Short, to)
			out = append(out, Change{Kind: "turn", Turn: pair,
				Text: i18n.Tf("delivery.turn_added", pair)})
		}
	}
	for _, to := range old {
		if !containsLine(now, to) {
			pair := turnPair(current.Short, to)
			out = append(out, Change{Kind: "turn", Turn: pair,
				Text: i18n.Tf("delivery.turn_removed", pair)})
		}
	}
	return out
}

// teamOf is the running team's name, from the member the prompt is for.
func teamOf(current definition.Start) string {
	return strings.TrimSuffix(current.Name, "-"+current.Short)
}

// turnNames is the endpoints a definition's turns give, with the running team's
// prefix taken off: a short member name, or human.
func turnNames(turns []string, team string) []string {
	out := make([]string, 0, len(turns))
	for _, turn := range turns {
		name := turn
		if team != "" {
			name = strings.TrimPrefix(name, team+"-")
		}
		if name == "human" || name == "人間" {
			name = "human"
		}
		out = append(out, name)
	}
	return out
}

// oldTurnNames reads the endpoints from the bullets of an old turns note. The
// line carries the herdr name and role; only the name before the role is used.
func oldTurnNames(lines []string, team string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		name := line
		if i := strings.IndexAny(name, " （("); i >= 0 {
			name = name[:i]
		}
		if name == "human" || name == "人間" {
			name = "human"
		} else if team != "" {
			name = strings.TrimPrefix(name, team+"-")
		}
		out = append(out, name)
	}
	return out
}

func turnPair(from, to string) string { return from + " → " + to }

// workerChanges names the workers a member may hire that changed, by purpose.
// The current purposes are in the definition; the old note's pairs are compared
// as text and reported by purpose, never by the whole prompt line.
func workerChanges(current definition.Start, segment, oldPrompt string) []Change {
	var nowPairs, nowPurposes []string
	for _, worker := range current.Workers {
		nowPairs = append(nowPairs, worker.Purpose+" → "+worker.Profile)
		nowPurposes = append(nowPurposes, worker.Purpose)
	}
	heading := firstHeading(segment)
	old := section(oldPrompt, heading)
	// A purpose that is already in the old note with another profile is a
	// changed profile, not a new worker: a person reading it is told what
	// changed, and the team's note says the next worker gets it (d-36ef).
	was := map[string]string{}
	for _, pair := range workerPairs(old) {
		purpose, profile, _ := strings.Cut(pair, " → ")
		was[purpose] = profile
	}
	var out []Change
	for i, pair := range nowPairs {
		if strings.Contains(oldPrompt, pair) {
			continue
		}
		purpose := nowPurposes[i]
		if before, ok := was[purpose]; ok && before != current.Workers[i].Profile {
			out = append(out, Change{Kind: "worker_profile", Worker: purpose,
				Text: i18n.Tf("delivery.worker_profile_changed", purpose)})
			continue
		}
		out = append(out, Change{Kind: "worker", Worker: purpose,
			Text: i18n.Tf("delivery.worker_added", purpose)})
	}
	for _, pair := range workerPairs(old) {
		purpose, _, _ := strings.Cut(pair, " → ")
		if !containsLine(nowPurposes, purpose) {
			out = append(out, Change{Kind: "worker", Worker: purpose,
				Text: i18n.Tf("delivery.worker_removed", purpose)})
		}
	}
	return out
}

var workerPair = regexp.MustCompile(`([^、\s]+) → ([^、\s]+)`)

// workerPairs reads the purpose → profile pairs out of an old workers note. The
// pairs stand after the note's colon, one line: the line before it says 「目的 →
// プロファイル」 as an example, and reading that as a pair would name a worker
// that never existed.
func workerPairs(text string) []string {
	var out []string
	for line := range strings.SplitSeq(text, "\n") {
		at := strings.LastIndex(line, ": ")
		if at < 0 {
			continue
		}
		for _, match := range workerPair.FindAllStringSubmatch(line[at+2:], -1) {
			out = append(out, match[1]+" → "+match[2])
		}
		return out
	}
	return out
}

// firstHeading is the first nonempty line of a segment, which is the section's
// heading for the turns and workers notes.
func firstHeading(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, "#") {
			return line
		}
	}
	return ""
}

// section is one heading's lines in prompt, up to the next heading.
func section(prompt, heading string) string {
	if heading == "" {
		return ""
	}
	_, after, ok := strings.Cut(prompt, heading)
	if !ok {
		return ""
	}
	rest := after
	if end := strings.Index(rest, "\n## "); end >= 0 {
		rest = rest[:end]
	}
	return rest
}

// bulletLines are the "- " lines of a section, trimmed.
func bulletLines(text string) []string {
	var out []string
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "- "); ok {
			out = append(out, strings.TrimSpace(after))
		}
	}
	return out
}

func containsLine(lines []string, line string) bool { return slices.Contains(lines, line) }

// dedupe drops a summary that repeats one already there (a document can be
// named by two segments of the same prompt).
func dedupe(changes []Change) []Change {
	seen := map[Change]bool{}
	out := make([]Change, 0, len(changes))
	for _, change := range changes {
		if seen[change] {
			continue
		}
		seen[change] = true
		out = append(out, change)
	}
	return out
}
