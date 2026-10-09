// Command hergy starts and refreshes agent teams in Herdr, one level above the
// pane: teams, roles and the turn.
package main

import (
	"os"

	"github.com/aq2bq/hergy/internal/app"
)

func main() {
	os.Exit(app.Run(os.Args[1:], os.Stdout, os.Stderr))
}
