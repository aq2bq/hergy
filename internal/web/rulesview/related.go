package rulesview

import (
	"sort"
	"strconv"
	"strings"

	"github.com/aq2bq/hergy/internal/rules"
)

// RelatedRule is one rule the panel lists beside the open one: its id, and where
// the panel opens it. Doc is the document's name, set only when that rule stands
// in another document than the body shows, so a link that leaves this document
// says so.
type RelatedRule struct {
	ID   string
	Href string
	Doc  string
}

// Related is the three lists of related rules (試作 refsIn・backrefs・
// derivedFrom): what this rule's text points at, what points at it, and what was
// derived from it. A list with nothing in it is left out of the panel.
type Related struct {
	Out  []RelatedRule
	Back []RelatedRule
	Kids []RelatedRule
}

// Any reports whether the panel has anything to show: the three lists together.
func (r Related) Any() bool { return len(r.Out)+len(r.Back)+len(r.Kids) > 0 }

// foundRule is one rule found by its id, with where it stands.
type foundRule struct {
	Doc     Doc
	Section int
	Index   int
	ID      string
}

// relatedOf builds the three lists for one rule of one document. The range is
// the documents the members that read shown also read (s.scope), the same range
// the report picker uses (試作 scopeOf); the derived list looks over every
// document of the layer.
func (s *Screen) relatedOf(layer string, docs []Doc, shown Doc, view string, edit EditRule) Related {
	var out Related
	if shown.Document == nil {
		return out
	}
	scope := s.scope(layer, docs, shown)
	meID := strings.TrimSpace(edit.ID)
	out.Out = s.linkedRules(scope, shown, view, edit.Text+"\n"+edit.Why, meID)
	out.Back = s.backrefs(scope, shown, view, meID)
	out.Kids = s.derivedFrom(layer, docs, shown, view, meID)
	return out
}

// linkedRules is the rules one piece of text points at: every id of the range
// that stands in the text on its own, resolved to a rule of the range, in the
// order the ids first appear (試作 refsIn). self is left out.
func (s *Screen) linkedRules(scope []Doc, shown Doc, view, text, self string) []RelatedRule {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	var out []RelatedRule
	seen := map[string]bool{}
	for _, id := range matchIDs(text, scopeIDs(scope)) {
		if id == self {
			continue
		}
		found, ok := resolveInScope(scope, shown, id)
		if !ok || seen[found.Doc.Path+"#"+found.ID] {
			continue
		}
		seen[found.Doc.Path+"#"+found.ID] = true
		out = append(out, relatedLink(found, view))
	}
	return out
}

// backrefs is the rules of the range whose text points at this rule: each is
// resolved in its own document's range, so only the rule the id really reaches
// is listed (試作 backrefs).
func (s *Screen) backrefs(scope []Doc, shown Doc, view, meID string) []RelatedRule {
	if meID == "" {
		return nil
	}
	var out []RelatedRule
	for _, doc := range scope {
		if doc.Document == nil {
			continue
		}
		for section := range doc.Document.Sections {
			list := doc.Document.Sections[section].Rules
			for index := range list {
				rule := list[index]
				if doc.Path == shown.Path && rule.ID == meID {
					continue // the rule itself
				}
				if !pointsAt(scope, doc, rule, meID, shown.Path) {
					continue
				}
				out = append(out, relatedLink(foundRule{Doc: doc, Section: section, Index: index, ID: rule.ID}, view))
			}
		}
	}
	return out
}

// derivedFrom is the rules of the layer whose origin is of the derived kind and
// names this rule (試作 derivedFrom). The reference is resolved in the range of
// the document it stands in, so an id that means another rule there does not
// count.
func (s *Screen) derivedFrom(layer string, docs []Doc, shown Doc, view, meID string) []RelatedRule {
	if meID == "" {
		return nil
	}
	var out []RelatedRule
	for _, doc := range docs {
		if doc.Document == nil {
			continue
		}
		scope := s.scope(layer, docs, doc)
		for section := range doc.Document.Sections {
			list := doc.Document.Sections[section].Rules
			for index := range list {
				if !derivedFromRule(scope, doc, list[index], meID, shown.Path) {
					continue
				}
				out = append(out, relatedLink(foundRule{Doc: doc, Section: section, Index: index, ID: list[index].ID}, view))
			}
		}
	}
	return out
}

// pointsAt says whether one rule's text points at the rule named by id in the
// document at path: the id stands in the text on its own and resolves there.
func pointsAt(scope []Doc, doc Doc, rule rules.Rule, id, path string) bool {
	if !containsID(rule.Text+"\n"+rule.Why, id) {
		return false
	}
	found, ok := resolveInScope(scope, doc, id)
	return ok && found.Doc.Path == path && found.ID == id
}

// derivedFromRule says whether one rule's derived origin names the rule at path.
func derivedFromRule(scope []Doc, doc Doc, rule rules.Rule, id, path string) bool {
	for _, origin := range rule.Origin {
		if origin.Kind != rules.KindDerived {
			continue
		}
		for _, ref := range origin.Refs {
			found, ok := resolveInScope(scope, doc, strings.TrimSpace(ref))
			if ok && found.Doc.Path == path && found.ID == id {
				return true
			}
		}
	}
	return false
}

// resolveInScope is the rule one id names, looked up the way the prototype's
// resolveId does: in the document it was written in first, then in the range.
func resolveInScope(scope []Doc, first Doc, id string) (foundRule, bool) {
	if id == "" {
		return foundRule{}, false
	}
	order := make([]Doc, 0, len(scope)+1)
	order = append(order, first)
	order = append(order, scope...)
	for _, doc := range order {
		if doc.Document == nil {
			continue
		}
		for section := range doc.Document.Sections {
			list := doc.Document.Sections[section].Rules
			for index := range list {
				if list[index].ID == id {
					return foundRule{Doc: doc, Section: section, Index: index, ID: id}, true
				}
			}
		}
	}
	return foundRule{}, false
}

// scopeIDs is every rule id of the range, the longest first so a shorter id does
// not hide a longer one beside it (試作 idRe).
func scopeIDs(scope []Doc) []string {
	seen := map[string]bool{}
	var ids []string
	for _, doc := range scope {
		if doc.Document == nil {
			continue
		}
		for _, rule := range doc.Document.Rules() {
			if rule.ID != "" && !seen[rule.ID] {
				seen[rule.ID] = true
				ids = append(ids, rule.ID)
			}
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		if len(ids[i]) != len(ids[j]) {
			return len(ids[i]) > len(ids[j])
		}
		return ids[i] < ids[j]
	})
	return ids
}

// relatedLink is one related rule as the panel links it: its id and where the
// panel opens it, with the document's name when it stands elsewhere.
func relatedLink(found foundRule, view string) RelatedRule {
	link := RelatedRule{ID: found.ID, Href: panelHref(found.Doc, view, found.Section, found.Index)}
	if view != "" && found.Doc.Path != view {
		link.Doc = found.Doc.Label()
	}
	return link
}

// panelHref is where the panel opens one rule, with the document the body shows
// carried along, so saving it comes back to that document.
func panelHref(doc Doc, view string, section, index int) string {
	return panelPrefix(doc, view) + "&section=" + strconv.Itoa(section) + "&i=" + strconv.Itoa(index)
}

// matchIDs is the ids that stand in text on their own, in the order they first
// appear. ids are expected longest first.
func matchIDs(text string, ids []string) []string {
	type hit struct {
		id string
		at int
	}
	var hits []hit
	for _, id := range ids {
		if at, ok := firstID(text, id); ok {
			hits = append(hits, hit{id: id, at: at})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].at < hits[j].at })
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.id
	}
	return out
}

// firstID is where an id first stands in text on its own.
func firstID(text, id string) (int, bool) {
	for from := 0; ; {
		i := strings.Index(text[from:], id)
		if i < 0 {
			return 0, false
		}
		at := from + i
		if idBoundary(text, at, at+len(id)) {
			return at, true
		}
		from = at + 1
	}
}

// containsID says whether an id stands in text on its own.
func containsID(text, id string) bool {
	_, ok := firstID(text, id)
	return ok
}

// idBoundary says the id between start and end stands alone: the character
// before and the one after are neither a letter, a digit nor '-'. Go's regexp
// has no lookaround, so the prototype's (?<![A-Za-z0-9-])…(?![A-Za-z0-9-]) is
// read here (idRe). An id is case-sensitive, so the two are not folded.
func idBoundary(text string, start, end int) bool {
	if start > 0 && idWord(text[start-1]) {
		return false
	}
	if end < len(text) && idWord(text[end]) {
		return false
	}
	return true
}

// idWord says one byte is part of an id: an ASCII letter, a digit or '-', as the
// prototype's character class has it. Every other byte (a multibyte character)
// is a boundary.
func idWord(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9', b == '-':
		return true
	}
	return false
}
