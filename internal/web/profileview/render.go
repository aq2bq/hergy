package profileview

import (
	"bytes"
	"embed"
	"html/template"
	"maps"
)

// The template is embedded: hergy serve is one binary, and the profiles screen
// comes from it. It brings no stylesheet of its own (d-207f): the frame carries
// the one stylesheet, and this screen draws with its classes.
//
//go:embed templates/*.html
var files embed.FS

// Renderer draws the body this screen hands back, using the WebUI's template
// functions (web.Funcs) so the words are the same here as everywhere else. It
// is a separate type so a test can build it, and so the screen can be parsed
// before a request rather than during one.
type Renderer struct {
	templates *template.Template
}

// paths is the screen's routes by the name its template uses, so a handler and
// a form share one spelling of a path.
var paths = map[string]string{
	"page": ProfilePath,
	"edit": EditPath,
	"save": SavePath,
}

// NewRenderer parses the screen's template with funcs. It adds the paths the
// forms point at (p), so the handlers and the template share one spelling of a
// route.
func NewRenderer(funcs template.FuncMap) (*Renderer, error) {
	merged := make(template.FuncMap, len(funcs)+1)
	maps.Copy(merged, funcs)
	merged["p"] = func(name string) string { return paths[name] }
	templates, err := template.New("profileview").Funcs(merged).ParseFS(files, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Renderer{templates: templates}, nil
}

// Profiles draws the profiles page's body.
func (r *Renderer) Profiles(data Profiles) (string, error) {
	return r.render("profiles.html", data)
}

func (r *Renderer) render(name string, data any) (string, error) {
	var buf bytes.Buffer
	if err := r.templates.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
