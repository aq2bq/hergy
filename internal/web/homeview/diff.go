package homeview

import (
	"bytes"
	"html/template"
	"net/http"

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

// diffScreen is the prompt diff: one team definition at a time, one member at a
// time, the prompt that was last handed against the prompt the files build now.
type diffScreen struct {
	port      Port
	templates *template.Template
}

// NewDiff builds the prompt diff screen.
func NewDiff(port Port) (web.Screen, error) {
	templates, err := template.New("").Funcs(web.Funcs()).ParseFS(files, "templates/diff.html")
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
	data := diffData{
		LayerName: i18n.Tf("web.layer.name", i18n.T("label."+layer)),
		Diff:      s.port.Diff(layer),
	}
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, "diff.html", data); err != nil {
		return web.View{}, err
	}
	return web.View{Title: i18n.T("web.diff.title"), Body: web.HTML(buf.String())}, nil
}

// diffData is what the diff template is given.
type diffData struct {
	// LayerName is the layer being shown, as a person says it.
	LayerName string
	// Diff is what the app read for this layer.
	Diff Diff
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
