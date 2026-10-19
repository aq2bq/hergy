package app

import (
	"io"
	"strings"

	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/gycli"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/team"
)

// ledgerCheck checks team definitions' ledger: rules against the project's
// ledger, reading gy once for the whole run. A definition that writes no
// ledger: rule is not checked, so a project that writes none anywhere never
// starts gy.
type ledgerCheck struct {
	env    team.Env
	loaded bool
	ledger gycli.Ledger
	usable bool
	writes []gycli.Write
	err    error
}

// findings is one definition's ledger warnings, and whether the ledger was
// checked at all. false means the ledger could not be read: the caller says so
// once, because a rule that was not checked is not a rule that was broken
// (spec-ledger 3, d-db1b).
func (c *ledgerCheck) findings(def *definition.Definition) ([]check.Finding, bool) {
	if len(def.Ledger) == 0 {
		return nil, true
	}
	c.load()
	if !c.usable || c.err != nil {
		return nil, false
	}
	writes := make([]check.LedgerWrite, len(c.writes))
	for i, write := range c.writes {
		writes[i] = check.LedgerWrite{Actor: write.Actor, Node: write.Node, Why: write.Why}
	}
	found, err := check.LedgerFindings(def, teamNames(def), writes, c.ledger.Decider)
	if err != nil {
		return nil, false
	}
	return found, true
}

// load reads the ledger once, on the first definition that needs it.
func (c *ledgerCheck) load() {
	if c.loaded {
		return
	}
	c.loaded = true
	ledger, ok := gycli.OpenLedger(c.env.ProjectRoot)
	if !ok {
		return
	}
	c.ledger = ledger
	c.usable = true
	c.writes, c.err = ledger.Writes()
}

// teamNames is every name one team definition runs under: its own name, and the
// names the launch record shows teams were started as from this definition
// (--as). A record belongs to the definition it read, so the team.yaml path is
// what a record is matched by, the way launchlog.Latest matches one (M1).
func teamNames(def *definition.Definition) []string {
	names := []string{def.Name}
	seen := map[string]bool{def.Name: true}
	log, err := launchlog.Open()
	if err != nil {
		return names
	}
	records, err := log.Read()
	if err != nil {
		return names
	}
	for _, record := range records {
		if record.TeamYAML != def.TeamYAML || record.Team == "" || seen[record.Team] {
			continue
		}
		seen[record.Team] = true
		names = append(names, record.Team)
	}
	return names
}

// printLedger writes one definition's ledger lines under its file line: a
// warning for each write a rule was broken by, or one line saying the ledger
// could not be read. It is called after the definition reads, so a definition
// that stops is not checked.
func printLedger(stdout io.Writer, def *definition.Definition, c *ledgerCheck) {
	found, checked := c.findings(def)
	if !checked {
		indentField(stdout, 1, label("ledger"), i18n.T("validate.ledger_unreadable"))
		return
	}
	for _, finding := range found {
		indentField(stdout, 1, label("warning"), finding.Text)
	}
}

// ledgerText is a definition's ledger: rules as one line for show: each rule as
// "<write> by <member>[, <member>...]", in the written order, the rules joined
// the way a list is written.
func ledgerText(rules []definition.Ledger) string {
	lines := make([]string, len(rules))
	for i, rule := range rules {
		lines[i] = rule.Write + " by " + strings.Join(rule.Members, i18n.T("list.separator"))
	}
	return strings.Join(lines, i18n.T("list.separator"))
}
