package app

import (
	"context"
	"html"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/web"
	"github.com/aq2bq/hergy/internal/web/profileview"
)

// TestTheProfilesScreenSavesThroughTheServer: the whole write path runs over
// HTTP, the way a person drives it. The page hands out this run's token and the
// hash of the file it read, the form posts the rows to the save, and the layer's
// own profiles.yaml holds what was typed: a profile added, one renamed, its args
// and the values of its env in, and a profile the form no longer holds deleted.
// The file the screen wrote is one hergy validate reads.
func TestTheProfilesScreenSavesThroughTheServer(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeProfiles(t, f)
	port := servePort{env: serveEnv(t)}
	screen, err := profileview.New(port)
	if err != nil {
		t.Fatalf("profileview.New: %v", err)
	}
	server, err := web.New(port, screen)
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}
	base := serveForTest(t, server)
	local := filepath.Join(f.proj, ".hergy", "profiles.yaml")

	page := getBody(t, base+"/profiles?layer=local")
	// The form the page draws carries what the save needs, and nothing else.
	form := url.Values{
		"token": {formField(t, page, "token")},
		"hash":  {formField(t, page, "hash")},
		"name":  {"impl", "fresh"},
		"kind":  {"claude", "codex"},
		"args":  {"--model\nopus", ""},
		"env":   {"OPENAI_API_KEY=sk-live\nHOME_HINT=here", ""},
	}
	answer := postForm(t, base+"/profiles/save?layer=local", form)
	if !strings.Contains(html.UnescapeString(answer), "saved") {
		t.Errorf("the answer of a save does not say it was saved:\n%s", answer)
	}
	want := `version: 1
profiles:
  fresh:
    kind: codex
  impl:
    kind: claude
    args:
      - --model
      - opus
    env:
      HOME_HINT: here
      OPENAI_API_KEY: sk-live
`
	if got := readFile(t, local); got != want {
		t.Errorf("the local profiles.yaml holds:\n%s\nwant:\n%s", got, want)
	}
	if err := validateProfiles(local); err != nil {
		t.Errorf("hergy validate refuses the file the screen wrote: %v", err)
	}
	global := filepath.Join(f.cfg, "hergy", "profiles.yaml")
	if got := readFile(t, global); !strings.Contains(got, "opencode") {
		t.Errorf("the global profiles.yaml changed to:\n%s\nwant the layer the save was not for", got)
	}

	// A form whose env line is not name=value is refused the way hergy
	// validate refuses it: the answer comes back with the reason, and the file
	// stays as it was.
	broken := url.Values{
		"token": form["token"],
		"hash":  {hashText(readFile(t, local))},
		"name":  {"impl"},
		"kind":  {"claude"},
		"args":  {""},
		"env":   {"OPENAI_API_KEY"},
	}
	answer = postForm(t, base+"/profiles/save?layer=local", broken)
	if got := readFile(t, local); got != want {
		t.Errorf("a refused save wrote:\n%s\nwant the file untouched:\n%s", got, want)
	}
	if !strings.Contains(html.UnescapeString(answer), "name=value") {
		t.Errorf("the answer of a refused save does not say why:\n%s", answer)
	}

	// The save of the global layer writes the global file: the layer travels in
	// the form, so a page of the global layer never writes the local one.
	page = getBody(t, base+"/profiles?layer=global")
	globalForm := url.Values{
		"token": {formField(t, page, "token")},
		"hash":  {formField(t, page, "hash")},
		"name":  {"impl"},
		"kind":  {"codex"},
		"args":  {""},
		"env":   {""},
	}
	postForm(t, base+"/profiles/save?layer=global", globalForm)
	if got := readFile(t, global); got != "version: 1\nprofiles:\n  impl:\n    kind: codex\n" {
		t.Errorf("the global profiles.yaml holds:\n%s\nwant only the profile the global form held", got)
	}
	if got := readFile(t, local); got != want {
		t.Errorf("the global save changed the local file to:\n%s\nwant:\n%s", got, want)
	}
}

// serveForTest serves server on a free port for the length of the test and
// answers the address it opened on.
func serveForTest(t *testing.T, server *web.Server) string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ready := make(chan string, 1)
	go func() {
		if err := server.Serve(ctx, 0, func(url string) { ready <- url }); err != nil {
			t.Errorf("serve stopped with %v, want nil", err)
		}
	}()
	select {
	case url := <-ready:
		return url
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not open its socket")
		return ""
	}
}

// getBody is one page of the test server, as its body.
func getBody(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	return bodyOf(t, resp)
}

// postForm posts one form the way the browser does, and answers the body of the
// answer. A refusal is an answer too: the screens say why with a 200, so htmx
// swaps it in (spec/10 の 4 節).
func postForm(t *testing.T, url string, form url.Values) string {
	t.Helper()
	resp, err := http.PostForm(url, form)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	return bodyOf(t, resp)
}

func bodyOf(t *testing.T, resp *http.Response) string {
	t.Helper()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s %s = %d, want 200", resp.Request.Method, resp.Request.URL, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// formField is one field of a page's form, by the name the form gives it: the
// token of this run, or the hash of the file the page was drawn from.
func formField(t *testing.T, page, name string) string {
	t.Helper()
	_, rest, ok := strings.Cut(page, `name="`+name+`" value="`)
	if !ok {
		t.Fatalf("the page carries no %s field:\n%s", name, page)
	}
	value, _, _ := strings.Cut(rest, `"`)
	return html.UnescapeString(value)
}
