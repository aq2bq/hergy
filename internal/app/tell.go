package app

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/screenlog"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/turns"
)

// runTell sends one member a message from a person or another member, and
// records the tell for the team's watcher: this is how a member hands its turn
// on, and how hergy knows who opened whose turn (r-103d, d-8b4e). --dry prints
// the message and sends and records nothing.
func runTell(env team.Env, opts *options.Options, stdout io.Writer) error {
	resolve := resolveTeam
	if opts.Dry {
		resolve = resolveTeamDry
	}
	r, err := resolve(env.Catalog(source.Disk{}), env.ProjectRoot, opts)
	if err != nil {
		return err
	}
	member, err := r.member(opts.Member)
	if err != nil {
		return err
	}
	from := ""
	if opts.From != "" {
		found, err := r.member(opts.From)
		if err != nil {
			return i18n.Errorf("error.tell_from_unknown", opts.From)
		}
		from = found.Name
	} else if !opts.Dry {
		from = senderName(r, os.Getenv("HERDR_PANE_ID"))
	}
	text := sendText(from, opts.Body)
	if opts.Dry {
		fmt.Fprintf(stdout, i18n.T("send.dry"), member.Name, text)
		return nil
	}
	if err := newLauncher(r, opts.Timeout, stdout).Tell(member, text); err != nil {
		return err
	}
	return recordTell(r.team.Name, from, member.Name)
}

// recordTell leaves the tell where the team's watcher takes it: who handed the
// turn to whom, by short names, the person as "". A tell nobody watches for is
// still recorded; the watcher drops what waited before it started.
func recordTell(teamName, fromHerdr, toHerdr string) error {
	dir, err := screenlog.DefaultDir()
	if err != nil {
		return err
	}
	from := ""
	if fromHerdr != "" {
		from = shortMember(fromHerdr, teamName)
	}
	return turns.WriteTell(turns.TellDir(dir, teamName), turns.Tell{From: from, To: shortMember(toHerdr, teamName), At: time.Now()})
}
