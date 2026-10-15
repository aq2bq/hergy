package homeview

import (
	"bytes"
	"html/template"
	"net/http"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/web"
)

// Bump is one document whose content moved since the last launch: the version it
// holds, the version a bump writes, and the team definition whose next launch
// hands the change over.
type Bump struct {
	// Path is the document, as a person writes it, and File is where it is on
	// disk: the form the screen writes carries File back.
	Path string
	File string
	// Layer is which layer the document came from (web.LayerLocal,
	// web.LayerGlobal): a document of the global layer reaches every project.
	Layer string
	// From is the version the document holds now (0 when the file writes none),
	// and To is the one a bump writes.
	From int
	To   int
	// Definition is the team definition to run after the bump, empty when no
	// definition of the layer reads the document.
	Definition string
	// Already is set when the version is already ahead of the one the launch that
	// read the document read: it has been raised since, so the screen offers no
	// button and says so.
	Already bool
}

// Answer is what a write answers with: the version it raised, or why it wrote
// nothing. A page left open, a resend or a double press raises nothing, so the
// screen shows the reason instead of a second bump (M3).
type Answer struct {
	// Written is set when a version was raised.
	Written bool
	// Raised is the document whose version was raised, with the version it moved
	// from and to; the zero value when nothing was written.
	Raised Bump
	// Why is why nothing was written, as one line for the screen: the launch
	// record does not hold the document, its content has not moved since the
	// launch that read it, or its version has been raised since that launch.
	Why string
}

// Bumps are the documents whose content moved since the last launch: what the
// bump screen lists, each with the version a bump writes. A document whose text
// the last launch read but the record does not hold back is not here: the two
// cannot be compared, so the screen cannot say it moved.
func (h Home) Bumps() []Bump {
	var out []Bump
	for _, doc := range h.Docs {
		if doc.Missing {
			continue
		}
		if len(doc.Added)+len(doc.Removed)+len(doc.Changed) == 0 && !doc.Whole {
			continue
		}
		out = append(out, Bump{
			Path:       doc.Path,
			File:       doc.File,
			Layer:      doc.Layer,
			From:       doc.Version,
			To:         doc.Version + 1,
			Definition: doc.Definition,
			Already:    doc.Bumped,
		})
	}
	return out
}

// bumpScreen raises a document's version (spec/10 の 7 節): it lists the
// documents whose content moved since the last launch, and each one's version
// n → n+1 is written by the app, through the same writer the rules screen uses.
type bumpScreen struct {
	port      Port
	templates *template.Template
}

// NewBump builds the screen that raises a version. It parses the template here,
// so a broken one stops hergy serve at once instead of on the first request.
func NewBump(port Port) (web.Screen, error) {
	templates, err := template.New("").Funcs(web.Funcs()).ParseFS(files, "templates/bump.html")
	if err != nil {
		return nil, err
	}
	return &bumpScreen{port: port, templates: templates}, nil
}

// Name is what the navigation calls the screen.
func (s *bumpScreen) Name() string { return i18n.T("web.bump.title") }

// Routes are the page and the write behind it. The write answers the page again,
// so a browser with no script raises a version and reads what to run next
// without a second screen of its own.
func (s *bumpScreen) Routes() []web.Route {
	return []web.Route{
		{Path: "/bump", Handler: s.show},
		{Path: "/bump/save", Write: true, Handler: s.raise},
	}
}

func (s *bumpScreen) show(r *http.Request) (web.View, error) {
	return s.page(web.LayerOf(r), web.Token(r), Answer{})
}

// raise writes one document's version and answers the page with the line that
// says what hands the change over. A write the server refuses because the
// document cannot be raised (the record does not hold it, it has not moved, or
// its version was already raised) answers the page with the reason, and a write
// the writer refuses (a file that moved under the screen, a path no member
// reads) is an error, which the frame shows with its reason.
func (s *bumpScreen) raise(r *http.Request) (web.View, error) {
	layer := web.LayerOf(r)
	answer, err := s.port.Bump(layer, r.PostFormValue("doc"))
	if err != nil {
		return web.View{}, err
	}
	return s.page(layer, web.Token(r), answer)
}

// page renders the screen: what moved, what a bump writes, and, after a write,
// what the bump did.
func (s *bumpScreen) page(layer, token string, answer Answer) (web.View, error) {
	home := s.port.Home(layer)
	data := bumpData{
		LayerName: i18n.Tf("web.layer.name", i18n.T("label."+layer)),
		Layer:     layer,
		Token:     token,
		Home:      home,
		Bumps:     home.Bumps(),
		Answer:    answer,
	}
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, "bump.html", data); err != nil {
		return web.View{}, err
	}
	return web.View{Title: i18n.T("web.bump.title"), Body: web.HTML(buf.String())}, nil
}

// bumpData is what the bump template is given.
type bumpData struct {
	// LayerName is the layer being shown, as a person says it.
	LayerName string
	// Layer is the layer being shown, for the write's own URL.
	Layer string
	// Token is this run's token: every form carries it.
	Token string
	// Home is what the app read for this layer. The screen takes the documents
	// that moved from it, and it is what says whether a launch has happened at
	// all and why the record could not be read.
	Home Home
	// Bumps is the documents whose content moved, and the version a bump writes.
	Bumps []Bump
	// Answer is what the last write did, or why it did nothing. The zero value
	// on the page that was not written to.
	Answer Answer
}
