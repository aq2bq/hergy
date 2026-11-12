package cli

import (
	"fmt"
	"io"

	"github.com/aq2bq/hergy/internal/ops"

	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/source"
)

func runMigrate(env ops.Environment, opts *options.Options, out io.Writer) error {
	layer := source.LayerLocal
	if opts.Global {
		layer = source.LayerGlobal
	}
	plan, err := ops.MigrateResources(env.Catalog(source.Disk{}).LayerDir(layer), opts.Dry)
	for _, file := range plan {
		fmt.Fprintf(out, "%s → %s (backup: %s)\n", file.Path, file.Destination, file.Backup)
	}
	return err
}
