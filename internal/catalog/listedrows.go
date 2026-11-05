package catalog

import (
	"path/filepath"

	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
)

// This file is the owner of one concept (AGENTS.md 概念の地図): which rows a
// list draws, and therefore what the band's numbers are (r-ea17, d-4d88).
//
// A row is one name in one place: a team definition's directory, a role's
// document (or, on the project's side, a name the list still shows without one:
// a legacy .yaml, or a name only a member names), and a profile in a layer's
// profiles.yaml. Both the list answers and the band's numbers are built from
// ListedRows, and a number is the number of rows: nothing else counts rows.
//
// 読めない文書を持つ行を出すかは、いまは利用者への確認（q-a8a3）の答え待ち。
// 答えが来たら rowShown の一行だけを変える（A: 出す・数える、B: 出さない・数えない）。

// ListedRow is one row of a list: a name in one place.
type ListedRow struct {
	Kind   string // "team", "role" or "profile"
	Name   string
	Place  string // catalog.PlaceProject or catalog.PlaceGlobal
	Path   string // the document the row stands on, empty for a name without one
	Legacy bool   // role: only a legacy .yaml stands behind the name
	NoDoc  bool   // role: no layer holds a document; a member names it
}

// Bands' kinds, as the API and the screens name them.
const (
	KindTeam    = "team"
	KindRole    = "role"
	KindProfile = "profile"
)

// ListedRows is every row one layer draws, for one kind, without reading a
// document: the row set is decided by names and the files that hold them.
func (c *Catalog) ListedRows(kind, layer string) []ListedRow {
	switch kind {
	case KindTeam:
		return c.teamRows(layer)
	case KindRole:
		return c.roleRows(layer)
	case KindProfile:
		return c.profileRows(layer)
	}
	return nil
}

// ShownRows is the rows the list draws and the band counts. A row whose
// document cannot be read stands or falls by this one decision; the rest of the
// program neither adds nor removes rows for any other reason.
func (c *Catalog) ShownRows(kind, layer string) []ListedRow {
	out := make([]ListedRow, 0, 8)
	for _, row := range c.ListedRows(kind, layer) {
		if c.rowShown(row) {
			out = append(out, row)
		}
	}
	return out
}

// rowShown is the one place the unreadable-row decision lives. Until the
// master's answer, it keeps what each list does today: a team definition's row
// stands even when its team.yaml does not read, a role's row stands only when
// its document reads, and a profile's row stands when its file reads (the file
// is parsed to know the names at all).
func (c *Catalog) rowShown(row ListedRow) bool {
	if row.Kind == KindTeam {
		return true
	}
	if row.Legacy || row.NoDoc {
		return true
	}
	return row.Path != "" && c.roleReads(row.Path)
}

// roleReads is whether one role document can be read.
func (c *Catalog) roleReads(path string) bool {
	_, ok, err := c.fileText(path)
	return ok && err == nil
}

func (c *Catalog) teamRows(layer string) []ListedRow {
	dir := c.LayerDir(layer)
	names := c.teamNames(dir)
	out := make([]ListedRow, 0, len(names))
	for _, name := range names {
		out = append(out, ListedRow{
			Kind: KindTeam, Name: name, Place: placeOfLayer(layer),
			Path: filepath.Join(dir, "teams", name, "team.yaml"),
		})
	}
	return out
}

func (c *Catalog) roleRows(layer string) []ListedRow {
	out := make([]ListedRow, 0, 8)
	for _, entry := range c.Roles() {
		hasProject := entry.HasDocument && !entry.Outside
		switch layer {
		case source.LayerLocal:
			switch {
			case hasProject:
				out = append(out, ListedRow{Kind: KindRole, Name: entry.Name, Place: PlaceProject, Path: entry.Path})
			case !entry.HasDocument:
				// No .md of its own: a legacy .yaml, or a name only a member
				// names. The list draws one row of it on the project's side.
				legacyPath := ""
				legacy := false
				if doc, ok := c.LegacyRole(entry.Name); ok {
					legacy, legacyPath = true, doc.Path
				}
				out = append(out, ListedRow{Kind: KindRole, Name: entry.Name, Place: PlaceProject, Path: legacyPath, Legacy: legacy, NoDoc: !legacy})
			}
		case source.LayerGlobal:
			path := ""
			switch {
			case entry.Outside:
				path = entry.Path
			case entry.UnusedPath != "":
				path = entry.UnusedPath
			}
			if path != "" {
				out = append(out, ListedRow{Kind: KindRole, Name: entry.Name, Place: PlaceGlobal, Path: path})
			}
		}
	}
	return out
}

func (c *Catalog) profileRows(layer string) []ListedRow {
	dir := c.LayerDir(layer)
	path := filepath.Join(dir, "profiles.yaml")
	text, ok, err := c.fileText(path)
	if err != nil || !ok {
		return nil
	}
	parsed, err := profile.ParseProfiles(text, c.Display(path))
	if err != nil {
		return nil
	}
	out := make([]ListedRow, 0, len(parsed))
	for name := range parsed {
		out = append(out, ListedRow{Kind: KindProfile, Name: name, Place: placeOfLayer(layer), Path: path})
	}
	return out
}

func placeOfLayer(layer string) string {
	if layer == source.LayerGlobal {
		return PlaceGlobal
	}
	return PlaceProject
}
