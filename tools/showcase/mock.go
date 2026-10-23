// The mock in front of the serve: a prototype's files are put into the screens
// the serve answers, so a new idea is looked at inside the showcase rather than
// in a file beside it.
//
// It is a proxy on 127.0.0.1: every request goes to the serve unchanged, except
// that an HTML answer for a chosen screen carries the prototype's stylesheets and
// scripts, and the prototype's own files are answered at /mock/<name>. The page
// stays one origin, so the screens' Content-Security-Policy (default-src
// 'self') still holds: the prototype must write its styles as classes or through
// the CSSOM, never as a style attribute.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

const (
	// mockConfigFile is the one file a mock directory may hold to choose the
	// screens and the files. Without it every screen gets every file.
	mockConfigFile = "mock.yaml"
	// mockPrefix is where the prototype's files are answered, on the serve's own
	// origin.
	mockPrefix = "/mock/"
	// mockLimit is the most one HTML answer may be, so a large page cannot fill
	// memory in front of the serve.
	mockLimit = 8 << 20
	// mockGrace is how long the proxy waits for its requests on a stop.
	mockGrace = 5 * time.Second
)

// mockConfig is mock.yaml: the screens the files go into, and the files in the
// order they go in. Both may be empty, which is every screen and every .css then
// .js in the directory.
type mockConfig struct {
	Version int      `yaml:"version"`
	Paths   []string `yaml:"paths"`
	Inject  []string `yaml:"inject"`
}

// mock is the proxy in front of one serve.
type mock struct {
	dir     string // the prototype's directory
	backend string // the serve's host:port
	files   []string
	paths   []string // empty: every screen
	url     string
	server  *http.Server
}

// startMock reads the directory, starts the proxy on 127.0.0.1:port, and returns
// it. backend is the serve's host:port, as its own URL names it.
func startMock(dir, backend string, port int) (*mock, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("--mock の %s が読めません: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("--mock の %s はディレクトリではありません", dir)
	}
	cfg, err := readMockConfig(dir)
	if err != nil {
		return nil, err
	}
	files, err := mockFiles(dir, cfg)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("--mock の %s に差し込む .css も .js もありません", dir)
	}
	m := &mock{dir: dir, backend: backend, files: files, paths: cleanPaths(cfg.Paths)}
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("試作の受け口を開けません: %w", err)
	}
	m.url = "http://" + ln.Addr().String()
	m.server = &http.Server{Handler: m, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = m.server.Serve(ln) }()
	return m, nil
}

// stop ends the proxy and its requests in flight.
func (m *mock) stop() {
	ctx, cancel := context.WithTimeout(context.Background(), mockGrace)
	defer cancel()
	_ = m.server.Shutdown(ctx)
}

// readMockConfig reads mock.yaml, or answers the empty config that takes every
// screen and every file.
func readMockConfig(dir string) (mockConfig, error) {
	raw, err := os.ReadFile(filepath.Join(dir, mockConfigFile))
	if errors.Is(err, fs.ErrNotExist) {
		return mockConfig{Version: 1}, nil
	}
	if err != nil {
		return mockConfig{}, fmt.Errorf("%s を読めません: %w", mockConfigFile, err)
	}
	var cfg mockConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return mockConfig{}, fmt.Errorf("%s: %w", mockConfigFile, err)
	}
	if cfg.Version != 1 {
		return mockConfig{}, fmt.Errorf("%s の version は 1 にしてください（%d が書かれています）", mockConfigFile, cfg.Version)
	}
	return cfg, nil
}

// mockFiles is the files to put in, in the order they go in: the ones mock.yaml
// names, or every .css then every .js of the directory, by name.
func mockFiles(dir string, cfg mockConfig) ([]string, error) {
	if len(cfg.Inject) > 0 {
		for _, name := range cfg.Inject {
			if strings.ContainsAny(name, `/\`) {
				return nil, fmt.Errorf("%s の inject にディレクトリは書けません（%q）", mockConfigFile, name)
			}
			if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
				return nil, fmt.Errorf("%s の inject の %q が読めません: %w", mockConfigFile, name, err)
			}
		}
		return cfg.Inject, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var css, js []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".css":
			css = append(css, entry.Name())
		case ".js":
			js = append(js, entry.Name())
		}
	}
	sort.Strings(css)
	sort.Strings(js)
	return append(css, js...), nil
}

// cleanPaths keeps the screens' paths as prefixes: /teams stands for /teams and
// the screens under it, and a trailing slash is not part of the comparison.
func cleanPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, one := range paths {
		one = strings.TrimSuffix(strings.TrimSpace(one), "/")
		if one == "" {
			continue
		}
		if !strings.HasPrefix(one, "/") {
			one = "/" + one
		}
		out = append(out, one)
	}
	return out
}

// ServeHTTP serves the prototype's own files, and hands every other request to
// the serve, putting the files into an HTML answer for a chosen screen.
func (m *mock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if name, ok := strings.CutPrefix(r.URL.Path, mockPrefix); ok {
		m.serveFile(w, name)
		return
	}
	m.forward(w, r)
}

// serveFile answers one file of the prototype's directory. Only the files at the
// top of the directory are served: a name with a slash is not one of them.
func (m *mock) serveFile(w http.ResponseWriter, name string) {
	if name == "" || path.Base(name) != name {
		http.Error(w, "ショーケースの試作に、そのファイルはありません", http.StatusNotFound)
		return
	}
	raw, err := os.ReadFile(filepath.Join(m.dir, name))
	if err != nil {
		http.Error(w, "ショーケースの試作に、そのファイルはありません", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", mockType(name))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(raw) //nolint:gosec // G705: the bytes are the person's own prototype file, served as its own type
}

// mockType is one file's content type, by its extension.
func mockType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".css":
		return "text/css; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".html":
		return "text/html; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

// forward hands one request to the serve. The request is the browser's own,
// except that it is addressed to the serve (its guard reads the Host) and a
// write's Origin names the serve, which is the address the same screen has when
// the mock is not there.
func (m *mock) forward(w http.ResponseWriter, r *http.Request) {
	out := r.Clone(r.Context())
	out.URL.Scheme = "http"
	out.URL.Host = m.backend
	out.Host = m.backend
	out.RequestURI = ""
	out.Header.Del("Connection")
	out.Header.Del("Keep-Alive")
	out.Header.Del("Upgrade")
	out.Header.Del("Accept-Encoding")
	if origin := r.Header.Get("Origin"); origin != "" {
		out.Header.Set("Origin", "http://"+m.backend)
	}
	resp, err := http.DefaultTransport.RoundTrip(out)
	if err != nil {
		http.Error(w, "ショーケースの serve に届きません: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if !m.injectsInto(r.URL.Path, resp) {
		copyHeader(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, mockLimit))
	if err != nil {
		http.Error(w, "画面を読めません: "+err.Error(), http.StatusBadGateway)
		return
	}
	page := m.inject(string(raw))
	copyHeader(w.Header(), resp.Header)
	w.Header().Del("Content-Length")
	w.Header().Set("Content-Length", strconv.Itoa(len(page)))
	w.WriteHeader(resp.StatusCode)
	_, _ = io.WriteString(w, page)
}

// injectsInto says this answer is an HTML screen the prototype goes into.
func (m *mock) injectsInto(screen string, resp *http.Response) bool {
	if len(m.files) == 0 {
		return false
	}
	if !strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
		return false
	}
	if len(m.paths) == 0 {
		return true
	}
	for _, one := range m.paths {
		if screen == one || strings.HasPrefix(screen, one+"/") {
			return true
		}
	}
	return false
}

// inject writes the prototype's files into one page: the stylesheets in the
// head, the scripts at the end of the body, both in the order mockFiles gave.
func (m *mock) inject(page string) string {
	var head, tail strings.Builder
	for _, name := range m.files {
		href := mockPrefix + name
		switch strings.ToLower(filepath.Ext(name)) {
		case ".css":
			fmt.Fprintf(&head, "<link rel=\"stylesheet\" href=%q>\n", href)
		default:
			fmt.Fprintf(&tail, "<script src=%q defer></script>\n", href)
		}
	}
	if i := strings.Index(page, "</head>"); i >= 0 {
		page = page[:i] + head.String() + page[i:]
	}
	if i := strings.LastIndex(page, "</body>"); i >= 0 {
		page = page[:i] + tail.String() + page[i:]
	}
	return page
}

// copyHeader copies one answer's headers, but for the ones that say how the body
// is carried.
func copyHeader(to, from http.Header) {
	for name, values := range from {
		switch http.CanonicalHeaderKey(name) {
		case "Connection", "Keep-Alive", "Transfer-Encoding", "Content-Length":
			continue
		}
		for _, value := range values {
			to.Add(name, value)
		}
	}
}
