package teamview

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/url"
	"strconv"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/status"
	"github.com/aq2bq/hergy/internal/web"
)

// templates and stylesheet are embedded: hergy serve is one binary, and what a
// screen shows comes from it.
//
//go:embed templates/*.html
var templateFiles embed.FS

//go:embed teamview.css
var stylesheet embed.FS

// assets are the files these screens bring: one stylesheet, served under
// /static/teamview/ and linked in the frame's head. The frame mounts a name
// once, so one screen names it and the others use its classes.
var assets = web.Assets{
	Name:        "teamview",
	Files:       stylesheet,
	Stylesheets: []string{"teamview.css"},
}

// Screens builds the screens of this package, in the order the navigation shows
// them. The templates are parsed here, so a broken one stops hergy serve at
// once rather than on the first request.
func Screens(port Port) ([]web.Screen, error) {
	templates, err := parse()
	if err != nil {
		return nil, err
	}
	base := base{port: port, templates: templates}
	return []web.Screen{
		&definitionScreen{base: base},
		&runningScreen{base: base},
		&figureScreen{base: base},
	}, nil
}

func parse() (*template.Template, error) {
	return template.New("").Funcs(web.Funcs()).Funcs(funcs()).ParseFS(templateFiles, "templates/*.html")
}

// funcs are the template functions these screens add to the frame's own: the
// panel a link points at, a figure coordinate, a shortened label, a list, and
// one value escaped for a URL.
func funcs() template.FuncMap {
	return template.FuncMap{
		"panel": func() string { return "#" + web.PanelID },
		"num":   num,
		"cut":   cut,
		"join":  join,
		"q":     url.QueryEscape,
	}
}

// base is what the three screens share: the port they read through and the
// templates they render with.
type base struct {
	port      Port
	templates *template.Template
}

// body renders one named template into the body of a page: the frame goes
// around it.
func (b base) body(name string, data any) (web.View, error) {
	rendered, err := b.render(name, data)
	if err != nil {
		return web.View{}, err
	}
	return web.View{Body: web.HTML(rendered)}, nil
}

// bare renders one named template into a fragment: what an HTMX swap puts in
// the right panel, or what replaces a part of the page.
func (b base) bare(name string, data any) (web.View, error) {
	rendered, err := b.render(name, data)
	if err != nil {
		return web.View{}, err
	}
	return web.View{Body: web.HTML(rendered), Bare: true}, nil
}

// missing is the panel a person gets when what a link named is no longer there:
// the reason, in the panel. It answers 200 with a fragment on purpose, so the
// swap that asked for it shows why rather than nothing.
func (b base) missing(line string) (web.View, error) {
	return b.bare("problem.html", line)
}

func (b base) render(name string, data any) (string, error) {
	var out bytes.Buffer
	if err := b.templates.ExecuteTemplate(&out, name, data); err != nil {
		return "", err
	}
	return out.String(), nil
}

// choose is the definition a screen shows: the one the URL names, else the
// first of the layer. The layer's definitions come back for the picker.
func (b base) choose(layer, name string) (string, []web.Team) {
	teams := b.port.Teams(layer)
	if name == "" && len(teams) > 0 {
		name = teams[0].Name
	}
	return name, teams
}

// states are the states Herdr reports, in the order the legend lists them.
var states = []string{"working", "idle", "blocked", "done"}

// word is one state as the legend writes it: the state the pill is coloured by,
// the word the pill writes (Herdr's own word, or hergy's word for a name with no
// agent), and the line that says what it means.
type word struct {
	State   string
	Text    string
	Meaning string
}

// legend is every state a row can show, "gone" last: the pill hergy status
// prints for that state, and the one line that says what it means.
func legend() []word {
	out := make([]word, 0, len(states)+1)
	for _, state := range states {
		out = append(out, word{State: state, Text: state, Meaning: stateMeaning(state)})
	}
	return append(out, word{State: "gone", Text: i18n.T("status.not_live"), Meaning: stateMeaning("gone")})
}

// stateMeaning is what one state means, in the one line the legend and the
// member panel write.
func stateMeaning(state string) string {
	return i18n.T("web.teamview.state." + state)
}

// stateWord is the word one row's state shows: what Herdr reports, or "gone"
// for a name with no live agent (hergy status prints the same distinction).
func stateWord(row status.Row) string {
	if !row.Live || row.State == "" {
		return "gone"
	}
	return row.State
}

// sentence is one row's sub line in the running list: the role and the engine,
// then what the terminal says it is doing.
func sentence(row status.Row) string {
	role := row.Role
	if row.Worker {
		role = i18n.T("web.teamview.worker")
	}
	return dot(role, row.Kind, row.Title)
}

// dot joins the parts of one line the way these screens write them, leaving out
// what is empty.
func dot(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, i18n.T("web.teamview.separator"))
}

// num writes a figure coordinate: a whole number without a decimal point, a
// fraction with as many digits as it needs.
func num(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// cut shortens a label to at most n runes, with an ellipsis when it cuts: a box
// in the figure is a fixed width, and a long name would run out of it.
func cut(text string, n int) string {
	runes := []rune(text)
	if len(runes) <= n || n < 2 {
		return text
	}
	return string(runes[:n-1]) + "…"
}

// join writes a list the way hergy writes lists.
func join(words []string) string { return strings.Join(words, i18n.T("list.separator")) }

// textWidth is how wide a run of text is in the figure's units: a wide (CJK)
// character counts 12 pixels and a narrow one 7.3. SVG cannot measure text, so a
// box drawn around text is drawn from this.
func textWidth(text string) float64 {
	var w float64
	for _, r := range text {
		if wide(r) {
			w += 12
			continue
		}
		w += 7.3
	}
	return w
}

// wide reports whether a rune takes a full cell, the ranges a terminal and a
// font treat as double width.
func wide(r rune) bool {
	switch {
	case r <= 0x1100:
		return false
	case r <= 0x115f, r == 0x2329, r == 0x232a:
		return true
	case r >= 0x2e80 && r <= 0xa4cf, r >= 0xac00 && r <= 0xd7a3:
		return true
	case r >= 0xf900 && r <= 0xfaff, r >= 0xfe30 && r <= 0xfe6f:
		return true
	case r >= 0xff00 && r <= 0xff60, r >= 0xffe0 && r <= 0xffe6:
		return true
	}
	return false
}

// pill is one state drawn inside a box of the figure: the word it writes, the
// line a pointer shows, and where its parts sit. The screen draws the rounded
// rectangle, its dot and its text at these numbers.
type pill struct {
	State        string
	Text         string
	Title        string
	X, Y, W, H   float64
	DotX, DotY   float64
	TextX, TextY float64
}

// newPill places one row's state inside a box: right-aligned on the box's top
// row, as wide as the word needs.
func newPill(row status.Row, boxX, boxY, boxW float64) pill {
	const (
		height = 22.0
		padX   = 28.0 // the dot, the gap after it, and the space before the edge
		inset  = 14.0
	)
	text := stateText(row)
	w := textWidth(text) + padX
	x := boxX + boxW - inset - w
	y := boxY + 11
	return pill{
		State: stateWord(row),
		Text:  text,
		Title: stateMeaning(stateWord(row)),
		X:     x,
		Y:     y,
		W:     w,
		H:     height,
		DotX:  x + 12,
		DotY:  y + height/2,
		TextX: x + 22,
		TextY: y + height/2 + 4,
	}
}

// notice is the one line a screen shows when Herdr could not be read: the
// reason the command wrote, wrapped in the sentence around it.
func notice(err error) string {
	if err == nil {
		return ""
	}
	return i18n.Tf("web.teamview.herdr_failed", fmt.Sprintf("%v", err))
}
