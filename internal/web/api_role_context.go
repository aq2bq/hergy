package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
)

type apiRoleUse struct {
	Project     string   `json:"project"`
	ProjectName string   `json:"project_name"`
	Place       string   `json:"place"`
	Name        string   `json:"name"`
	Members     []string `json:"members"`
}

type apiRoleContext struct {
	Name    string       `json:"name"`
	Project string       `json:"project"`
	Place   string       `json:"place"`
	Path    string       `json:"path"`
	State   string       `json:"state"`
	Text    string       `json:"text"`
	Hash    string       `json:"hash"`
	Doc     *r2Doc       `json:"doc,omitempty"`
	Uses    []apiRoleUse `json:"uses"`
	Scanned []string     `json:"scanned"`
	Errors  []string     `json:"errors"`
	Problem string       `json:"problem"`
}

func (s *Server) apiRoleContext(w http.ResponseWriter, r *http.Request) {
	context := r.URL.Query().Get("context")
	if context != "" && context != "project" && context != "global" {
		apiAnswer(w, nil, refuseAt("place", t("web.rename.place")))
		return
	}
	name := r.PathValue("name")
	if name == "" || catalog.RoleNameProblem(name) != nil {
		apiAnswer(w, nil, refuseAt("name", t("web.role_context.name")))
		return
	}
	scoped, refusal := s.withProject(projectQuery(r))
	if refusal != nil {
		apiAnswer(w, nil, refusal)
		return
	}
	cat := scoped.cat
	if r.URL.Query().Get("context") == "global" {
		cat = s.globalCatalog()
	}
	out := scoped.roleContext(name, cat, projectQuery(r), r.URL.Query().Get("context") == "global")
	apiAnswer(w, out, nil)
}

func (s *Server) roleContext(name string, cat *catalog.Catalog, project string, global bool) apiRoleContext {
	if project == "" && !global {
		project = cat.Base()
	}
	out := apiRoleContext{Name: name, Project: project, Place: "project", State: "missing", Uses: []apiRoleUse{}, Scanned: []string{}, Errors: []string{}, Problem: t("web.role_context.missing")}
	doc, found := cat.Role(name)
	if !found {
		out.Path = cat.Display(cat.RolePath("project", name))
		if global {
			out.Place = "global"
			out.Path = cat.Display(cat.RolePath("global", name))
		}
		return out
	}
	if doc.Layer == source.LayerGlobal {
		out.Place = "global"
	}
	out.Path = cat.Display(doc.Path)
	text, err := cat.Reader().Read(doc)
	if err != nil {
		out.State = "unreadable"
		out.Problem = err.Error()
		return out
	}
	out.Text, out.Hash = text, source.Hash(text)
	parsed, problems := rules.ReadAll(text, out.Path)
	if parsed == nil {
		out.State = "unreadable"
		out.Problem = t("web.role_context.unreadable")
		if len(problems) > 0 {
			out.Problem = problems[0].Text
		}
		return out
	}
	one := r2DocOf(parsed)
	out.Doc = one
	out.State = "readable"
	out.Problem = ""
	if out.Place == "project" {
		out.addUses(cat, doc.Path, project, filepath.Base(cat.Base()), "project")
		return out
	}
	out.addUses(s.globalCatalog(), doc.Path, "", "グローバル", "global")
	if s.hq == nil {
		out.Errors = append(out.Errors, t("web.role_context.usage_failed"))
		return out
	}
	projects, err := s.hq.Projects()
	if err != nil {
		out.Errors = append(out.Errors, err.Error())
		return out
	}
	for _, p := range hqShownProjects(projects) {
		one, refusal := s.catalogAt(p.Path)
		if refusal != nil {
			out.Errors = append(out.Errors, p.Name+": "+refusal.Text)
			continue
		}
		out.addUses(one, doc.Path, p.Path, p.Name, "project")
	}
	return out
}

func (out *apiRoleContext) addUses(cat *catalog.Catalog, path, project, label, place string) {
	out.Scanned = append(out.Scanned, label)
	role, ok := cat.Role(out.Name)
	if !ok {
		return
	}
	actual, err := os.Stat(role.Path)
	if err != nil {
		out.Errors = append(out.Errors, err.Error())
		return
	}
	target, err := os.Stat(path)
	if err != nil {
		out.Errors = append(out.Errors, err.Error())
		return
	}
	if !os.SameFile(actual, target) {
		return
	}
	for _, listing := range cat.Definitions() {
		places := []string{}
		if place == "project" && listing.Local {
			places = append(places, "project")
		}
		if listing.Global {
			places = append(places, "global")
		}
		for _, defPlace := range places {
			def, err := cat.Definition(listing.Name, catalog.Layer(defPlace))
			if def == nil {
				if err != nil {
					out.Errors = append(out.Errors, label+" / "+listing.Name+": "+err.Error())
				}
				continue
			}
			use := apiRoleUse{Project: project, ProjectName: label, Place: defPlace, Name: listing.Name, Members: []string{}}
			for _, m := range def.Graph.Members {
				if m.Role == out.Name {
					use.Members = append(use.Members, m.Name)
				}
			}
			if len(use.Members) > 0 {
				out.Uses = append(out.Uses, use)
			}
		}
	}
}

func (s *Server) roleContextStillCurrent(in apiIn) *apiErr {
	if in.RoleContext == "" {
		return nil
	}
	if in.RoleContext != "project" && in.RoleContext != "global" {
		return refuseAt("place", t("web.rename.place"))
	}
	var doc r2Doc
	if err := json.Unmarshal(in.Doc, &doc); err != nil {
		return refuseAt("doc", err.Error())
	}
	if doc.Name == "" || catalog.RoleNameProblem(doc.Name) != nil {
		return refuseAt("name", t("web.role_context.name"))
	}
	cat := s.cat
	if in.RoleContext == "global" {
		cat = s.globalCatalog()
	}
	resolved, ok := cat.Role(doc.Name)
	if !ok || catalog.Layer(in.Place) != resolved.Layer {
		return refuse(http.StatusConflict, t("web.role_context.changed"))
	}
	return nil
}
