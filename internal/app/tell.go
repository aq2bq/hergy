package app

import (
	"fmt"
	"io"
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/screenlog"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/turns"
)

// runTell sends one member a message from a person or another member, and
// records the tell for the team's watcher: this is how a member hands its turn
// on, and how hergy knows who opened whose turn (r-103d, d-8b4e). A tell to
// the human (人間, human) sends nothing: it marks what the member wrote in its
// pane as the person's, so the watcher does not ask who it is for (d-e601).
// --dry prints the message and sends and records nothing.
func runTell(env team.Env, opts *options.Options, stdout io.Writer) error {
	resolve := resolveTeam
	if opts.Dry {
		resolve = resolveTeamDry
	}
	r, err := resolve(env.Catalog(source.Disk{}), env.ProjectRoot, opts)
	if err != nil {
		return err
	}
	if turns.IsHuman(opts.Member) {
		return tellHuman(r, opts, stdout)
	}
	if opts.Body == "" {
		return i18n.Errorf("error.tell_needs_body")
	}
	member, err := r.member(opts.Member)
	if err != nil {
		return err
	}
	from := domainname.Seen{}
	if opts.From != "" {
		found, err := r.member(opts.From)
		if err != nil {
			return i18n.Errorf("error.tell_from_unknown", opts.From)
		}
		from = found.Name.Observation()
	} else if !opts.Dry {
		from = senderName(r, currentPane())
	}
	text := sendText(from.String(), opts.Body)
	if opts.Dry {
		fmt.Fprintf(stdout, i18n.T("send.dry"), member.Name.String(), text)
		return nil
	}
	if err := newLauncher(r, opts.Timeout, stdout).Tell(member, text); err != nil {
		return err
	}
	return recordTell(r.team.Name.Observation(), from, domainname.ShortIn(member.Name.Observation(), r.team.Name.Observation()).String())
}

// tellHuman records that the member whose pane hergy runs in (or --from) marked
// its text as the person's. Nothing is sent: the person reads the pane. From
// a pane that is no member's there is nothing to mark, and it is refused.
func tellHuman(r *resolved, opts *options.Options, stdout io.Writer) error {
	from := domainname.Seen{}
	if opts.From != "" {
		found, err := r.member(opts.From)
		if err != nil {
			return i18n.Errorf("error.tell_from_unknown", opts.From)
		}
		from = found.Name.Observation()
	} else if !opts.Dry {
		from = senderName(r, currentPane())
	}
	if opts.Dry {
		fmt.Fprint(stdout, i18n.T("send.dry_human"))
		return nil
	}
	if from.String() == "" {
		return i18n.Errorf("error.tell_human_from_person")
	}
	if err := recordTell(r.team.Name.Observation(), from, turns.Human); err != nil {
		return err
	}
	fmt.Fprint(stdout, i18n.T("send.told_human"))
	return nil
}

// recordTell leaves the tell where the team's watcher takes it: who handed the
// turn to whom, the sender by herdr name (the person as ""), the recipient by
// short name or turns.Human. A tell nobody watches for is still recorded; the
// watcher drops what waited before it started.
func recordTell(teamName, fromHerdr domainname.Seen, to string) error {
	dir, err := screenlog.DefaultDir()
	if err != nil {
		return err
	}
	from := ""
	if fromHerdr.String() != "" {
		from = domainname.ShortIn(fromHerdr, teamName).String()
	}
	return turns.WriteTell(turns.TellDir(dir, teamName.String()), turns.Tell{From: from, To: to, At: time.Now()})
}
