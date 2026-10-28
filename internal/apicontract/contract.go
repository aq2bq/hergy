package apicontract

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

//go:embed openapi.yaml
var specYAML []byte

//go:embed pending.txt
var pendingTxt string

// Spec is the parsed promise, validated as an OpenAPI document.
func Spec() (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(specYAML)
	if err != nil {
		return nil, fmt.Errorf("openapi.yaml: %w", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("openapi.yaml: %w", err)
	}
	return doc, nil
}

// Pending is the operations whose shape is not written yet, by operationId.
func Pending() map[string]bool {
	out := map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(pendingTxt))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			out[line] = true
		}
	}
	return out
}

// Checker is the handler wrapper the tests put in front of the server, and the
// failures it has seen. It is safe for the concurrent requests a test makes.
type Checker struct {
	router  routers.Router
	pending map[string]bool
	mu      sync.Mutex
	failed  []string
}

// NewChecker reads the promise. A promise that does not read stops the tests.
func NewChecker() (*Checker, error) {
	doc, err := Spec()
	if err != nil {
		return nil, err
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, err
	}
	return &Checker{router: router, pending: Pending()}, nil
}

// Failures is what the checked requests and answers broke, sorted and without
// repeats.
func (c *Checker) Failures() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	seen := map[string]bool{}
	out := []string{}
	for _, f := range c.failed {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// Report is the end of a test binary's run (goleak.Cleanup): it prints what
// the checked calls broke and exits non-zero when anything did, so a broken
// promise fails the package even when every test passed.
func (c *Checker) Report(code int) {
	failures := c.Failures()
	if len(failures) > 0 {
		fmt.Fprintln(os.Stderr, "API contract (internal/apicontract/openapi.yaml):")
		for _, f := range failures {
			fmt.Fprintln(os.Stderr, "  "+f)
		}
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func (c *Checker) fail(format string, args ...any) {
	c.mu.Lock()
	c.failed = append(c.failed, fmt.Sprintf(format, args...))
	c.mu.Unlock()
}

// Wrap puts the check in front of next. Only /api/ is checked; the pages and
// the static files are not part of the promise.
func (c *Checker) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		route, params, err := c.router.FindRoute(r)
		if err != nil {
			c.fail("%s %s: not in openapi.yaml (%v)", r.Method, r.URL.Path, err)
			next.ServeHTTP(w, r)
			return
		}
		id := route.Operation.OperationID
		if c.pending[id] {
			next.ServeHTTP(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		check := r.Clone(r.Context())
		check.Body = io.NopCloser(bytes.NewReader(body))
		in := &openapi3filter.RequestValidationInput{
			Request: check, PathParams: params, Route: route,
			Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc, MultiError: true},
		}
		if err := openapi3filter.ValidateRequest(r.Context(), in); err != nil {
			c.fail("%s (%s %s) request: %v", id, r.Method, route.Path, err)
		}
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)
		for k, vs := range rec.Header() {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
		out := &openapi3filter.ResponseValidationInput{
			RequestValidationInput: in, Status: rec.Code, Header: rec.Header(),
			Body:    io.NopCloser(bytes.NewReader(rec.Body.Bytes())),
			Options: &openapi3filter.Options{IncludeResponseStatus: true, MultiError: true},
		}
		if err := openapi3filter.ValidateResponse(r.Context(), out); err != nil {
			c.fail("%s (%s %s) answer %d: %v", id, r.Method, route.Path, rec.Code, err)
		}
	})
}
