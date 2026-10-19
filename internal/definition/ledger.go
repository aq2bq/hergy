package definition

import (
	"fmt"
	"slices"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
)

// Ledger is one rule a team definition writes in ledger:: a write, and the
// members allowed to make it. It is the shape hergy can check now; another
// shape (a chain, an order) would be a new field beside it (spec-ledger, 足さないもの).
type Ledger struct {
	// Write is the name of a gy write, as the first two words of its why: "req
	// approve", "question close", or "question to human" for a question whose
	// decider is no member of the team.
	Write string
	// Members are the member names allowed to make the write, in the order the
	// line writes them. A worker's name cannot be here: a worker takes no rules.
	Members []string
}

// ledgerWrites is the writes ledger: may name. The first ten are gy's own write
// names (the first two words of the why a write carries); questionToHuman is
// opening a question whose decider is no member of the team (spec-ledger).
var ledgerWrites = []string{
	"req add",
	"req approve",
	"req done",
	"req revise",
	"criterion satisfy",
	"need add",
	"need close",
	"decide",
	QuestionAdd,
	"question close",
	QuestionToHuman,
}

// The two writes a question needs, told apart by its decider: QuestionAdd is a
// question whose decider is a member of the team, and QuestionToHuman is one
// whose decider is nobody in the team (the human, or outside it). A definition's
// ledger: may name either.
const (
	QuestionAdd     = "question add"
	QuestionToHuman = "question to human"
)

// ledgerWriteKnown reports whether a name is one ledger: may write.
func ledgerWriteKnown(name string) bool {
	return slices.Contains(ledgerWrites, name)
}

// ledgerWords is the writes of ledger: as one comma-joined list, for the
// sentence that refuses an unknown one.
func ledgerWords() string {
	return strings.Join(ledgerWrites, i18n.T("list.separator"))
}

// parseLedger reads the ledger: lines, each "- <write> by <member>[, <member>...]",
// into rules. A line hergy cannot read, a write it does not know, a name that is
// no member, and a worker's name all stop the definition, the way a broken
// turns: line does (spec-ledger: 形の検査).
func parseLedger(entries *stringList, members []MemberNode, workers []WorkerNode) ([]Ledger, error) {
	if entries == nil {
		return nil, nil
	}
	rules := make([]Ledger, 0, len(*entries))
	for i, line := range *entries {
		// The label names the line as a person counts lines, so a broken line
		// reads "ledger: の 2 行目"/"line 2 in ledger:".
		label := i18n.Tf("label.ledger_line", i+1)
		rule, err := parseLedgerLine(line, label, members, workers)
		if err != nil {
			return nil, field(fmt.Sprintf("ledger[%d]", i), err)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// parseLedgerLine reads one ledger: line into its rule: the write before " by ",
// and the member names after it.
func parseLedgerLine(line, label string, members []MemberNode, workers []WorkerNode) (Ledger, error) {
	write, rest, found := strings.Cut(line, " by ")
	if !found {
		return Ledger{}, i18n.Errorf("error.ledger_shape", label, strings.TrimSpace(line))
	}
	write = strings.TrimSpace(write)
	if !ledgerWriteKnown(write) {
		return Ledger{}, i18n.Errorf("error.ledger_write_unknown", label, strings.TrimSpace(line), ledgerWords())
	}
	names := ledgerNames(rest)
	for _, name := range names {
		// An empty piece is a missing name: no member at all, or a comma with
		// nothing between, which is the line's shape rather than a member.
		if name == "" {
			return Ledger{}, i18n.Errorf("error.ledger_shape", label, strings.TrimSpace(line))
		}
		if isMemberName(members, name) {
			continue
		}
		if isWorkerName(workers, name) {
			return Ledger{}, i18n.Errorf("error.ledger_worker_name", label, name)
		}
		return Ledger{}, i18n.Errorf("error.ledger_no_member", label, name)
	}
	return Ledger{Write: write, Members: names}, nil
}

// ledgerNames is the member names a ledger: line lists: the text after " by ",
// split on commas with the ends trimmed. An empty piece is kept, so a trailing
// comma or a doubled comma is refused rather than silently dropped.
func ledgerNames(rest string) []string {
	pieces := strings.Split(rest, ",")
	names := make([]string, 0, len(pieces))
	for _, piece := range pieces {
		names = append(names, strings.TrimSpace(piece))
	}
	return names
}

// isMemberName reports whether a name is one of the definition's member names.
func isMemberName(members []MemberNode, name string) bool {
	for i := range members {
		if members[i].Name == name {
			return true
		}
	}
	return false
}

// isWorkerName reports whether a name is a worker's: the node's own name,
// "<from>/<purpose>", or the purpose alone, which is what a person is likely to
// write.
func isWorkerName(workers []WorkerNode, name string) bool {
	for _, worker := range workers {
		if name == worker.From+"/"+worker.Purpose || name == worker.Purpose {
			return true
		}
	}
	return false
}
