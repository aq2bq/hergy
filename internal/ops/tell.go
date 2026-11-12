package ops

import (
	"context"
	"fmt"
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/herdr"
	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
	"github.com/aq2bq/hergy/internal/ops/internal/protocol"
	"github.com/aq2bq/hergy/internal/ops/internal/screenlog"
	"github.com/aq2bq/hergy/internal/ops/internal/turns"
)

// TellInput names an unresolved recipient and optional sender. Dry mode keeps
// the existing definition-only contract: it reads neither herdr nor workers.
type TellInput struct {
	MemberInput
	From, Body string
	Now        func() time.Time
}

// TellResult names the observed parties of a completed tell.
type TellResult struct {
	From, To domainname.Seen
	Human    bool
}

// Tell resolves both parties, sends once and records the handoff for the
// watcher. Its existing turn rules apply equally to members and hired workers.
func Tell(ctx context.Context, in TellInput) (TellResult, error) {
	r, err := resolveMember(in.MemberInput, in.Dry)
	if err != nil {
		return TellResult{}, err
	}
	parties := tellParties(r, in)
	out := TellResult{Human: turns.IsHuman(in.Member)}
	if !out.Human && in.Body == "" {
		return out, i18n.Errorf("error.tell_needs_body")
	}
	if in.From != "" {
		out.From, err = findTellParty(r.Team.Name.Observation(), parties, in.From)
		if err != nil {
			return out, i18n.Errorf("error.tell_from_unknown", in.From)
		}
	} else if !in.Dry && CurrentPane() != "" {
		pane := CurrentPane()
		if agent, e := herdr.New(portOrDefault(in.Port)).Agent(pane); e == nil {
			for _, party := range parties {
				if party == agent.Name {
					out.From = party
					break
				}
			}
		}
	}
	if out.Human {
		if in.Dry {
			emit(in.Observe, Event{Text: i18n.T("send.dry_human")})
			return out, nil
		}
		if out.From.String() == "" {
			return out, i18n.Errorf("error.tell_human_from_person")
		}
		if err := recordTell(r.Team.Name.Observation(), out.From, turns.Human, in.Now); err != nil {
			return out, err
		}
		emit(in.Observe, Event{Text: i18n.T("send.told_human")})
		return out, nil
	}
	out.To, err = findTellParty(r.Team.Name.Observation(), parties, in.Member)
	if err != nil {
		return out, err
	}
	text := sendText(out.From.String(), in.Body)
	if in.Dry {
		emit(in.Observe, Event{Text: fmt.Sprintf(i18n.T("send.dry"), out.To.String(), text)})
		return out, nil
	}
	emit(in.Observe, Event{Text: pluginNote()})
	h := herdr.New(stopPort{Port: portOrDefault(in.Port), ctx: ctx})
	if err := h.Verify(protocol.ModePrompt); err != nil {
		return out, err
	}
	if err := h.Prompt(out.To, text); err != nil {
		return out, err
	}
	emit(in.Observe, Event{Text: fmt.Sprintf(i18n.T("send.sent"), out.To.String(), text)})
	return out, recordTell(r.Team.Name.Observation(), out.From, domainname.ShortIn(out.To, r.Team.Name.Observation()).String(), in.Now)
}

func findTellParty(team domainname.Seen, parties []domainname.Seen, raw string) (domainname.Seen, error) {
	// The short spelling wins, as it does for definition members.
	for _, p := range parties {
		if domainname.ShortIn(p, team).String() == raw {
			return p, nil
		}
	}
	for _, p := range parties {
		if p.String() == raw {
			return p, nil
		}
	}
	return domainname.Seen{}, i18n.Errorf("error.tell_target_unknown", raw)
}

// tellParties includes a worker only when a hire record, worker tab and live
// agent agree. A similarly prefixed foreign pane does not become a sender.
func tellParties(r *Resolved, in TellInput) []domainname.Seen {
	parties := make([]domainname.Seen, 0, len(r.Starts))
	for _, s := range r.Starts {
		parties = append(parties, s.Name.Observation())
	}
	if in.Dry {
		return parties
	}
	read := readSession(in.Port)
	if read.Err != nil {
		return parties
	}
	live := map[domainname.Seen]bool{}
	for _, a := range read.Agents {
		live[a.Name] = true
	}
	tabs := map[domainname.Seen]bool{}
	for _, tab := range read.Tabs {
		if w, ok := tab.Label.WorkerIn(r.Team.Name.Observation()); ok {
			tabs[w.Agent()] = true
		}
	}
	log, err := launchlog.Open()
	if err != nil {
		return parties
	}
	records, err := log.Read()
	if err != nil {
		return parties
	}
	seen := map[domainname.Seen]bool{}
	for _, record := range records {
		if record.Team != r.Team.Name.Observation() || !record.IsHire() || record.Hire == nil {
			continue
		}
		worker := record.Hire.Worker
		if live[worker] && tabs[worker] && !seen[worker] {
			parties = append(parties, worker)
			seen[worker] = true
		}
	}
	return parties
}

func recordTell(teamName, fromHerdr domainname.Seen, to string, now func() time.Time) error {
	dir, err := screenlog.DefaultDir()
	if err != nil {
		return err
	}
	from := ""
	if fromHerdr.String() != "" {
		from = domainname.ShortIn(fromHerdr, teamName).String()
	}
	if now == nil {
		now = time.Now
	}
	return turns.WriteTell(turns.TellDir(dir, teamName.String()), turns.Tell{From: from, To: to, At: now()})
}
