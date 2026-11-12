package ops

import "github.com/aq2bq/hergy/internal/ops/internal/catalog"

// ListResourceRows is the only row-set read available to entry points. The
// private catalog distinguishes definitions from unreadable leftover folders.
func ListResourceRows(cat *catalog.Catalog, kind, layer string) []catalog.ListedRow {
	return catalog.ListRows(cat, kind, layer)
}

// ResourceRowReadable reads or changes the selected resource.
func ResourceRowReadable(cat *catalog.Catalog, row catalog.ListedRow) (bool, string) {
	return catalog.RowReadable(cat, row)
}

// ResourceCounts reads or changes the selected resource.
func ResourceCounts(cat *catalog.Catalog, layer string) (int, int, int) {
	return catalog.CountRows(cat, layer)
}
