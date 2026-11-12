package catalog

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aq2bq/hergy/internal/source"
)

// A residual folder is a list row, not a definition or an unnamed command's
// candidate. Check both a held catalog and one opened after the folder appears.
func TestResidualTeamFoldersDoNotChangeDefinitionsOrDefault(t *testing.T) {
	for _, layer := range []string{source.LayerLocal, source.LayerGlobal} {
		t.Run(layer, func(t *testing.T) {
			l := newLayers(t)
			l.writeFullTeam(t, layer, "one", "the only definition")
			held := l.catalog()
			before := held.Definitions()
			_, defaultErr := held.Definition("", "")
			_, readErr := held.Read("")
			for _, dir := range []string{l.local, l.global} {
				write(t, filepath.Join(dir, "teams", "leftover", "notes.txt"), "keep me\n")
			}
			for _, cat := range []*Catalog{held, l.catalog()} {
				if got := cat.Definitions(); !reflect.DeepEqual(got, before) {
					t.Errorf("Definitions = %v, want %v", got, before)
				}
				if got := cat.candidateNames(); !reflect.DeepEqual(got, []string{"one"}) {
					t.Errorf("candidate names = %v, want only one", got)
				}
				if def, err := cat.Definition("", layer); err != nil || def.Name != "one" {
					t.Errorf("unnamed %s definition = %v, %v; want one", layer, def, err)
				}
				if def, err := cat.Definition("", ""); fmt.Sprint(err) != fmt.Sprint(defaultErr) || err == nil && def.Name != "one" {
					t.Errorf("unnamed definition = %v, %v; previous error %v", def, err, defaultErr)
				}
				if def, err := cat.Read(""); fmt.Sprint(err) != fmt.Sprint(readErr) || err == nil && def.Name != "one" {
					t.Errorf("unnamed read = %v, %v; previous error %v", def, err, readErr)
				}
				for _, rowLayer := range []string{source.LayerLocal, source.LayerGlobal} {
					found := false
					for _, row := range cat.ListedRows(KindTeam, rowLayer) {
						if row.Name != "leftover" {
							continue
						}
						found = true
						if ok, why := cat.ReadableOf(row); ok || why == "" {
							t.Errorf("residual row in %s = readable %v, reason %q", rowLayer, ok, why)
						}
					}
					if !found {
						t.Errorf("ListedRows in %s omitted the residual folder", rowLayer)
					}
				}
			}
		})
	}
}
