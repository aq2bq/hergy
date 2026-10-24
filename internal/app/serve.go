package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aq2bq/hergy/internal/browser"
	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
)

// breakLine writes one broken settings file as one line: where it is, as a
// person writes it, and why, with the whole reason on one line.
func breakLine(env team.Env, path string, err error) string {
	return check.BreakLine(env.Display, path, err)
}

// listingPath is the team.yaml of one layer's row of the listing.
func listingPath(row catalog.Listing, layer string) string {
	if layer == source.LayerGlobal {
		return row.GlobalPath
	}
	return row.LocalPath
}

// herdrTabList lists the tabs herdr reports, with the labels a team is found by.
func herdrTabList() ([]protocol.Tab, error) {
	port := herdrcli.New(os.Getenv("HERGY_HERDR"))
	return herdr.New(port).Tabs()
}

// runServe serves the screens on this machine and prints the URL once the socket
// is open, then opens that URL in the person's default browser. It runs in the
// foreground until Ctrl-C, then waits for the requests in flight.
//
// A browser that cannot be opened is one line on stderr and nothing else: the
// screens are already answering, so the command goes on and the line carries the
// URL to open by hand (d-fb89).
func runServe(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	// A projects: entry hergy cannot read is a broken setting, and a serve that
	// started anyway would show a headquarters that silently lost a project
	// (d-73fc). The language was checked before this ran.
	cfg, err := env.ReadConfigFile()
	if err != nil {
		return err
	}
	if _, _, err := env.ProjectPaths(cfg); err != nil {
		return fmt.Errorf("%s: %w", env.DisplayPath(env.ConfigPath()), err)
	}
	cat := env.Catalog(source.Disk{})
	read := newActivityRead(env)
	read.ctx = ctx
	server, err := web.New(cat, read)
	if err != nil {
		return err
	}
	return server.Serve(ctx, opts.Port, func(url string) {
		fmt.Fprintln(stdout, i18n.Tf("serve.url", url))
		if opts.NoOpen {
			return
		}
		if err := openBrowser(url); err != nil {
			fmt.Fprintln(stderr, i18n.Tf("serve.no_browser", url))
		}
	})
}

// openBrowser opens a URL in the person's browser. It is internal/browser's
// Open, held as a variable so a test can watch what runServe asks for without a
// browser opening on the machine the tests run on.
var openBrowser = browser.Open

// serveClock is the moment the screens are read at. It is now, held as a
// variable so a run can place the moment its screens are read at rather than
// leaning on the wall clock it happens to sit at: the fixture's times are
// counted from this moment, and a moment that is not today's is written with its
// date (spec-activity 6 A4). A run read around midnight would otherwise want
// "22:50 より前" where the screen, rightly, writes "10/20 22:50 より前"
// (r-f647).
var serveClock = time.Now
