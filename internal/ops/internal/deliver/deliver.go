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
	"unicode"
	"unicode/utf8"

	"github.com/aq2bq/hergy/internal/definition"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/migrate"
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
	Turn     string `json:"turn,omitempty"`
	Worker   string `json:"worker,omitempty"`
	Text     string `json:"text"`
	Diff     string `json:"diff,omitempty"`
}

// Doc is one rule document as the comparison reads it. Path is the file's own
// path, Display the path a screen shows, and Text its contents.
type Doc struct {
	Path    string
	Display string
	Text    string
}

// Compare is what changed between the prompt a member last read and the prompt
// its current definition builds. lang is the language both prompts are written
// in (the receipt's, which the current prompt is rebuilt with); oldDocs is
// only that member's previously read documents by path; current is the current Start (its
// prompt, segments, turns and workers) and docs are the current rule documents.
func Compare(team domainname.TeamName, lang, oldPrompt string, oldDocs map[string]string, current definition.Start, docs []Doc) []Change {
	if oldPrompt == "" {
		return []Change{{Kind: "no_record", Text: i18n.T("delivery.no_record")}}
	}
	if oldPrompt == current.Prompt {
		return nil
	}
	out := documentChanges(oldDocs, docs)
	for _, segment := range current.Segments {
		if segment.Text == "" || strings.Contains(oldPrompt, segment.Text) {
			continue
		}
		switch segment.Kind {
		case "team_rules", "role_doc":
			continue
		case "turns":
			out = append(out, turnChanges(domainname.RunningNames(team.Ref()), current, segment.Text, oldPrompt, lang)...)
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

// displayOf is the document as a summary names it: its base name, so the
// sentence stays short while the Change keeps the whole path.
func displayOf(doc Doc) string { return filepath.Base(doc.Path) }

// ruleNameLength is the most runes of a rule's text a change quotes: enough to
// tell one rule from another, and short enough to read in one line.

// turnChanges compares the turns a member hands to, by their endpoints. The
// current endpoints come from the definition (short member names and human);
// the old endpoints are read from the old prompt's turns note. The summary is
// the pair "lead → analyst", never a prompt line (r-a8fa audit 1 B).
func turnChanges(team domainname.Naming, current definition.Start, segment, oldPrompt, lang string) []Change {
	now := turnNames(current.Turns, team)
	old := oldTurnNames(bulletLines(section(oldPrompt, firstHeading(segment))), team, lang)
	var out []Change
	for _, to := range now {
		if !containsLine(old, to) {
			pair := turnPair(current.Short.String(), to)
			out = append(out, Change{Kind: "turn", Turn: pair,
				Text: i18n.Tf("delivery.turn_added", pair)})
		}
	}
	for _, to := range old {
		if !containsLine(now, to) {
			pair := turnPair(current.Short.String(), to)
			out = append(out, Change{Kind: "turn", Turn: pair,
				Text: i18n.Tf("delivery.turn_removed", pair)})
		}
	}
	return out
}

// turnNames is the endpoints a definition's turns give, with the running team's
// prefix taken off: a short member name, or human.
func turnNames(turns []string, team domainname.Naming) []string {
	out := make([]string, 0, len(turns))
	for _, turn := range turns {
		name := turn
		if team.String() != "" {
			name = team.Short(name)
		}
		if name == "human" || name == "人間" {
			name = "human"
		}
		out = append(out, name)
	}
	return out
}

// oldTurnNames reads the endpoints from the bullets of an old turns note. The
// note writes each party as the item template does ("- <name>（ロール <role>）"
// or the human's own line); a bullet of another list that follows the note is
// not a turn, so it is left out (r-7aad).
func oldTurnNames(lines []string, team domainname.Naming, lang string) []string {
	item := turnItemOf(lang)
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		name, ok := item.nameOf(line)
		if !ok {
			continue
		}
		if name == "human" {
			out = append(out, "human")
			continue
		}
		if team.String() != "" {
			name = team.Short(name)
		}
		out = append(out, name)
	}
	return out
}

// turnItem is the shape the turns note writes one party in, taken from the
// locale's own item template: the words between a member's name and its role,
// and the words after the role.
type turnItem struct {
	marker string
	suffix string
}

// turnItemOf is the turns note's item shape in one language.
func turnItemOf(lang string) turnItem {
	member := i18n.Text(lang, "turns.member_item")
	_, afterName, _ := strings.Cut(member, "%s")
	marker, afterRole, _ := strings.Cut(afterName, "%s")
	return turnItem{marker: marker, suffix: afterRole}
}

// nameOf is the party one bullet of an old turns note names. A bullet that is
// not written as the note writes its parties (a roster line, "- lead:
// demo-lead") is no turn and answers false.
func (t turnItem) nameOf(line string) (string, bool) {
	if isHumanTurn(line) {
		return "human", true
	}
	name, rest, ok := strings.Cut(line, t.marker)
	if !ok || name == "" || !strings.HasSuffix(rest, t.suffix) {
		return "", false
	}
	return name, true
}

// isHumanTurn says a bullet of an old turns note names the human. The human's
// item has no role, so it is the one party the note writes without the role
// marker; it starts with the human's name in either language.
func isHumanTurn(line string) bool {
	for _, name := range []string{"human", "人間"} {
		rest, ok := strings.CutPrefix(line, name)
		if !ok {
			continue
		}
		if rest == "" {
			return true
		}
		first, _ := utf8.DecodeRuneInString(rest)
		if !unicode.IsLetter(first) && !unicode.IsDigit(first) {
			return true
		}
	}
	return false
}

func turnPair(from, to string) string { return from + " → " + to }

// workerChanges names the workers a member may hire that changed, by purpose.
// The current purposes are in the definition; the old note's pairs are compared
// as text and reported by purpose, never by the whole prompt line.
func workerChanges(current definition.Start, segment, oldPrompt string) []Change {
	var nowPairs, nowPurposes []string
	for _, worker := range current.Workers {
		nowPairs = append(nowPairs, worker.Purpose.String()+" → "+worker.Profile)
		nowPurposes = append(nowPurposes, worker.Purpose.String())
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
	if end := strings.Index(rest, "\n#"); end >= 0 {
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

// documentChanges compares raw prose, converting only historical YAML snapshots.
func documentChanges(old map[string]string, docs []Doc) []Change {
	var out []Change
	seen := map[string]bool{}
	for _, doc := range docs {
		key := doc.Path
		before, exists := old[key]
		if !exists {
			legacy := strings.TrimSuffix(key, ".md") + ".yaml"
			if raw, ok := old[legacy]; ok {
				seen[legacy] = true
				converted, err := migrate.Convert(raw, legacy)
				if err != nil {
					out = append(out, Change{Kind: "document", Document: doc.Display, Text: i18n.Tf("delivery.document_unreadable", displayOf(doc))})
					continue
				}
				before = converted.Text
			}
		}
		seen[key] = true
		if before != doc.Text {
			out = append(out, Change{Kind: "document", Document: doc.Display, Text: i18n.Tf("delivery.document_changed", displayOf(doc)), Diff: textDiff(before, doc.Text)})
		}
	}
	keys := make([]string, 0, len(old))
	for path := range old {
		keys = append(keys, path)
	}
	slices.Sort(keys)
	for _, path := range keys {
		if seen[path] || (filepath.Ext(path) != ".md" && filepath.Ext(path) != ".yaml") {
			continue
		}
		// Launch records may also contain topology and profiles.
		if filepath.Base(path) == "team.yaml" || filepath.Base(path) == "profiles.yaml" {
			continue
		}
		before := old[path]
		display := path
		if filepath.Ext(path) == ".yaml" {
			display = strings.TrimSuffix(path, ".yaml") + ".md"
			converted, err := migrate.Convert(before, path)
			if err != nil {
				out = append(out, Change{Kind: "document", Document: display, Text: i18n.Tf("delivery.document_unreadable", filepath.Base(display))})
				continue
			}
			before = converted.Text
		}
		out = append(out, Change{Kind: "document", Document: display, Text: i18n.Tf("delivery.document_changed", filepath.Base(display)), Diff: textDiff(before, "")})
	}
	return out
}

func textDiff(before, after string) string {
	a, b := strings.Split(before, "\n"), strings.Split(after, "\n")
	for len(a) > 0 && len(b) > 0 && a[0] == b[0] {
		a, b = a[1:], b[1:]
	}
	for len(a) > 0 && len(b) > 0 && a[len(a)-1] == b[len(b)-1] {
		a, b = a[:len(a)-1], b[:len(b)-1]
	}
	var out strings.Builder
	for _, line := range a {
		out.WriteString("- " + line + "\n")
	}
	for _, line := range b {
		out.WriteString("+ " + line + "\n")
	}
	return out.String()
}
