package browser

import (
	"slices"
	"testing"
)

// TestCommandPerSystem: each system gets the command it ships for opening a URL,
// with the URL as the last argument. The table names the systems instead of
// asking this one, so nothing opens while the tests run.
func TestCommandPerSystem(t *testing.T) {
	const url = "http://127.0.0.1:8787/"
	for _, tc := range []struct {
		goos string
		name string
		args []string
	}{
		{goos: "darwin", name: "open", args: []string{url}},
		{goos: "linux", name: "xdg-open", args: []string{url}},
		{goos: "windows", name: "rundll32", args: []string{"url.dll,FileProtocolHandler", url}},
		{goos: "freebsd", name: "xdg-open", args: []string{url}},
	} {
		name, args := command(tc.goos, url)
		if name != tc.name || !slices.Equal(args, tc.args) {
			t.Errorf("command(%q, url) = %q %q, want %q %q", tc.goos, name, args, tc.name, tc.args)
		}
		if last := args[len(args)-1]; last != url {
			t.Errorf("command(%q, url) hands over %q, want the URL itself", tc.goos, last)
		}
	}
}

// TestOpenWithoutACommand: a system whose opener is not there (an empty PATH
// stands in for it) comes back as an error, and nothing is started: no browser
// opens while the tests run.
func TestOpenWithoutACommand(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := Open("http://127.0.0.1:8787/"); err == nil {
		t.Error("Open with no opener on PATH reported success, want an error")
	}
}
