package apicontract

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// TestSpecReads: openapi.yaml is a valid OpenAPI document.
func TestSpecReads(t *testing.T) {
	if _, err := Spec(); err != nil {
		t.Fatal(err)
	}
}

var routeLine = regexp.MustCompile(`mux\.HandleFunc\("(GET|POST|PUT|DELETE) (/api/[^"]+)"`)
var param = regexp.MustCompile(`\{[^}]+\}`)

// kinds is what /api/{kind} serves (internal/web/api.go).
var kinds = []string{"team", "role", "profile", "activity"}

// TestEveryRouteIsWritten: every /api/ route server.go registers is an
// operation in openapi.yaml, and every operation there is a route. A route
// added without its promise, or a promise left behind by a route that went,
// fails here.
func TestEveryRouteIsWritten(t *testing.T) {
	src, err := os.ReadFile("../web/server.go")
	if err != nil {
		t.Fatal(err)
	}
	routes := map[string]bool{}
	for _, m := range routeLine.FindAllStringSubmatch(string(src), -1) {
		method, path := m[1], m[2]
		if strings.Contains(path, "{kind}") {
			for _, k := range kinds {
				routes[method+" "+param.ReplaceAllString(strings.Replace(path, "{kind}", k, 1), "{}")] = true
			}
			continue
		}
		routes[method+" "+param.ReplaceAllString(path, "{}")] = true
	}
	doc, err := Spec()
	if err != nil {
		t.Fatal(err)
	}
	written := map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			written[method+" "+param.ReplaceAllString(path, "{}")] = true
		}
	}
	for _, r := range sorted(routes) {
		if !written[r] {
			t.Errorf("%s is served but not written in openapi.yaml", r)
		}
	}
	for _, w := range sorted(written) {
		if !routes[w] {
			t.Errorf("%s is written in openapi.yaml but not served", w)
		}
	}
}

// TestPendingIsOperations: every line of pending.txt names an operation.
func TestPendingIsOperations(t *testing.T) {
	doc, err := Spec()
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			ids[op.OperationID] = true
		}
	}
	for id := range Pending() {
		if !ids[id] {
			t.Errorf("pending.txt names %s, which openapi.yaml does not have", id)
		}
	}
}

// TestWrittenOperationsAreWhole: an operation that is not pending says its
// whole shape. Its request body and its answers have a schema, every object
// in them closes with additionalProperties: false (a field the other side
// does not know is an error, not a field quietly passed), and no schema is
// the empty one that lets anything through.
func TestWrittenOperationsAreWhole(t *testing.T) {
	doc, err := Spec()
	if err != nil {
		t.Fatal(err)
	}
	pending := Pending()
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			if pending[op.OperationID] {
				continue
			}
			where := op.OperationID + " (" + method + " " + path + ")"
			if op.RequestBody != nil && op.RequestBody.Value != nil {
				for ct, media := range op.RequestBody.Value.Content {
					wholeSchema(t, where+" request "+ct, media.Schema, map[*openapi3.Schema]bool{})
				}
			}
			if op.Responses == nil || op.Responses.Len() == 0 {
				t.Errorf("%s: no answer written", where)
				continue
			}
			for status, resp := range op.Responses.Map() {
				if resp.Value == nil {
					continue
				}
				if status == "default" && resp.Value.Description != nil && strings.Contains(*resp.Value.Description, "pending") {
					t.Errorf("%s: still has the pending answer", where)
				}
				for ct, media := range resp.Value.Content {
					wholeSchema(t, where+" answer "+status+" "+ct, media.Schema, map[*openapi3.Schema]bool{})
				}
			}
		}
	}
}

func wholeSchema(t *testing.T, where string, ref *openapi3.SchemaRef, seen map[*openapi3.Schema]bool) {
	t.Helper()
	if ref == nil || ref.Value == nil {
		t.Errorf("%s: no schema", where)
		return
	}
	s := ref.Value
	if seen[s] {
		return
	}
	seen[s] = true
	empty := s.Type == nil && len(s.AllOf) == 0 && len(s.OneOf) == 0 && len(s.AnyOf) == 0 && s.Not == nil && len(s.Properties) == 0 && s.Items == nil && len(s.Enum) == 0
	if empty && !s.Nullable {
		t.Errorf("%s: the empty schema lets anything through", where)
	}
	if s.Type.Is("object") || len(s.Properties) > 0 {
		if s.AdditionalProperties.Has == nil || *s.AdditionalProperties.Has {
			if s.AdditionalProperties.Schema == nil {
				t.Errorf("%s: an object without additionalProperties: false", where)
			}
		}
	}
	for name, p := range s.Properties {
		wholeSchema(t, where+"."+name, p, seen)
	}
	if s.Items != nil {
		wholeSchema(t, where+"[]", s.Items, seen)
	}
	if s.AdditionalProperties.Schema != nil {
		wholeSchema(t, where+"{}", s.AdditionalProperties.Schema, seen)
	}
	for _, list := range [][]*openapi3.SchemaRef{s.AllOf, s.OneOf, s.AnyOf} {
		for _, x := range list {
			wholeSchema(t, where, x, seen)
		}
	}
}

func sorted(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
