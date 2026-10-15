package homeview

import (
	"bytes"
	"html/template"
	"net/http"
	"net/url"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/web"
)

// Diff is the prompt diff screen's data.
type Diff struct {
	// Problem is why the record could not be read, as one line; empty when it
	// was read.
	Problem string
	// Definitions is one entry per team definition of the layer that reads.
	Definitions []Definition
}

// Definition is one team definition's prompt diff.
type Definition struct {
	// Name is the team definition name.
	Name string
	// Launched is whether this definition has been launched from this machine
	// yet; without one there is no prompt to compare with.
	Launched bool
	// Members is one entry per member of the definition.
	Members []Member
}

// Member is one member: the prompt it was last handed, and the prompt the files
// build now, as a line diff.
type Member struct {
	// Name is the member's herdr name ("demo-lead").
	Name string
	// Launched is whether a launch of this definition handed this member a
	// prompt.
	Launched bool
	// Lines is the diff when there is one; empty when the two prompts are the
	// same.
	Lines []Line
}

// Same reports whether nothing changed in the member's prompt, so the screen can
// fold the member away: every line of the diff is one both prompts have.
func (m Member) Same() bool {
	for _, line := range m.Lines {
		if line.Kind != Same {
			return false
		}
	}
	return true
}

// Add is how many lines the member's prompt gained, and Del how many it lost:
// the counts the member's chip shows.
func (m Member) Add() int { return count(m.Lines, Plus) }

// Del is how many lines the member's prompt lost.
func (m Member) Del() int { return count(m.Lines, Minus) }

// count is one kind of line in a diff.
func count(lines []Line, kind string) int {
	n := 0
	for _, line := range lines {
		if line.Kind == kind {
			n++
		}
	}
	return n
}

// Line is one line of a prompt diff.
type Line struct {
	// Kind is Same, Plus or Minus.
	Kind string
	// Text is the line itself, without its mark.
	Text string
}

// The kinds of a diff line: what both prompts have, what only the launched
// prompt has, and what only the current one has.
const (
	Same  = "same"
	Plus  = "plus"
	Minus = "minus"
)

// Mark is the one character a diff line starts with.
func (l Line) Mark() string {
	switch l.Kind {
	case Plus:
		return "+"
	case Minus:
		return "-"
	default:
		return " "
	}
}

// Folded is one row of the diff panel: a line (Kind Same, Plus or Minus, with
// its Mark and Text) or a folded run of unchanged lines (Kind Skip, with how
// many lines it stands for).
type Folded struct {
	// Kind is Same, Plus, Minus, or Skip.
	Kind string
	// Mark is "+", "-", or empty for a line that both prompts have.
	Mark string
	// Text is the line itself.
	Text string
	// N is how many unchanged lines a Skip row folds away.
	N int
}

// Skip is the kind of a folded run of unchanged lines.
const Skip = "skip"

// Fold turns a line diff into the rows the panel draws: a run of more than four
// unchanged lines keeps its first two and its last two, with one Skip row
// naming how many it folded (試作 diffHtml の flush). Shorter runs, and every
// changed line, are drawn as they are.
func Fold(lines []Line) []Folded {
	var out []Folded
	var run []Line
	flush := func() {
		switch {
		case len(run) > 4:
			for _, line := range run[:2] {
				out = append(out, folded(line))
			}
			out = append(out, Folded{Kind: Skip, N: len(run) - 4})
			for _, line := range run[len(run)-2:] {
				out = append(out, folded(line))
			}
		default:
			for _, line := range run {
				out = append(out, folded(line))
			}
		}
		run = nil
	}
	for _, line := range lines {
		if line.Kind == Same {
			run = append(run, line)
			continue
		}
		flush()
		out = append(out, folded(line))
	}
	flush()
	return out
}

// folded is one changed or unchanged line as a row.
func folded(line Line) Folded {
	row := Folded{Kind: line.Kind, Text: line.Text}
	if line.Kind != Same {
		row.Mark = line.Mark()
	}
	return row
}

// diffScreen is the prompt diff: one member at a time, the prompt that was last
// handed against the prompt the files build now.
type diffScreen struct {
	port      Port
	templates *template.Template
}

// NewDiff builds the prompt diff screen.
func NewDiff(port Port) (web.Screen, error) {
	templates, err := template.New("").Funcs(web.Funcs()).
		ParseFS(files, "templates/diff.html")
	if err != nil {
		return nil, err
	}
	return &diffScreen{port: port, templates: templates}, nil
}

// Name is what the navigation calls the screen.
func (s *diffScreen) Name() string { return i18n.T("web.diff.title") }

// Routes answers the diff path.
func (s *diffScreen) Routes() []web.Route {
	return []web.Route{{Path: "/diff", Handler: s.show}}
}

func (s *diffScreen) show(r *http.Request) (web.View, error) {
	layer := web.LayerOf(r)
	diff := s.port.Diff(layer)
	data := diffData{
		LayerName: i18n.Tf("web.layer.name", i18n.T("label."+layer)),
		Layer:     layer,
		Diff:      diff,
		// The guide's last step names how many rules moved; the diff itself
		// does not carry that, so the home of the layer is read for it.
		ChangedRules: changedRules(s.port.Home(layer)),
	}
	for _, def := range diff.Definitions {
		if !def.Launched {
			data.NoLaunch = append(data.NoLaunch, def.Name)
		}
		for _, member := range def.Members {
			data.Rows = append(data.Rows, diffRowOf(layer, def.Name, member))
		}
	}
	selectMember(&data, r.URL.Query().Get("member"))
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, "diff.html", data); err != nil {
		return web.View{}, err
	}
	return web.View{Title: i18n.T("web.diff.title"), Body: web.HTML(buf.String())}, nil
}

// selectMember marks the member the panel shows: the one the query names, else
// the first whose prompt changed, else the first.
func selectMember(data *diffData, want string) {
	pick := -1
	for i := range data.Rows {
		if want != "" && data.Rows[i].key == want {
			pick = i
			break
		}
	}
	if pick < 0 {
		for i := range data.Rows {
			if data.Rows[i].Add+data.Rows[i].Del > 0 {
				pick = i
				break
			}
		}
	}
	if pick < 0 && len(data.Rows) > 0 {
		pick = 0
	}
	if pick < 0 {
		return
	}
	data.Rows[pick].Selected = true
	data.Selected = &data.Rows[pick]
}

// diffRow is one member as the diff screen shows it: where its chip points, what
// its counts are, and the rows of its prompt diff.
type diffRow struct {
	// key is what the ?member= query carries: "<team>/<member>".
	key string
	// Team is the team definition, and Member the member's herdr name.
	Team   string
	Member string
	// Layer is the layer being shown, as a person says it.
	Layer string
	// Href is where this member's chip points.
	Href string
	// Selected is set for the one member the panel shows.
	Selected bool
	// Never is set when no launch handed this member a prompt, and Same when
	// the launched prompt and the current one agree.
	Never bool
	Same  bool
	// Add and Del are how many lines the prompt gained and lost.
	Add int
	Del int
	// Rows is the diff, folded, when there is one to draw.
	Rows []Folded
}

// diffRowOf builds one member's row. The layer is the one being shown: every
// definition of the layer belongs to it.
func diffRowOf(layer, team string, member Member) diffRow {
	row := diffRow{
		key:    team + "/" + member.Name,
		Team:   team,
		Member: member.Name,
		Layer:  layer,
		Never:  !member.Launched,
		Add:    member.Add(),
		Del:    member.Del(),
	}
	row.Same = !row.Never && member.Same()
	row.Href = "/diff?layer=" + url.QueryEscape(layer) + "&member=" + url.QueryEscape(row.key)
	if !row.Never && !row.Same {
		row.Rows = Fold(member.Lines)
	}
	return row
}

// diffData is what the diff template is given.
type diffData struct {
	// LayerName is the layer being shown, as a person says it.
	LayerName string
	// Layer is the layer being shown, as the URLs carry it.
	Layer string
	// Diff is what the app read for this layer.
	Diff Diff
	// ChangedRules is how many rules moved since the last launch, over the
	// documents of the layer; the guide's last step names it.
	ChangedRules int
	// Rows is one entry per member of every definition, in the app's order.
	Rows []diffRow
	// Selected is the one member the panel shows, nil when there is none.
	Selected *diffRow
	// NoLaunch is the team definitions with no launch record, by name.
	NoLaunch []string
}

// DiffLines is the line diff of two prompts: the lines both have, and the lines
// only one of them has, as the longest common subsequence of the two. The
// standard library has no diff and hergy adds no dependency for one; a prompt is
// a few hundred lines, so the table is small. A text far larger than that is
// compared coarsely (every line out, every line in) rather than building a table
// nobody wants to hold.
func DiffLines(before, after []string) []Line {
	if len(before)*len(after) > maxCells {
		return append(only(Minus, before), only(Plus, after)...)
	}
	// common[i][j] is the length of the longest common subsequence of
	// before[i:] and after[j:], filled from the end.
	common := make([][]int, len(before)+1)
	for i := range common {
		common[i] = make([]int, len(after)+1)
	}
	for i := len(before) - 1; i >= 0; i-- {
		for j := len(after) - 1; j >= 0; j-- {
			if before[i] == after[j] {
				common[i][j] = common[i+1][j+1] + 1
				continue
			}
			common[i][j] = max(common[i+1][j], common[i][j+1])
		}
	}
	var out []Line
	i, j := 0, 0
	for i < len(before) && j < len(after) {
		switch {
		case before[i] == after[j]:
			out = append(out, Line{Kind: Same, Text: before[i]})
			i++
			j++
		case common[i+1][j] >= common[i][j+1]:
			out = append(out, Line{Kind: Minus, Text: before[i]})
			i++
		default:
			out = append(out, Line{Kind: Plus, Text: after[j]})
			j++
		}
	}
	return append(append(out, only(Minus, before[i:])...), only(Plus, after[j:])...)
}

// maxCells is how big a diff table may get before a comparison turns coarse:
// with ints, a million cells is a few megabytes.
const maxCells = 1 << 20

// only is one side's lines, for the coarse diff and for the tails of the fine
// one.
func only(kind string, lines []string) []Line {
	out := make([]Line, 0, len(lines))
	for _, text := range lines {
		out = append(out, Line{Kind: kind, Text: text})
	}
	return out
}
