package web

import (
	"html/template"
	"net/url"
	"strconv"

	"github.com/aq2bq/hergy/internal/i18n"
)

// Guide draws the four steps that stand above the screens of the record (the
// prototype's guideBar): ① what moved, ② the rules, ③ the prompt diff, ④ the
// version. step is the one being shown, layer is the layer in view, changes is
// how many rules moved since the last launch (the last step names it), and doc
// is the document the rules step opens, when the screen has one.
//
// It is a template function rather than markup in each screen because one bar
// stands on four screens of two packages, and a bar that drifts between them is
// a bar nobody can compare. The four paths are written here rather than taken
// from the screens: this package is below every screen, and the paths are fixed
// (d-207f).
func Guide(step int, layer string, changes int, doc ...string) template.HTML {
	open := ""
	if len(doc) > 0 && doc[0] != "" {
		open = "&amp;doc=" + url.QueryEscape(doc[0])
	}
	query := "?layer=" + url.QueryEscape(layer)
	last := i18n.T("web.guide.step.bump")
	if changes > 0 {
		last += i18n.Tf("web.guide.step.bump_changes", changes)
	}
	steps := []struct {
		label string
		href  string
	}{
		{i18n.T("web.guide.step.changes"), "/" + query},
		{i18n.T("web.guide.step.fix"), "/rules" + query + open},
		{i18n.T("web.guide.step.diff"), "/diff" + query},
		{last, "/bump" + query},
	}
	var b []byte
	b = append(b, `<nav class="guide" aria-label="`...)
	b = append(b, template.HTMLEscapeString(i18n.T("web.guide.label"))...)
	b = append(b, `">`...)
	for i, one := range steps {
		if i > 0 {
			b = append(b, `<span class="garrow">→</span>`...)
		}
		n := i + 1
		class, current := "gstep", ""
		switch {
		case n == step:
			class, current = "gstep cur", ` aria-current="step"`
		case n < step:
			class = "gstep past"
		}
		b = append(b, `<a class="`+class+`" href="`+one.href+`"`+current+`><span class="gnum">`...)
		b = append(b, strconv.Itoa(n)...)
		b = append(b, `</span>`...)
		b = append(b, template.HTMLEscapeString(one.label)...)
		b = append(b, `</a>`...)
	}
	b = append(b, `</nav>`...)
	return HTML(string(b))
}
