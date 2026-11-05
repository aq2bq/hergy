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
// profiles.yaml — or, where the file cannot be read, the one row that stands
// for the file. Both the list answers and the band's numbers are built from
// ListedRows, and a number is the number of rows: nothing else counts rows.
//
// 読めない（壊れた・権限の無い）文書の行も、一覧に出して件数に数える（d-17ab、
// q-a8a3 の答え A）。行を出すかは rowShown の一行が決め、理由は ReadableOf が返す。

// ListedRow is one row of a list: a name in one place.
type ListedRow struct {
	Kind   string // "team", "role" or "profile"
	Name   string
	Place  string // catalog.PlaceProject or catalog.PlaceGlobal
	Path   string // the document the row stands on, empty for a name without one
	Legacy bool   // role: only a legacy .yaml stands behind the name
	NoDoc  bool   // role: no layer holds a document; a member names it
	// Problem is why a row that stands for a whole file (a profiles.yaml that
	// cannot be read) cannot be read. The row is drawn and counted anyway
	// (r-ea17, d-17ab).
	Problem string
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

// rowShown is the one place the unreadable-row decision lives: every row stands
// and is counted, whether or not its document reads (d-17ab).
func (c *Catalog) rowShown(_ ListedRow) bool { return true }

// ReadableOf is whether the row's document can be read, with the sentence a
// list shows beside a row that cannot (d-17ab: the row stands and is counted).
func (c *Catalog) ReadableOf(row ListedRow) (bool, string) {
	if row.Problem != "" {
		return false, row.Problem
	}
	if row.Legacy || row.NoDoc || row.Path == "" {
		return true, ""
	}
	_, ok, err := c.fileText(row.Path)
	if err == nil && ok {
		return true, ""
	}
	if err != nil {
		return false, err.Error()
	}
	return false, ""
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

// profileRows is the rows one layer's profiles.yaml draws. A file that is not
// there is empty and draws none; a file that cannot be read, or that names no
// profile because it cannot be read as profiles.yaml, draws the one row that
// stands for the file, with the reason (r-ea17, d-17ab).
func (c *Catalog) profileRows(layer string) []ListedRow {
	dir := c.LayerDir(layer)
	path := filepath.Join(dir, "profiles.yaml")
	place := placeOfLayer(layer)
	text, ok, err := c.fileText(path)
	if err != nil {
		return []ListedRow{{Kind: KindProfile, Name: filepath.Base(path), Place: place, Path: path, Problem: err.Error()}}
	}
	if !ok {
		return nil
	}
	entries, problems := profile.ReadAll(text, c.Display(path))
	if len(entries) == 0 {
		for _, problem := range problems {
			if problem.Sev == profile.Err {
				return []ListedRow{{Kind: KindProfile, Name: filepath.Base(path), Place: place, Path: path, Problem: problem.Text}}
			}
		}
	}
	out := make([]ListedRow, 0, len(entries))
	for _, entry := range entries {
		out = append(out, ListedRow{Kind: KindProfile, Name: entry.Name, Place: place, Path: path})
	}
	return out
}

func placeOfLayer(layer string) string {
	if layer == source.LayerGlobal {
		return PlaceGlobal
	}
	return PlaceProject
}
