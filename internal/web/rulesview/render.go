package rulesview

import (
	"bytes"
	"embed"
	"html/template"
	"maps"
	"strings"

	"github.com/aq2bq/hergy/internal/web"
)

// The templates are embedded: hergy serve is one binary, and the rules screen
// comes from it.
//
//go:embed templates/*.html
var files embed.FS

// Renderer draws the bodies this screen hands back, using the WebUI's template
// functions (web.Funcs) so the words and the one inline rule are the same here
// as everywhere else. It is a separate type so a test can build it with the
// functions a test needs, and so the screen can be parsed before a request
// rather than during one.
type Renderer struct {
	templates *template.Template
}

// NewRenderer parses the screen's templates with funcs. It adds the paths the
// forms and the links point at (p) and join, so the handlers and the templates
// share one spelling of a route.
func NewRenderer(funcs template.FuncMap) (*Renderer, error) {
	merged := make(template.FuncMap, len(funcs)+3)
	maps.Copy(merged, funcs)
	merged["p"] = func(name string) string { return paths[name] }
	merged["panelid"] = func() string { return "#" + web.DrawerID }
	merged["drawerid"] = func() string { return web.DrawerID }
	merged["join"] = strings.Join
	templates, err := template.New("rulesview").Funcs(merged).ParseFS(files, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Renderer{templates: templates}, nil
}

// paths are the routes the templates ask for by name, so a path is written once.
var paths = map[string]string{
	"rules":     PathRules,
	"ruleslist": PathRulesList,
	"panel":     PathPanel,
	"paneledit": PathPanelEdit,
	"preview":   PathPreview,
	"save":      PathSave,
	"delete":    PathDelete,
	"report":    PathReport,
	"bump":      PathBump,
}

// Rules draws the rules screen's body: the files, the numbers, the filters and
// the rules.
func (r *Renderer) Rules(data Rules) (string, error) {
	return r.render("rules.html", data)
}

// RulesList draws the rules list alone, inside its #rules-list element: what the
// search box swaps in while a person types, so the frame around it does not
// change and the cursor stays where it is.
func (r *Renderer) RulesList(data Rules) (string, error) {
	return r.render("rules-list", data)
}

// Panel draws the edit panel's body: a fragment for the right panel.
func (r *Renderer) Panel(data Panel) (string, error) {
	return r.render("panel.html", data)
}

// Preview draws the preview of the rule being edited: the section as a member
// reads it, and the break the reader names, when there is one.
func (r *Renderer) Preview(data Panel) (string, error) {
	return r.render("preview.html", data)
}

// Close draws the right panel with nothing open: the drawer element as the
// frame writes it, so a fragment can put it back the way it was.
func (r *Renderer) Close() (string, error) {
	return r.render("drawer-empty.html", nil)
}

// PanelFailed draws the right panel with a reason and nothing else: what the
// panel answers when the rule it was asked for cannot be opened, so the person
// sees why instead of nothing (r-06a1).
func (r *Renderer) PanelFailed(data PanelFailed) (string, error) {
	return r.render("panel-failed.html", data)
}

func (r *Renderer) render(name string, data any) (string, error) {
	var buf bytes.Buffer
	if err := r.templates.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
