// Package browser opens a URL in the browser the person has chosen for their
// machine. It is a leaf: it knows the command each system hands a URL to and
// nothing else, so hergy serve can open its own address without knowing
// anything about a browser.
package browser

import (
	"os/exec"
	"runtime"
)

// Open hands url to this machine's default browser and returns once the command
// that opens it has started. The browser keeps running on its own; hergy does
// not wait for it. An error means the command could not be started at all (a
// Linux machine without xdg-open, for one). What to say about that belongs to
// the caller: hergy serve goes on either way.
func Open(url string) error {
	name, args := command(runtime.GOOS, url)
	cmd := exec.Command(name, args...) //nolint:gosec // G204: name and args are the fixed table below, and url is hergy's own listener
	if err := cmd.Start(); err != nil {
		return err
	}
	// The command is left to finish on its own: open, xdg-open and rundll32
	// hand the URL over and exit, and hergy does not wait for the browser
	// behind them.
	return cmd.Process.Release()
}

// command is the program, with its arguments, that opens url in the default
// browser on goos: the ones the systems themselves ship (d-fb89).
func command(goos, url string) (name string, args []string) {
	switch goos {
	case "darwin":
		return "open", []string{url}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		// Linux, and the other unixes hergy is built for: xdg-open is the
		// command those systems agree on.
		return "xdg-open", []string{url}
	}
}
