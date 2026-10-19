package check

import (
	"fmt"
	"slices"
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
)

// LedgerWrite is one write of the ledger, as the ledger check reads it: the
// writer, the node it is about, and why. The first two words of why name the
// write (gy writes "req approve r-1c42"), which is what a team definition's
// ledger: rules speak of (spec-ledger).
type LedgerWrite struct {
	Actor string
	Node  string
	Why   string
}

// Decider reads the decider of one question node. It is asked for the writes
// that need it: a question add, whose name is "question to human" instead when
// its decider is nobody in the team.
type Decider func(node string) (string, error)

// LedgerFindings is what one team definition's ledger: rules say about the
// ledger: every write of a writer of this team that a rule gives to other
// members. teams are the names the definition runs under: its own, and the names
// the launch record shows it was started as (--as). A writer of no name in teams
// is not this team's writer and is not judged (spec-ledger: チームの外の書き手は見ない).
//
// It is a warning, not a stop: d-db1b keeps hergy from refusing a write. An
// error is a question whose decider could not be read, which the caller reports
// as a ledger that was not checked.
func LedgerFindings(def *definition.Definition, teams []string, writes []LedgerWrite, decider Decider) ([]Finding, error) {
	if len(def.Ledger) == 0 {
		return nil, nil
	}
	// The decider is read only for the writes that turn on it, and only when a
	// rule speaks of a question at all.
	judgesQuestion := hasLedgerRule(def, definition.QuestionAdd) || hasLedgerRule(def, definition.QuestionToHuman)
	var out []Finding
	for _, write := range writes {
		team, member, ok := teamWriter(def, teams, write.Actor)
		if !ok {
			continue
		}
		name := writeName(write.Why)
		if name == definition.QuestionAdd && judgesQuestion {
			outside, err := asksOutsideTeam(def, teams, write.Node, decider)
			if err != nil {
				return nil, err
			}
			if outside {
				name = definition.QuestionToHuman
			}
		}
		for i, rule := range def.Ledger {
			if rule.Write != name || slices.Contains(rule.Members, member) {
				continue
			}
			out = append(out, Finding{
				Field: fmt.Sprintf("ledger[%d]", i),
				Kind:  KindLedger,
				Text: i18n.Tf("validate.ledger_breach",
					write.Node, name, write.Actor, team, strings.Join(rule.Members, i18n.T("list.separator"))),
			})
		}
	}
	return out, nil
}

// teamWriter is the team name and member name a writer of this team goes by, and
// whether the actor is one at all. The longest team name whose "<name>-" the
// actor starts with wins, so a team launched as "a-b" is not read as team "a"
// with a member "b-lead"; a name that is no member of the definition is another
// team's writer, or an older way of writing one, and is not this team's.
func teamWriter(def *definition.Definition, teams []string, actor string) (string, string, bool) {
	best := ""
	for _, team := range teams {
		if strings.HasPrefix(actor, team+"-") && len(team) > len(best) {
			best = team
		}
	}
	if best == "" {
		return "", "", false
	}
	member := actor[len(best)+1:]
	if !hasMember(def, member) {
		return "", "", false
	}
	return best, member, true
}

// asksOutsideTeam is whether a question's decider is no member of the team: the
// human, another team, or nobody. A decider is written the way gy records a
// writer, so a member's is its herdr name.
func asksOutsideTeam(def *definition.Definition, teams []string, node string, decider Decider) (bool, error) {
	who, err := decider(node)
	if err != nil {
		return false, err
	}
	if who == "" {
		return true, nil
	}
	for _, team := range teams {
		for i := range def.Graph.Members {
			if who == team+"-"+def.Graph.Members[i].Name {
				return false, nil
			}
		}
	}
	return true, nil
}

// writeName is the name of a write: the first two words of its why.
func writeName(why string) string {
	fields := strings.Fields(why)
	switch len(fields) {
	case 0:
		return ""
	case 1:
		return fields[0]
	default:
		return fields[0] + " " + fields[1]
	}
}

// hasLedgerRule reports whether the definition gives a rule about one write.
func hasLedgerRule(def *definition.Definition, write string) bool {
	for _, rule := range def.Ledger {
		if rule.Write == write {
			return true
		}
	}
	return false
}

// hasMember reports whether a name is one of the definition's member names.
func hasMember(def *definition.Definition, name string) bool {
	for i := range def.Graph.Members {
		if def.Graph.Members[i].Name == name {
			return true
		}
	}
	return false
}
