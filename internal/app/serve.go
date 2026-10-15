package app

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aq2bq/hergy/internal/config"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
	"github.com/aq2bq/hergy/internal/web/rulesview"
)

// servePort is what the app hands the screens: the same layers, the same team
// definitions and the same rule files the commands read. The screens ask here
// and never reach around it, which is how internal/web stays out of the CLI's
// edge and out of hergy's top layer.
type servePort struct{ env team.Env }

// Teams lists one layer's team definitions with what the team list shows: how
// many members team.yaml writes, how many rules those members read, and how many
// of those rules have no check. A definition that cannot be read keeps its row
// and carries why, written the way hergy validate writes a broken file.
func (p servePort) Teams(layer string) []web.Team {
	var out []web.Team
	for _, row := range p.env.List() {
		switch layer {
		case web.LayerGlobal:
			if !row.Global {
				continue
			}
		default:
			if !row.Local {
				continue
			}
		}
		cfg, err := validateTeam(p.env, row.Name, layer)
		if err != nil {
			out = append(out, web.Team{
				Name:   row.Name,
				Broken: breakLine(p.env, listingPath(row, layer), err),
			})
			continue
		}
		tally := ruleTally(cfg)
		out = append(out, web.Team{
			Name:    row.Name,
			Members: len(cfg.Members),
			Rules:   tally.Total,
			NoCheck: tally.Total - tally.WithCheck,
		})
	}
	return out
}

// LayerRoot is where one layer's files live, as a person writes it.
func (p servePort) LayerRoot(layer string) string {
	if layer == web.LayerGlobal {
		return p.env.Display(p.env.GlobalDir)
	}
	return p.env.Display(p.env.LocalDir)
}

// listingPath is the team.yaml of one layer's row of the listing.
func listingPath(row team.Listing, layer string) string {
	if layer == web.LayerGlobal {
		return row.GlobalPath
	}
	return row.LocalPath
}

// breakLine writes one broken settings file as one line: where it is, as a
// person writes it, and why, with the whole reason on one line. hergy validate
// and the web UI both show a broken file this way.
func breakLine(env team.Env, path string, err error) string {
	msg := strings.ReplaceAll(err.Error(), path, env.Display(path))
	// Some team-file errors do not name the file; the file-level parser errors
	// already do. Show the location once either way.
	if !strings.Contains(msg, env.Display(path)) {
		msg = fmt.Sprintf("%s: %s", env.Display(path), msg)
	}
	return strings.Join(strings.Fields(msg), " ")
}

// runServe serves the screens on this machine and prints the URL once the socket
// is open. It runs in the foreground until Ctrl-C, then waits for the requests
// in flight. It reads the settings the way the other commands read them and
// changes no file the screens do not write.
func runServe(ctx context.Context, env team.Env, opts *options.Options, stdout io.Writer) error {
	port := servePort{env: env}
	rulesScreen, err := rulesview.New(port)
	if err != nil {
		return err
	}
	server, err := web.New(port, rulesScreen)
	if err != nil {
		return err
	}
	return server.Serve(ctx, opts.Port, func(url string) {
		fmt.Fprintln(stdout, i18n.Tf("serve.url", url))
	})
}

// ruleTally counts what one team's members read, the way hergy validate counts
// the whole machine.
func ruleTally(cfg *config.Config) *rules.Tally {
	tally := &rules.Tally{}
	collectRules(tally, map[string]bool{}, cfg)
	return tally
}
