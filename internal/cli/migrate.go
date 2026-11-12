package cli

import (
	"fmt"
	"io"

	"github.com/aq2bq/hergy/internal/ops"

	"github.com/aq2bq/hergy/internal/migrate"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/source"
)

func runMigrate(env ops.Environment, opts *options.Options, out io.Writer) error {
	layer := source.LayerLocal
	if opts.Global {
		layer = source.LayerGlobal
	}
	plan, err := migrate.Plan(env.Catalog(source.Disk{}).LayerDir(layer))
	if err != nil {
		return err
	}
	for _, file := range plan {
		fmt.Fprintf(out, "%s → %s (backup: %s)\n", file.Path, file.Destination, file.Backup)
	}
	if opts.Dry {
		return nil
	}
	for _, file := range plan {
		if err := file.Apply(); err != nil {
			return fmt.Errorf("%s: %w", file.Path, err)
		}
	}
	return nil
}
