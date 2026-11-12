// Command hergy starts and refreshes agent teams in Herdr, one level above the
// pane: teams, roles and the turn.
package main

import (
	"os"

	"github.com/aq2bq/hergy/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
