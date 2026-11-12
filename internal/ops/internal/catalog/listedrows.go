package catalog

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
)

// This file decides which rows a list draws, and therefore what the band's
// numbers are (r-ea17, d-4d88). It lives under internal/ops/internal, so the
// entrances cannot count rows of their own: they ask ops.
//
// A row is one name in one place: a team definition's directory, a role's
// document (or, on the project's side, a name the list still shows without one:
// a legacy .yaml, or a name only a member names), and a profile in a layer's
// profiles.yaml — or, where the file cannot be read, the one row that stands
// for the file. Both the list answers and the band's numbers are built from
// listedRows, and a number is the number of rows: nothing else counts rows.
// A team folder with files but no team.yaml remains an unreadable row, but is
// not a definition or an unnamed command's candidate. Empty folders draw no row.
//
// 読めない（壊れた・権限の無い）文書の行も、一覧に出して件数に数える（d-17ab、
// q-a8a3 の答え A）。行を出すかは rowShown の一行が決め、理由は readableOf が返す。

// ListedRow is one row of a list: a name in one place.
type ListedRow struct {
	Kind   string // "team", "role" or "profile"
	Name   string
	Place  domainplace.Place // catalog.PlaceProject or catalog.PlaceGlobal
	Path   string            // the document the row stands on, empty for a name without one
	Legacy bool              // role: only a legacy .yaml stands behind the name
	NoDoc  bool              // role: no layer holds a document; a member names it
	// InEffect and Hidden are the place's answer for a name both layers hold:
	// the project's row is in effect and the global one is hidden. A name only
	// one layer holds carries neither (r-ea17, the same rule as before).
	InEffect bool
	Hidden   bool
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

// listedRows is every row one layer draws, for one kind, without reading a
// document: the row set is decided by names and the files that hold them.
func (c *Catalog) listedRows(kind, layer string) []ListedRow {
	rows := c.rowsOf(kind, layer)
	other := source.LayerLocal
	if layer == source.LayerLocal {
		other = source.LayerGlobal
	}
	both := map[string]bool{}
	for _, one := range c.rowsOf(kind, other) {
		if one.Problem == "" {
			both[one.Name] = true
		}
	}
	for i := range rows {
		if rows[i].Problem != "" || !both[rows[i].Name] {
			continue
		}
		rows[i].InEffect = layer == source.LayerLocal
		rows[i].Hidden = layer == source.LayerGlobal
	}
	return rows
}

// rowsOf is the kind's rows in one layer, before the two layers are compared.
func (c *Catalog) rowsOf(kind, layer string) []ListedRow {
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

// shownRows is the rows the list draws and the band counts. A row whose
// document cannot be read stands or falls by this one decision; the rest of the
// program neither adds nor removes rows for any other reason.
func (c *Catalog) shownRows(kind, layer string) []ListedRow {
	out := make([]ListedRow, 0, 8)
	for _, row := range c.listedRows(kind, layer) {
		if c.rowShown(row) {
			out = append(out, row)
		}
	}
	return out
}

// rowShown is the one place the unreadable-row decision lives: every row stands
// and is counted, whether or not its document reads (d-17ab).
func (c *Catalog) rowShown(_ ListedRow) bool { return true }

// FileState is whether a file is there, and whether it can be read, with the
// reason it cannot: the one place a screen asks before it writes (r-ea17,
// d-0bf1). A file that is not there is the empty file a save would make.
func (c *Catalog) FileState(path string) (there, readable bool, why string) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, true, ""
		}
		return true, false, err.Error()
	}
	_, ok, err := c.fileText(path)
	if err == nil && ok {
		return true, true, ""
	}
	if err != nil {
		return true, false, err.Error()
	}
	return true, false, ""
}

// StateUnreadable is the API's word for a place whose file cannot be read
// (r-ea17, d-0d00). It is written here once; the screens and the API copy it.
const StateUnreadable = "unreadable"

// Unreadable answers the API's two words for a path: why it cannot be read, and
// the state word (r-ea17, d-0d00). A path that reads answers ("", "").
func (c *Catalog) Unreadable(path string) (problem, state string) {
	if path == "" {
		return "", ""
	}
	if there, ok, why := c.FileState(path); !ok {
		return why, StateUnreadable
	} else if !there && filepath.Base(path) == "team.yaml" {
		// team.yaml is required in a nonempty definition folder. Other absent
		// documents remain optional files a save can create (FileState's rule).
		if files, err := os.ReadDir(filepath.Dir(path)); err == nil && len(files) > 0 {
			if _, err := os.Stat(path); err != nil {
				return err.Error(), StateUnreadable
			}
		}
	}
	return "", ""
}

// readableOf is whether the row's document can be read, with the sentence a
// list shows beside a row that cannot (d-17ab: the row stands and is counted).
func (c *Catalog) readableOf(row ListedRow) (bool, string) {
	if row.Problem != "" {
		return false, row.Problem
	}
	if row.Legacy || row.NoDoc || row.Path == "" {
		return true, ""
	}
	why, _ := c.Unreadable(row.Path)
	return why == "", why
}

func (c *Catalog) teamRows(layer string) []ListedRow {
	dir := c.LayerDir(layer)
	names := teamRowNames(dir)
	out := make([]ListedRow, 0, len(names))
	for _, name := range names {
		out = append(out, ListedRow{
			Kind: KindTeam, Name: name, Place: domainplace.Selection(layer).At(c.base),
			Path: filepath.Join(dir, "teams", name, "team.yaml"),
		})
	}
	return out
}

// teamRowNames reads child folders each time: their contents can change without
// changing the parent teams/ directory's version.
func teamRowNames(dir string) []string {
	root := filepath.Join(dir, "teams")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			continue
		}
		files, err := os.ReadDir(path)
		// A folder that cannot be read stays a row with its own read error.
		if len(files) > 0 || (err != nil && !errors.Is(err, os.ErrNotExist)) {
			names = append(names, entry.Name())
		}
	}
	return names // ReadDir's order is the list's order.
}

func (c *Catalog) roleRows(layer string) []ListedRow {
	out := make([]ListedRow, 0, 8)
	for _, entry := range c.Roles() {
		hasProject := entry.HasDocument && !entry.Outside
		switch layer {
		case source.LayerLocal:
			switch {
			case hasProject:
				out = append(out, ListedRow{Kind: KindRole, Name: entry.Name, Place: domainplace.In(c.base), Path: entry.Path})
			case !entry.HasDocument:
				// No .md of its own: a legacy .yaml, or a name only a member
				// names. The list draws one row of it on the project's side.
				legacyPath := ""
				legacy := false
				if doc, ok := c.LegacyRole(entry.Name); ok {
					legacy, legacyPath = true, doc.Path
				}
				out = append(out, ListedRow{Kind: KindRole, Name: entry.Name, Place: domainplace.In(c.base), Path: legacyPath, Legacy: legacy, NoDoc: !legacy})
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
				out = append(out, ListedRow{Kind: KindRole, Name: entry.Name, Place: domainplace.Global(), Path: path})
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
	place := domainplace.Selection(layer).At(c.base)
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

// ListRows returns the privately owned set of visible rows.
func ListRows(c *Catalog, kind, layer string) []ListedRow { return c.shownRows(kind, layer) }

// RowReadable reports the row document's readability.
func RowReadable(c *Catalog, row ListedRow) (bool, string) { return c.readableOf(row) }
