package profileview

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/aq2bq/hergy/internal/config"
	"github.com/aq2bq/hergy/internal/i18n"
)

// Row is one profile as the form holds it: the four fields as they were typed.
// The screen reads the rows into profiles only when it saves, so what stands on
// the page reaches a file only through SaveBody.
type Row struct {
	// Name is the profile's name, and Kind the engine it starts.
	Name string
	Kind string
	// Args are the arguments, one to a line, and Env the environment variables,
	// one name=value to a line: the way the form holds them (r-daf8).
	Args string
	Env  string
	// Over and Shadowed are the marks the other layer gives this name: Over for
	// a local profile the global layer also holds, Shadowed for a global one a
	// local profile of the same name takes the place of. They are read from
	// disk, so a name the file does not hold yet carries none.
	Over     bool
	Shadowed bool
}

// blank says a row holds nothing: it is the empty row the add button opened,
// which a save leaves out. A row of spaces counts as nothing here, so a person
// can empty a row by clearing its fields.
func (r Row) blank() bool {
	return strings.TrimSpace(r.Name) == "" && strings.TrimSpace(r.Kind) == "" &&
		strings.TrimSpace(r.Args) == "" && strings.TrimSpace(r.Env) == ""
}

// EditBody draws the page again from the form the person is in, with the row the
// add button opened added, or the row the delete button names taken out. It
// writes nothing.
func (s *Screen) EditBody(layer, token string, values url.Values) (string, error) {
	return s.page(pageState{
		layer: layer,
		token: token,
		rows:  applyRowAction(rowsOf(values), values),
		hash:  values.Get("hash"),
	})
}

// SaveBody is the one write of this screen (r-daf8, spec/10 の 4 節): it reads
// the rows the form holds, writes them back as the layer's profiles.yaml, and
// draws the page with what happened. Nothing is written when
//
//   - the form carries no hash (the page was not one hergy drew),
//   - the rows cannot be read as profiles (a profile with no name, a name
//     written twice, an env line that is not name=value), or
//   - the check hergy validate runs refuses the file hergy would write.
//
// The page then comes back with the reason and the rows as they were typed, so
// nothing the person wrote is lost. A file that changed on disk since it was
// read is refused by the Port, which says the same thing in its own words.
func (s *Screen) SaveBody(layer, token string, values url.Values) (string, error) {
	rows := rowsOf(values)
	refuse := func(reason string) (string, error) {
		return s.page(pageState{
			layer:  layer,
			token:  token,
			rows:   rows,
			hash:   values.Get("hash"),
			reason: reason,
		})
	}
	if values.Get("hash") == "" {
		return refuse(i18n.T("web.profiles.error.no_hash"))
	}
	profiles, err := profilesOf(rows)
	if err != nil {
		return refuse(err.Error())
	}
	text, err := config.EmitProfiles(profiles)
	if err != nil {
		return refuse(err.Error())
	}
	// The check hergy validate runs on a profiles.yaml, over the text hergy
	// would write: a save that would leave a file hergy cannot read back is not
	// made (r-daf8).
	if reason := s.port.CheckProfiles(layer, text); reason != "" {
		return refuse(reason)
	}
	if err := s.port.WriteProfiles(layer, text, values.Get("hash")); err != nil {
		return refuse(err.Error())
	}
	return s.page(pageState{layer: layer, token: token, notice: i18n.T("web.profiles.saved")})
}

// rowsOf reads the rows of the form: the four fields of a row are the values
// the browser sent under one name, in the order the rows stand on the page.
func rowsOf(values url.Values) []Row {
	names := values["name"]
	rows := make([]Row, 0, len(names))
	for i := range names {
		rows = append(rows, Row{
			Name: names[i],
			Kind: valueAt(values, "kind", i),
			Args: valueAt(values, "args", i),
			Env:  valueAt(values, "env", i),
		})
	}
	return rows
}

// valueAt is the i-th value of one field of the form, empty when the form
// carried fewer values than rows.
func valueAt(values url.Values, field string, i int) string {
	if i < len(values[field]) {
		return values[field][i]
	}
	return ""
}

// applyRowAction is what the two buttons that are not the save do to the rows on
// the page: the add button opens one more empty row, and the delete button names
// the place of the row it takes out. A place that is not on the page leaves the
// rows as they are.
func applyRowAction(rows []Row, values url.Values) []Row {
	if values.Get("add") != "" {
		return append(rows, Row{})
	}
	place := values.Get("drop")
	if place == "" {
		return rows
	}
	i, err := strconv.Atoi(place)
	if err != nil || i < 0 || i >= len(rows) {
		return rows
	}
	out := make([]Row, 0, len(rows)-1)
	out = append(out, rows[:i]...)
	return append(out, rows[i+1:]...)
}

// markRows puts the marks of the layer's file on the rows the form holds, by
// name: a row whose name the file does not hold yet (a new profile, or one the
// person renamed) carries no mark until it is saved. The marks are read from
// disk, so they are what the other layer holds, not what the form says.
func markRows(rows []Row, profiles []Profile) {
	marks := make(map[string]Profile, len(profiles))
	for _, profile := range profiles {
		marks[profile.Name] = profile
	}
	for i := range rows {
		mark, ok := marks[strings.TrimSpace(rows[i].Name)]
		if !ok {
			continue
		}
		rows[i].Over, rows[i].Shadowed = mark.Over, mark.Shadowed
	}
}

// profilesOf reads the rows of the form as the profiles they hold. An empty row
// is left out; a row whose name is empty but holds something else is refused
// rather than written under a name the person did not give; a name written twice
// is refused, because the file would hold only one of them.
//
// The names, the kinds, the args and the env are checked by the same check
// hergy validate runs (CheckProfiles), over the file hergy would write.
func profilesOf(rows []Row) (map[string]config.Profile, error) {
	out := make(map[string]config.Profile, len(rows))
	for i, row := range rows {
		if row.blank() {
			continue
		}
		name := strings.TrimSpace(row.Name)
		if name == "" {
			return nil, i18n.Errorf("web.profiles.error.no_name", i+1)
		}
		if _, twice := out[name]; twice {
			return nil, i18n.Errorf("web.profiles.error.duplicate", name)
		}
		env, err := envOf(row.Env)
		if err != nil {
			return nil, err
		}
		out[name] = config.Profile{
			Name: name,
			Kind: strings.TrimSpace(row.Kind),
			Args: argsOf(row.Args),
			Env:  env,
		}
	}
	return out, nil
}

// argsOf reads the args of one row: one argument to a line, an empty line left
// out. The ends of an argument are trimmed, so an indented line is the same
// argument as one at the margin (試作 profilesView の args).
func argsOf(text string) []string {
	var out []string
	for _, line := range lines(text) {
		out = append(out, strings.TrimSpace(line))
	}
	return out
}

// envOf reads the env of one row: one NAME=value to a line, the name trimmed
// and the value kept as it was typed. A line with no = is refused rather than
// dropped in silence, and a name written twice is refused, because the file
// would hold only one of them.
func envOf(text string) (map[string]string, error) {
	var out map[string]string
	for _, line := range lines(text) {
		name, value, found := strings.Cut(line, "=")
		name = strings.TrimSpace(name)
		if !found || name == "" {
			return nil, i18n.Errorf("web.profiles.error.env_line", strings.TrimSpace(line))
		}
		if _, twice := out[name]; twice {
			return nil, i18n.Errorf("web.profiles.error.env_duplicate", name)
		}
		if out == nil {
			out = map[string]string{}
		}
		out[name] = value
	}
	return out, nil
}

// lines is the non-empty lines of a textarea: what a person leaves blank in the
// form is not an argument and not a variable.
func lines(text string) []string {
	var out []string
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

// joinLines is how the form holds a list: one item to a line. It is the other
// half of argsOf and envOf.
func joinLines(items []string) string {
	return strings.Join(items, "\n")
}
